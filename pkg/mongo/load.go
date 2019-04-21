// Copyright © 2019 Stephen Bunn
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
// // This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package mongo

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/scbunn/mdbload/pkg/queue"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"
)

// Prometheus metrics
var (
	operationLatency = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: "mdbload",
			Name:      "operation_latency_seconds",
			Help:      "operational latency of mdbload",
		},
		[]string{"operation"},
	)

	operationFailure = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "mdbload",
			Name:      "operation_failure_total",
			Help:      "the number of failed mdbload mongo operations",
		},
		[]string{"operation"},
	)

	// need a separate document counter because an insert operation could
	// insert more than one document
	documentCounter = prometheus.NewCounter(
		prometheus.CounterOpts{
			Namespace: "mdbload",
			Name:      "documents_total",
			Help:      "The number of documents inserted",
		},
	)
)

// MongoLoadOptions type for containing load testing options
type MongoLoadOptions struct {
	Version              string
	ConnectionString     string
	Database             string
	Collection           string
	SocketTimeout        time.Duration
	ServerConnectTimeout time.Duration
	TestDuration         time.Duration
	MaxPoolSize          uint16
	ReadPreference       string
	EnableJournal        bool
	WriteAcks            int
	Queue                *queue.Queue
	PrometheusRegistry   *prometheus.Registry
}

// MongoLoad type for managing load tests to a mongo cluster
type MongoLoad struct {
	ctx     context.Context
	db      *mongo.Database
	options *MongoLoadOptions
	queue   *queue.Queue
}

// MongoDocument is the structure we stuff in a queue to read it later
type MongoDocument struct {
	Id        string
	Hostname  string
	Timestamp int64
}

func configureOptions(opts *MongoLoadOptions) *options.ClientOptions {
	o := options.Client()
	o.SetMaxPoolSize(opts.MaxPoolSize)
	o.SetAppName("MongoLoadTest " + opts.Version)
	o.ApplyURI(opts.ConnectionString)
	o.SetConnectTimeout(opts.ServerConnectTimeout)
	o.SetConnectTimeout(1 * time.Second)
	o.SetServerSelectionTimeout(opts.ServerConnectTimeout)
	o.SetServerSelectionTimeout(1 * time.Second)
	o.SetSocketTimeout(opts.SocketTimeout)

	// Configure Read Preference
	mode, err := readpref.ModeFromString(opts.ReadPreference)
	if err != nil {
		log.WithFields(log.Fields{
			"read preference": opts.ReadPreference,
			"error":           err,
		}).Fatal("could not set read preference")
	}
	rp, err := readpref.New(mode)
	o.SetReadPreference(rp)

	// Configure write concern
	journal := writeconcern.J(opts.EnableJournal)
	writeAcks := writeconcern.W(opts.WriteAcks)
	wc := writeconcern.New(journal, writeAcks)
	o.SetWriteConcern(wc)

	log.WithFields(log.Fields{
		"AppName":                *o.AppName,
		"ConnectTimeout":         fmt.Sprintf("%s", o.ConnectTimeout),
		"Hosts":                  o.Hosts,
		"MaxPoolSize":            *o.MaxPoolSize,
		"ServerSelectionTimeout": fmt.Sprintf("%s", o.ServerSelectionTimeout),
		"SocketTimeout":          fmt.Sprintf("%s", o.SocketTimeout),
		"Database":               opts.Database,
		"Collection":             opts.Collection,
		"ReadPreference":         rp.Mode(),
		"Write Journal":          opts.EnableJournal,
		"Write Acks":             opts.WriteAcks,
	}).Info("MongoDB driver configured")
	return o
}

func (m *MongoLoad) registerPrometheusMetrics(registry *prometheus.Registry) {
	registry.MustRegister(operationLatency)
	registry.MustRegister(operationFailure)
	registry.MustRegister(documentCounter)
}

// Init Initialize a new connection to mongo and set the database
// If Init fails to initialize a database then all other mongo operations will
// fail.
func (m *MongoLoad) Init(ctx context.Context, opts *MongoLoadOptions) error {
	o := configureOptions(opts)
	m.registerPrometheusMetrics(opts.PrometheusRegistry)

	client, err := mongo.NewClient(o)
	if err != nil {
		return fmt.Errorf("Could not connect to mongo: %v", err)
	}
	if err = client.Connect(ctx); err != nil {
		return fmt.Errorf("mongo client could not connect with background context: %v", err)
	}

	m.queue = opts.Queue
	m.ctx = ctx
	db := client.Database(opts.Database)
	m.db = db
	m.options = opts
	if err = client.Ping(m.ctx, nil); err != nil {
		return err
	}
	log.Info("Connected to mongo cluster")
	return nil
}

// InsertDocuments attempts to insert a batch of documents as a single operation.
// This method uses the mongo InsertMany operation.
//
// The document argument is expected to be a slice of BSON objects.
//
// This method returns an OperationResult and slice of ObjectsIDs if the operation
// was successful.  If the operation failed the slice will be nil.
// ObjectIDs are converted to hex and represented as strings if the _id is an
// ObjectID
func (m *MongoLoad) InsertDocuments(documents []interface{}) ([]string, bool) {
	documentCounter.Add(float64(len(documents)))
	collection := m.db.Collection(m.options.Collection)

	start := time.Now()
	result, err := collection.InsertMany(m.ctx, documents)
	operationLatency.WithLabelValues("insert").Observe(time.Since(start).Seconds())

	if err != nil {
		operationFailure.WithLabelValues("insert").Add(float64(len(documents)))
		return nil, false
	}
	return m.ObjectIDsToString(result.InsertedIDs), true
}

//InsertDocument attempts to insert a single document into a mongo collection.
//
//The method returns an OperationResult and string with the object id of the
//inserted document.  If the operation was unsuccessful the string will be an
//empty string.
//
//document is expected to be a BSON object
func (m *MongoLoad) InsertDocument(document interface{}) (string, bool) {
	collection := m.db.Collection(m.options.Collection)
	documentCounter.Inc()
	start := time.Now()
	result, err := collection.InsertOne(m.ctx, document)
	operationLatency.WithLabelValues("insert").Observe(time.Since(start).Seconds())

	if err != nil {
		log.Error(err)
		operationFailure.WithLabelValues("insert").Inc()
		return "", false
	}
	return m.ObjectIDToString(result.InsertedID.(primitive.ObjectID)), true
}

// ObjectIDToString converts a mongo ObjectID to a string representation of
// the hex value.
func (m *MongoLoad) ObjectIDToString(oid primitive.ObjectID) string {
	return oid.Hex()
}

// ObjectIDsToString converts an array of ObjectID primitives to their string
// representations.
func (m *MongoLoad) ObjectIDsToString(oids []interface{}) []string {
	var results []string
	for _, oid := range oids {
		results = append(results, m.ObjectIDToString(oid.(primitive.ObjectID)))
	}
	return results
}

// ConvertJSONtoBSON converts a JSON string to a BSON object
func (m *MongoLoad) ConvertJSONtoBSON(document string) interface{} {
	/*
		var bsonDocument interface{}
		err := mgoBSON.UnmarshalJSON([]byte(document), &bsonDocument)
		if err != nil {
			log.Fatal(err)
		}
		return bsonDocument
	*/
	var bsonDocument interface{}
	err := bson.UnmarshalExtJSON([]byte(document), false, &bsonDocument)
	if err != nil {
		log.Fatal(err)
	}
	return bsonDocument
}

// InsertOneRoutine writes documents in a loop until the duration has expired
// or a request to exit.
//
// InsertOne expects a document channel
func (m *MongoLoad) InsertOneRoutine(docs chan interface{}, waitGroup *sync.WaitGroup) {
	defer waitGroup.Done()
	hostname, _ := os.Hostname()
	timeout := time.After(m.options.TestDuration)

	// block until we get a document
	// Document should be a BSON object
	document := <-docs
	q := *m.queue
	for {
		select {
		case <-timeout: // duration has elapsed so bail
			return
		case document = <-docs: // get a new document if there is one
		default: // don't block until timeout
		}

		// write a document
		id, ok := m.InsertDocument(document)
		if !ok {
			log.WithFields(log.Fields{
				"ok": ok,
				"id": id,
			}).Error("failed to insert document")
			continue // don't enqueue a failed insert
		}
		q.Enqueue(MongoDocument{
			Id:        id,
			Hostname:  hostname,
			Timestamp: time.Now().UnixNano(),
		})
	}
}

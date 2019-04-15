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
	"log"
	"sync"
	"time"

	"github.com/scbunn/mdbload/pkg/telemetry"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	mgoBSON "gopkg.in/mgo.v2/bson"
)

// OperationResult stores the telemetry made from a read or write operation
// The OperationResult is used to compute load statistics for display output and
// is not used as a part of the prometheus chain if prometheus is enabled.
type OperationResult struct {
	Operation     string
	Duration      float64
	DocumentCount int
	Success       bool
}

// MongoLoadOptions type for containing load testing options
type MongoLoadOptions struct {
	MongoClientOptions   *options.ClientOptions
	ConnectionString     string
	Database             string
	Collection           string
	OperationTimeout     time.Duration
	ServerConnectTimeout time.Duration
	TestDuration         time.Duration
}

// MongoLoad type for managing load tests to a mongo cluster
type MongoLoad struct {
	ctx     context.Context
	db      *mongo.Database
	options *MongoLoadOptions
}

// Init Initialize a new connection to mongo and set the database
// If Init fails to initialize a database then all other mongo operations will
// fail.
func (m *MongoLoad) Init(ctx context.Context, opts *MongoLoadOptions) error {
	o := options.Client()
	o.SetMaxPoolSize(100)
	client, err := mongo.NewClient(o.ApplyURI(opts.ConnectionString))
	if err != nil {
		return fmt.Errorf("Could not connect to mongo: %v", err)
	}
	if err = client.Connect(ctx); err != nil {
		return fmt.Errorf("mongo client could not connect with background context: %v", err)
	}

	fmt.Println(*o.MaxPoolSize)
	m.ctx = ctx
	db := client.Database(opts.Database)
	m.db = db
	m.options = opts
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
func (m *MongoLoad) InsertDocuments(documents []interface{}) (*OperationResult, []string) {
	opResult := OperationResult{
		Operation:     "InsertMany",
		DocumentCount: len(documents),
	}
	collection := m.db.Collection(m.options.Collection)
	start := time.Now()
	result, err := collection.InsertMany(m.ctx, documents)
	opResult.Duration = time.Since(start).Seconds()

	if err != nil {
		opResult.Success = false
		fmt.Println(err)
		return &opResult, nil
	}
	opResult.Success = true
	return &opResult, m.ObjectIDsToString(result.InsertedIDs)

}

//InsertDocument attempts to insert a single document into a mongo collection.
//
//The method returns an OperationResult and string with the object id of the
//inserted document.  If the operation was unsuccessful the string will be an
//empty string.
//
//document is expected to be a BSON object
func (m *MongoLoad) InsertDocument(document interface{}, metrics *telemetry.PrometheusMetrics) (*OperationResult, string) {
	opResult := OperationResult{
		Operation:     "InsertOne",
		DocumentCount: 1,
	}
	inserts := *metrics.Inserts
	insertLatency := *metrics.InsertLatency
	failedInserts := *metrics.FailedInserts

	inserts.Inc()
	collection := m.db.Collection(m.options.Collection)
	start := time.Now()
	result, err := collection.InsertOne(m.ctx, document)
	opResult.Duration = time.Since(start).Seconds()
	insertLatency.Observe(opResult.Duration)

	if err != nil {
		opResult.Success = false
		fmt.Println(err)
		failedInserts.Inc()
		return &opResult, ""
	}
	opResult.Success = true
	return &opResult, m.ObjectIDToString(result.InsertedID.(primitive.ObjectID))

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
	var bsonDocument interface{}
	err := mgoBSON.UnmarshalJSON([]byte(document), &bsonDocument)
	if err != nil {
		log.Fatal(err)
	}
	return bsonDocument
}

// InsertOneRoutine writes documents in a loop until the duration has expired
// or a request to exit.
//
// InsertOne expects a document channel
func (m *MongoLoad) InsertOneRoutine(docs chan interface{}, results chan *OperationResult, waitGroup *sync.WaitGroup, metrics *telemetry.PrometheusMetrics) {
	defer waitGroup.Done()
	timeout := time.After(m.options.TestDuration)

	// block until we get a document
	// Document should be a BSON object
	document := <-docs
	for {
		select {
		case <-timeout: // duration has elapsed so bail
			return
		case document = <-docs: // get a new document if there is one
		default: // don't block until timeout
		}

		// write a document
		or, _ := m.InsertDocument(document, metrics)
		if !or.Success {
			fmt.Printf("insert failed: %v\n", or)
		}

		select {
		case results <- or: // send the result to the results channel
		default:
			fmt.Println("result channel buffer full; losing a result")
		}
	}

}

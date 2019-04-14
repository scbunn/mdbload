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
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	mgoBSON "gopkg.in/mgo.v2/bson"
)

// OperationResult stores the telemetry made from a read or write operation
type OperationResult struct {
	operation     string
	duration      float64
	documentCount int
	success       bool
}

// MongoTest structure for generating r/w load on a mongodb cluster
type MongoTest struct {
	client           *mongo.Client
	options          *options.ClientOptions
	ConnectionString string
	Timeout          time.Duration
	Database         string
	Collection       string
}

// GetClient return an instance of the mongo client
func (m *MongoTest) GetClient() (*mongo.Client, error) {
	if m.client != nil {
		return m.client, nil
	}

	options := m.GetOptions()
	client, err := mongo.Connect(context.TODO(), options.ApplyURI(m.ConnectionString))
	if err != nil {
		return nil, err
	}
	m.client = client
	return m.client, nil
}

// GetOptions return an instance of a mongo options object
// with a set of defaults if not defined.
func (m *MongoTest) GetOptions() *options.ClientOptions {
	if m.options != nil {
		return m.options
	}

	options := options.Client()
	options.ConnectTimeout = &m.Timeout
	options.SocketTimeout = &m.Timeout
	options.ServerSelectionTimeout = &m.Timeout
	m.options = options
	return m.options
}

// InsertDocuments inserts a slice of BSON documents as an InsertMany
// operation.
func (m *MongoTest) InsertDocuments(documents []interface{}) OperationResult {
	var duration float64
	success := true
	client, _ := m.GetClient()
	collection := client.Database(m.Database).Collection(m.Collection)
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	start := time.Now()
	_, err := collection.InsertMany(ctx, documents)
	duration = time.Since(start).Seconds()
	if err != nil {
		log.Fatal(err)
		success = false
	}

	return OperationResult{
		operation:     "InsertMany",
		duration:      duration,
		documentCount: len(documents),
		success:       success,
	}
}

// InsertDocument inserts a BSON document as an InsertOne operation
func (m *MongoTest) InsertDocument(document interface{}) OperationResult {
	var duration float64
	success := true
	client, _ := m.GetClient()
	collection := client.Database(m.Database).Collection(m.Collection)
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)

	start := time.Now()
	_, err := collection.InsertOne(ctx, document)
	duration = time.Since(start).Seconds()
	if err != nil {
		log.Fatal(err)
		success = false
	}

	return OperationResult{
		operation:     "InsertOne",
		duration:      duration,
		documentCount: 1,
		success:       success,
	}
}

// ConvertJSONtoBSON converts a JSON string to a BSON object
func (m *MongoTest) ConvertJSONtoBSON(document string) interface{} {
	var bsonDocument interface{}
	err := mgoBSON.UnmarshalJSON([]byte(document), &bsonDocument)
	if err != nil {
		log.Fatal(err)
	}

	return bsonDocument
}

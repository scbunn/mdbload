// Copyright © 2019 Stephen Bunn
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/scbunn/mdbload/pkg/mongo"
	"github.com/scbunn/mdbload/pkg/telemetry"
	lane "gopkg.in/oleiade/lane.v1"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// prometheusOptions builds a new telemetry.PrometheusOptions object
func prometheusOptions() *telemetry.PrometheusOptions {
	options := telemetry.PrometheusOptions{
		Frequency: viper.GetDuration("telemetry.pushgateway.frequency"),
		Server:    viper.GetString("telemetry.pushgateway.server"),
	}
	return &options
}

// mongoDbOptions builds a new mongo.MongoLoadOptions object
func mongoDbOptions() *mongo.MongoLoadOptions {
	options := mongo.MongoLoadOptions{
		ConnectionString: viper.GetString("mongodb.connectionString"),
		Database:         viper.GetString("mongodb.database"),
		Collection:       viper.GetString("mongodb.collection"),
		TestDuration:     viper.GetDuration("duration"),
	}
	return &options
}

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a load test",
	Long:  `Starts a new load test against a mongodb cluter`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("start called: " + viper.GetString("mongodb.connectionString"))
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		mongoOptions := mongoDbOptions()
		promOptions := prometheusOptions()
		mdb := new(mongo.MongoLoad)
		if err := mdb.Init(ctx, mongoOptions); err != nil {
			log.Fatal(err)
		}
		var documents []string
		var bsonDocuments []interface{}
		documents = append(documents, "{\"name\": \"foobar\"}")
		documents = append(documents, "{\"name\": \"bob jones\"}")

		for _, doc := range documents {
			bson := mdb.ConvertJSONtoBSON(doc)
			bsonDocuments = append(bsonDocuments, bson)
		}

		// initial variables
		var resultsQueue *lane.Queue = lane.NewQueue()
		results := make(chan *mongo.OperationResult, 1024)
		docChannel := make(chan interface{}, 10)
		defer close(docChannel)

		// wait groups for to sync go routines
		var loadWaitGroup sync.WaitGroup
		var utilityWaitGroup sync.WaitGroup
		doneChannel := make(chan bool)
		pgExit := make(chan bool)
		defer close(doneChannel)
		defer close(pgExit)

		// start utility routines
		utilityWaitGroup.Add(3)
		metrics := telemetry.PrometheusMetrics{}
		metrics.Init()
		go updateDocument(bsonDocuments, docChannel, &utilityWaitGroup, doneChannel, &metrics)
		go getResults(results, resultsQueue, &utilityWaitGroup)
		go telemetry.PushMetrics(promOptions, &metrics, &utilityWaitGroup, pgExit)

		// Start Load Generation
		for i := 0; i < 20; i++ {
			loadWaitGroup.Add(1)
			go mdb.InsertOneRoutine(docChannel, results, &loadWaitGroup, &metrics)
		}
		loadWaitGroup.Wait()
		fmt.Println("done with load")

		// clean up utility routines
		close(results)
		doneChannel <- true
		pgExit <- true
		utilityWaitGroup.Wait()

		// get the results
		fmt.Printf("documents: %v\n", resultsQueue.Size())
		//		fmt.Printf("TPS: %v\n", telemetry.TPS(resultsQueue, mongoOptions.TestDuration))
	},
}

func getResults(results chan *mongo.OperationResult, q *lane.Queue, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case opResult, ok := <-results:
			if !ok { // channel was closed; bail
				fmt.Println("getResults is closing down shop")
				return
			}
			q.Enqueue(opResult)
		}
	}
}

func updateDocument(documents []interface{}, docs chan interface{}, wg *sync.WaitGroup, done chan bool, metrics *telemetry.PrometheusMetrics) {
	defer wg.Done()
	templatesGenerated := *metrics.TemplatesGenerated
	for {
		select {
		case <-done:
			fmt.Println("updateDocument closing down shop")
			return
		case docs <- documents[0]:
			templatesGenerated.Inc()
		default:
		}
	}
}

func init() {
	rootCmd.AddCommand(startCmd)

	// General flags
	startCmd.Flags().Duration("duration", 30*time.Second, "Duration of the load test")
	viper.BindPFlag("duration", startCmd.Flags().Lookup("duration"))

	// MongoDB settings
	startCmd.Flags().String("mongodb-connection-string", "mongodb://127.0.0.1:27017", "MongoDB Connection String")
	viper.BindPFlag("mongodb.connectionString", startCmd.Flags().Lookup("mongodb-connection-string"))
	startCmd.Flags().Duration("mongodb-server-timeout", 1*time.Second, "MongoDB server connection timeout")
	viper.BindPFlag("mongodb.serverTimeout", startCmd.Flags().Lookup("mongodb-server-timeout"))
	startCmd.Flags().String("mongodb-database", "loadtest", "Database to use for load tests")
	viper.BindPFlag("mongodb.database", startCmd.Flags().Lookup("mongodb-database"))
	startCmd.Flags().String("mongodb-collection", "samples", "Collection to use for load tests")
	viper.BindPFlag("mongodb.collection", startCmd.Flags().Lookup("mongodb-collection"))

	// Telemetry
	startCmd.Flags().Duration("pushgateway-frequency", 30*time.Second, "Frequency to push metrics to a prometheus push gateway")
	viper.BindPFlag("telemetry.pushgateway.frequency", startCmd.Flags().Lookup("pushgateway-frequency"))
	startCmd.Flags().String("pushgateway-server", "127.0.0.1:9091", "Server and port of the prometheus push gateway")
	viper.BindPFlag("telemetry.pushgateway.server", startCmd.Flags().Lookup("pushgateway-server"))
}

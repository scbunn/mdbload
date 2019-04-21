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
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/scbunn/mdbload/pkg/mongo"
	"github.com/scbunn/mdbload/pkg/queue"
	"github.com/scbunn/mdbload/pkg/telemetry"

	log "github.com/sirupsen/logrus"
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
		ReadPreference:   viper.GetString("mongodb.readPreference"),
		WriteAcks:        viper.GetInt("mongodb.writeConcern"),
		EnableJournal:    viper.GetBool("mongodb.writeJournal"),
		MaxPoolSize:      uint16(viper.GetUint("mongodb.connectionPoolSize")),
	}
	return &options
}

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a load test",
	Long:  `Starts a new load test against a mongodb cluter`,
	Run: func(cmd *cobra.Command, args []string) {
		log.WithFields(log.Fields{
			"version": VERSION,
			"build":   fmt.Sprintf("%s.%s", GITSHA, BUILDTIME),
		}).Info("Starting local test")
		ctx := context.Background()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		registry := prometheus.NewRegistry()
		// testing
		/*
			rq := queue.RedisQueue{}
			rq.Init(registry)
		*/

		// create the queue
		var q queue.Queue
		mq := queue.MemoryQueue{}
		mq.Init(registry)
		q = &mq

		mongoOptions := mongoDbOptions()
		promOptions := prometheusOptions()
		mongoOptions.Queue = &q
		mongoOptions.PrometheusRegistry = registry
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
		utilityWaitGroup.Add(1)
		metrics := telemetry.Prometheus{
			Options:  promOptions,
			Registry: registry,
		}
		go updateDocument(bsonDocuments, docChannel, &utilityWaitGroup, doneChannel)

		if viper.GetBool("telemetry.pushgateway.enable") {
			utilityWaitGroup.Add(1)
			go metrics.PushMetrics(&utilityWaitGroup, pgExit)
		}

		// Start Load Generation
		for i := 0; i < 20; i++ {
			loadWaitGroup.Add(1)
			go mdb.InsertOneRoutine(docChannel, &loadWaitGroup)
		}
		loadWaitGroup.Wait()
		log.Info("Done with load")

		//		log.Info(telemetry.DumpMetrics(&metrics))

		// queue stuff
		log.WithFields(log.Fields{
			"size": q.Size(),
		}).Info("queued up")
		log.Info(q.Head())
		item := q.Dequeue()
		switch item.(type) {
		case mongo.MongoDocument:
			log.Info(item)
		case string:
			i := mongo.MongoDocument{}
			err := json.Unmarshal([]byte(item.(string)), &i)
			if err != nil {
				log.Error(err)
			}
			log.Info(i)
		}

		for q.Head() != nil {
			item := q.Dequeue()
			log.Info(item)
		}

		// clean up utility routines
		doneChannel <- true
		if viper.GetBool("telemetry.pushgateway.enable") {
			pgExit <- true
		}

		utilityWaitGroup.Wait()

	},
}

func updateDocument(documents []interface{}, docs chan interface{}, wg *sync.WaitGroup, done chan bool) {
	defer wg.Done()
	//	templatesGenerated := *metrics.TemplatesGenerated
	for {
		select {
		case <-done:
			log.Info("updateDocument closing down shop")
			return
		case docs <- documents[0]:
			//			templatesGenerated.Inc()
		default:
		}
	}
}

func init() {
	rootCmd.AddCommand(startCmd)

	// General flags
	startCmd.Flags().Duration("duration", 30*time.Second, "Duration of the load test")
	viper.BindPFlag("duration", startCmd.Flags().Lookup("duration"))

	// Telemetry
	startCmd.Flags().Bool("pushgateway-enable", false, "Enable pushing metrics to a prometheus push gateway")
	viper.BindPFlag("telemetry.pushgateway.enable", startCmd.Flags().Lookup("pushgateway-enable"))
	startCmd.Flags().Duration("pushgateway-frequency", 30*time.Second, "Frequency to push metrics to a prometheus push gateway")
	viper.BindPFlag("telemetry.pushgateway.frequency", startCmd.Flags().Lookup("pushgateway-frequency"))
	startCmd.Flags().String("pushgateway-server", "127.0.0.1:9091", "Server and port of the prometheus push gateway")
	viper.BindPFlag("telemetry.pushgateway.server", startCmd.Flags().Lookup("pushgateway-server"))
}

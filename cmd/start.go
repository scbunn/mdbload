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
	"os"
	"sync"
	"text/template"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/scbunn/docgen"
	"github.com/scbunn/mdbload/pkg/mongo"
	"github.com/scbunn/mdbload/pkg/queue"
	"github.com/scbunn/mdbload/pkg/telemetry"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	templateDuration = prometheus.NewSummary(
		prometheus.SummaryOpts{
			Namespace: "mdbload",
			Name:      "generate_template_duration_seconds",
			Help:      "The duration to generate a template",
		},
	)
)

// prometheusOptions builds a new telemetry.PrometheusOptions object
func prometheusOptions() *telemetry.PrometheusOptions {
	options := telemetry.PrometheusOptions{
		Frequency: viper.GetDuration("telemetry.pushgateway.frequency"),
		Server:    viper.GetString("telemetry.pushgateway.server"),
	}
	return &options
}

type TelemetryData struct {
	registry               *prometheus.Registry
	pushGatewayExitChannel chan bool
	prometheusOptions      *telemetry.PrometheusOptions
}

func configureTelemetry(wg *sync.WaitGroup) (*TelemetryData, bool) {
	td := TelemetryData{
		registry:               prometheus.NewRegistry(),
		pushGatewayExitChannel: make(chan bool),
		prometheusOptions:      prometheusOptions(),
	}

	td.registry.MustRegister(templateDuration)
	metrics := telemetry.Prometheus{
		Options:  td.prometheusOptions,
		Registry: td.registry,
	}

	if viper.GetBool("telemetry.pushgateway.enable") {
		wg.Add(1)
		go metrics.PushMetrics(wg, td.pushGatewayExitChannel)
	}

	return &td, true
}

func createQueue(registry *prometheus.Registry) *queue.Queue {
	var q queue.Queue
	var queueType string
	l := log.WithFields(log.Fields{
		"type": queueType,
	})
	// TODO: wire up this boolean
	if viper.GetBool("queue.redis.enable") {
		// TODO: Redis Options
		rq := queue.RedisQueue{
			Server:   viper.GetString("queue.redis.server"),
			Registry: registry,
		}
		rq.Init()
		q = &rq
		queueType = "Redis"
		l = l.WithFields(log.Fields{
			"server": viper.GetString("queue.redis.server"),
		})
	} else {
		mq := queue.MemoryQueue{
			Registry: registry,
		}
		mq.Init()
		q = &mq
		queueType = "Memory"
	}
	l.WithFields(log.Fields{
		"type": queueType,
	}).Info("created new document queue")
	return &q
}

func createLoadTester(registry *prometheus.Registry, q *queue.Queue) (*mongo.MongoLoad, func()) {
	// Create a new context
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	options := mongo.MongoLoadOptions{
		ConnectionString:     viper.GetString("mongodb.connectionString"),
		Database:             viper.GetString("mongodb.database"),
		Collection:           viper.GetString("mongodb.collection"),
		TestDuration:         viper.GetDuration("duration"),
		SocketTimeout:        viper.GetDuration("mongodb.socketTimeout"),
		ServerConnectTimeout: viper.GetDuration("mongodb.serverConnectTimeout"),
		ConnectionTimeout:    viper.GetDuration("mongodb.connectTimeout"),
		ReadPreference:       viper.GetString("mongodb.readPreference"),
		WriteAcks:            viper.GetInt("mongodb.writeConcern"),
		EnableJournal:        viper.GetBool("mongodb.writeJournal"),
		MaxPoolSize:          uint16(viper.GetUint("mongodb.connectionPoolSize")),
		Version:              VERSION,
		Queue:                q,
		PrometheusRegistry:   registry,
	}
	mdb := new(mongo.MongoLoad)
	if err := mdb.Init(ctx, &options); err != nil {
		log.Fatal(err)
	}
	return mdb, cancel
}

func generateDocuments() chan interface{} {
	documentChannel := make(chan interface{}, 1024)
	templateDirectory := viper.GetString("templates.directory")
	templateName := viper.GetString("templates.name")
	l := log.WithFields(log.Fields{
		"directory": templateDirectory,
		"name":      templateName,
	})

	templates, err := docgen.ParseTemplates(templateDirectory)
	if err != nil {
		l.WithFields(log.Fields{
			"error": err,
		}).Fatal("Could not start document generation")
	}

	// Start template generation in a goroutine
	l.Info("Starting document generation")
	go createDocumentsFromTemplates(templates, templateName, documentChannel)
	return documentChannel
}

// create new documents from a template and pump them into the document template channel
func createDocumentsFromTemplates(templates *template.Template, name string, c chan interface{}) {
	document := renderDocument(templates, name)
	for {
		select {
		case c <- document:
			document = renderDocument(templates, name)
		}
	}
}

// start a new load test; This function blocks
func startLoadGeneration(documents chan interface{}, mdb *mongo.MongoLoad) {
	// TODO: wire these up
	wg := new(sync.WaitGroup)
	writes := viper.GetInt("goroutines.writes")
	reads := viper.GetInt("goroutines.reads")
	l := log.WithFields(log.Fields{
		"writes": writes,
		"reads":  reads,
	})

	l.Info("Creating load generation goroutines")
	for i := 0; i < writes; i++ {
		wg.Add(1)
		go mdb.InsertOneRoutine(documents, wg)
	}
	for i := 0; i < reads; i++ {
		wg.Add(1)
		go mdb.ReadOneRoutine(wg)
	}
	wg.Wait()
}

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start a load test",
	Long:  `Starts a new load test against a mongodb cluter`,
	Run: func(cmd *cobra.Command, args []string) {
		hostname, _ := os.Hostname()
		wg := new(sync.WaitGroup)
		l := log.WithFields(log.Fields{
			"instance": hostname,
		})

		l.WithFields(log.Fields{
			"version":  VERSION,
			"build":    fmt.Sprintf("%s.%s", BUILDTIME, GITSHA),
			"duration": viper.GetDuration("duration"),
		}).Info("Starting a new instance")

		// configureTelemetry
		telemetry, ok := configureTelemetry(wg)
		if !ok {
			l.Error("Telemetry failed")
		}
		defer close(telemetry.pushGatewayExitChannel)

		// Create the queue
		q := createQueue(telemetry.registry)

		// Create a new Mongo Load Tester
		mdb, cancel := createLoadTester(telemetry.registry, q)

		// Start Document Generation
		documentChannel := generateDocuments()

		// Start Load Generation
		startLoadGeneration(documentChannel, mdb)

		l.Info("load test completed")

		// clean up utility routines
		if viper.GetBool("telemetry.pushgateway.enable") {
			telemetry.pushGatewayExitChannel <- true
		}

		wg.Wait()
		cancel()
	},
}

func renderDocument(templates *template.Template, name string) interface{} {
	var template string
	var err error
	l := log.WithFields(log.Fields{
		"template": name,
		"rendered": template,
	})
	start := time.Now()
	//TODO: update docgen to support all file extensions
	template, err = docgen.RenderTemplate(name, templates)
	if err != nil {
		l.WithFields(log.Fields{
			"error": err,
		}).Fatal("could not render the template")
	}
	l.Debug("new template rendered")
	templateDuration.Observe(time.Since(start).Seconds())
	return mongo.ConvertJSONtoBSON(template)
}

func init() {
	rootCmd.AddCommand(startCmd)

	// General flags
	startCmd.Flags().Duration("duration", 30*time.Second, "Duration of the load test")
	startCmd.Flags().Int("write-routines", 1, "number of writing goroutines")
	startCmd.Flags().Int("read-routines", 1, "number of reading goroutines")
	viper.BindPFlag("duration", startCmd.Flags().Lookup("duration"))
	viper.BindPFlag("goroutines.writes", startCmd.Flags().Lookup("write-routines"))
	viper.BindPFlag("goroutines.reads", startCmd.Flags().Lookup("read-routines"))

	// Telemetry
	startCmd.Flags().Bool("enable-pushgateway", false, "Enable pushing metrics to a prometheus push gateway")
	viper.BindPFlag("telemetry.pushgateway.enable", startCmd.Flags().Lookup("enable-pushgateway"))
	startCmd.Flags().Duration("pushgateway-frequency", 30*time.Second, "Frequency to push metrics to a prometheus push gateway")
	viper.BindPFlag("telemetry.pushgateway.frequency", startCmd.Flags().Lookup("pushgateway-frequency"))
	startCmd.Flags().String("pushgateway-server", "127.0.0.1:9091", "Server and port of the prometheus push gateway")
	viper.BindPFlag("telemetry.pushgateway.server", startCmd.Flags().Lookup("pushgateway-server"))

	// Templates
	startCmd.Flags().String("template-dir", ".", "Directory where document templates are located")
	startCmd.Flags().String("template-name", "example.template", "Name of the template to use for generation")
	viper.BindPFlag("templates.directory", startCmd.Flags().Lookup("template-dir"))
	viper.BindPFlag("templates.name", startCmd.Flags().Lookup("template-name"))

	// Queue
	startCmd.Flags().Bool("enable-redis", false, "Enable redis document queue")
	startCmd.Flags().String("redis-server", "127.0.0.1:6379", "Redis server and port")
	viper.BindPFlag("queue.redis.enable", startCmd.Flags().Lookup("enable-redis"))
	viper.BindPFlag("queue.redis.server", startCmd.Flags().Lookup("redis-server"))
}

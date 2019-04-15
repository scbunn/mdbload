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

package telemetry

import (
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

// PrometheusOptions holds the options for performing prometheus operations
type PrometheusOptions struct {
	Frequency time.Duration
	Server    string
}

// PrometheusMetrics exported prometheus metrics used in mdbload
type PrometheusMetrics struct {
	InsertLatency              *prometheus.Histogram
	ReadLatency                *prometheus.Histogram
	Inserts                    *prometheus.Counter
	FailedInserts              *prometheus.Counter
	TemplatesGenerated         *prometheus.Counter
	TemplateGenerationDuration *prometheus.Histogram
}

// Init initialize all the prometheus metrics
func (p *PrometheusMetrics) Init() {
	insertLatency := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "mdbload",
		Name:      "mongodb_insert_latency_seconds",
		Help:      "insert latency of mongodb load test inserts",
		Buckets:   prometheus.LinearBuckets(0.01, 0.10, 10),
	})
	p.InsertLatency = &insertLatency

	readLatency := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "mdbload",
		Name:      "mongodb_read_latency_seconds",
		Help:      "read latency of mongodb load test reads",
		Buckets:   prometheus.LinearBuckets(0.01, 0.10, 10),
	})
	p.ReadLatency = &readLatency

	inserts := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "mdbload",
		Name:      "mongodb_inserts_requested_total",
		Help:      "number of document inserts requested",
	})
	p.Inserts = &inserts

	failedInserts := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "mdbload",
		Name:      "mongodb_inserts_failed_total",
		Help:      "number of documents inserts that failed",
	})
	p.FailedInserts = &failedInserts

	templatesGenerated := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "mdbload",
		Name:      "templates_generated_total",
		Help:      "number of documents generated from a template",
	})
	p.TemplatesGenerated = &templatesGenerated

	templateGenDuration := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "mdbload",
		Name:      "template_generation_duration_seconds",
		Help:      "the duration to generate a document from a template",
		Buckets:   prometheus.LinearBuckets(0.01, 0.10, 10),
	})
	p.TemplateGenerationDuration = &templateGenDuration
}

// PushMetrics pushes prometheus metrics to a push gateway every n seconds
func PushMetrics(o *PrometheusOptions, metrics *PrometheusMetrics, wg *sync.WaitGroup, exit chan bool) {
	defer wg.Done()

	p := push.New(o.Server, "mongodb_load_gen").
		Collector(*metrics.InsertLatency).
		Collector(*metrics.Inserts).
		Collector(*metrics.FailedInserts).
		Collector(*metrics.ReadLatency).
		Collector(*metrics.TemplatesGenerated).
		Collector(*metrics.TemplateGenerationDuration).
		Collector(prometheus.NewGoCollector()).
		Grouping("instance", "foobar")
	for {
		select {
		case <-time.After(o.Frequency):
			fmt.Println("pushing metrics")
			if err := p.Add(); err != nil {
				fmt.Println(err)
			}
		case <-exit:
			fmt.Println("metrics shutting down")
			p.Add()
			return
		}
	}
}

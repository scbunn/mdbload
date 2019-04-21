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
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	log "github.com/sirupsen/logrus"
)

// PrometheusOptions holds the options for performing prometheus operations
type PrometheusOptions struct {
	Frequency time.Duration
	Server    string
}

type Prometheus struct {
	Registry *prometheus.Registry
	Options  *PrometheusOptions
}

// PushMetrics will push metrics from the registry at Frequency
func (p *Prometheus) PushMetrics(waitGroup *sync.WaitGroup, exit chan bool) {
	defer waitGroup.Done()
	p.Registry.MustRegister(prometheus.NewGoCollector())
	hostname, _ := os.Hostname()
	l := log.WithFields(log.Fields{
		"server": p.Options.Server,
	})

	pusher := push.New(p.Options.Server, "mdbload").Gatherer(p.Registry)
	pusher.Grouping("instance", hostname)
	for {
		select {
		case <-time.After(p.Options.Frequency):
			p.push(pusher)
		case <-exit:
			l.Debug("Prometheus Push Metrics shutdown signal received")
			p.push(pusher)
			return
		}
	}
}

func (p *Prometheus) push(pusher *push.Pusher) {
	l := log.WithFields(log.Fields{
		"server":    p.Options.Server,
		"frequency": p.Options.Frequency,
	})
	l.Info("pushing metrics")
	if err := pusher.Add(); err != nil {
		l.WithField("error", err).Error("could not push metrics.")
	}
}

/*
func DumpMetricsics *PrometheusMetrics) string {
	metric := &dto.Metric{}
	m := *metrics.InsertLatency
	m.Write(metric)
	marshaler := jsonpb.Marshaler{}
	marshaler.Marshal(os.Stdout, metric)
	return proto.MarshalTextString(metric)
}


// Init initialize all the prometheus metrics
func (p *PrometheusMetrics) Init() {

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
*/

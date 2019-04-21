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

package queue

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Queue generic queue interface
type Queue interface {
	Enqueue(interface{})
	Dequeue() interface{}
	Size() int
	Empty() bool
	Head() interface{}
	Init(registry *prometheus.Registry)
}

var (
	queueLatency = prometheus.NewSummaryVec(
		prometheus.SummaryOpts{
			Namespace: "mdbload",
			Name:      "queue_latency_seconds",
			Help:      "Latency of queue operations",
		},
		[]string{"operation"},
	)

	queueSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "mdbload",
			Name:      "items_queued",
			Help:      "the approximate number of items in the queue",
		},
	)

	queueError = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "mdbload",
			Name:      "queue_errors_total",
			Help:      "Number of errored queue operations",
		},
		[]string{"operation"},
	)
)

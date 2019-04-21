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
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/oleiade/lane.v1"
)

// MemoryQueue is an in-memory FIFO queue implementing the Queue interface
type MemoryQueue struct {
	queue *lane.Queue
}

// Init initializes a new in memory queue
func (q *MemoryQueue) Init(registry *prometheus.Registry) {
	if q.queue == nil {
		q.queue = lane.NewQueue()
	}
	registry.MustRegister(queueLatency)
	registry.MustRegister(queueSize)
	registry.MustRegister(queueError)
}

// Enqueue adds a new item to the queue
func (q *MemoryQueue) Enqueue(item interface{}) {
	start := time.Now()
	q.queue.Enqueue(item)
	queueLatency.WithLabelValues("enqueue").Observe(time.Since(start).Seconds())
	queueSize.Inc()
}

// Dequeue removes and returns the left most item in the queue
func (q *MemoryQueue) Dequeue() interface{} {
	start := time.Now()
	i := q.queue.Dequeue()
	queueLatency.WithLabelValues("dequeue").Observe(time.Since(start).Seconds())
	queueSize.Dec()
	return i
}

// Head returns the left most item in the queue but does not change the queue
func (q *MemoryQueue) Head() interface{} {
	return q.queue.Head()
}

// Size returns the approximate number of elements in the queue
func (q *MemoryQueue) Size() int {
	return q.queue.Size()
}

// Empty returns true if the queue is empty; false otherwise
func (q *MemoryQueue) Empty() bool {
	return q.Size() == 0
}

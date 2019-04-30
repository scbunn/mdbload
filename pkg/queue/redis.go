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
	"encoding/json"
	"time"

	"github.com/go-redis/redis"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

// RedisQueue is a distributed FIFO queue using Redis
type RedisQueue struct {
	client   *redis.Client
	key      string
	Registry *prometheus.Registry
	Server   string
}

// Init initializes a new RedisQueue
func (q *RedisQueue) Init() bool {
	if q.client == nil {
		q.client = redis.NewClient(&redis.Options{
			Addr: q.Server,
		})
	}
	q.key = "mdbload:queue"
	q.Registry.MustRegister(queueLatency)
	q.Registry.MustRegister(queueSize)
	q.Registry.MustRegister(queueError)

	return true
}

// Enqueue adds a new item to the queue
func (q *RedisQueue) Enqueue(item interface{}) {
	start := time.Now()
	i, err := json.Marshal(item)
	if err != nil {
		queueError.WithLabelValues("enqueue").Inc()
		log.Error(err)
		return
	}
	if err = q.client.RPush(q.key, string(i)).Err(); err != nil {
		log.Error(err)
		queueError.WithLabelValues("enqueue").Inc()
		return
	}
	queueSize.Inc()
	queueLatency.WithLabelValues("enqueue").Observe(time.Since(start).Seconds())
}

// Dequeue pop the tail off the queue and returns it
func (q *RedisQueue) Dequeue() interface{} {
	start := time.Now()
	item, err := q.client.BLPop(1*time.Second, q.key).Result()
	if err != nil {
		log.WithFields(log.Fields{
			"error": err,
			"key":   q.key,
			"item":  item,
		}).Error("error getting an item from the queue.")
		queueError.WithLabelValues("dequeue").Inc()
		return nil
	}
	queueLatency.WithLabelValues("dequeue").Observe(time.Since(start).Seconds())
	queueSize.Dec()
	return item[1]
}

// Size returns the approximate number of elements in the queue
func (q *RedisQueue) Size() int {
	count, err := q.client.LLen(q.key).Result()
	if err != nil {
		log.Error(err)
		return -1
	}
	return int(count)
}

// Empty returns true if the queue is empty
func (q *RedisQueue) Empty() bool {
	return q.Size() == 0
}

// Head returns the left most item in the queue without modifying the queue
func (q *RedisQueue) Head() interface{} {
	item, err := q.client.LRange(q.key, 0, 0).Result()
	if err != nil {
		log.Error(err)
		return nil
	}
	if len(item) > 0 {
		return item[0]
	}
	return nil
}

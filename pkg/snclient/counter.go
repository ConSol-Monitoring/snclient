package snclient

import (
	"container/list"
	"time"
)

type Counter struct {
	noCopy noCopy

	retentionTime float64
	data          *list.List
}

type CounterValue struct {
	timestamp time.Time
	value     float64
}

// NewCounter creates a new Counter with given retention time
func NewCounter(retentionTime float64) *Counter {
	c := &Counter{
		data:          list.New(),
		retentionTime: retentionTime,
	}

	return c
}

// Set adds a new value with current timestamp
func (c *Counter) Set(val float64) {
	c.data.PushBack(&CounterValue{
		timestamp: time.Now(),
		value:     val,
	})
	c.Trim()
}

// Trim removes all entries older than now-duration
func (c *Counter) Trim() {
	trimAfter := time.Now().Add(-1 * time.Duration(c.retentionTime) * time.Second)

	cur := c.data.Front()
	for {
		if cur == nil {
			break
		}
		if val, ok := cur.Value.(*CounterValue); ok {
			if val.timestamp.Before(trimAfter) {
				c.data.Remove(cur)
			} else {
				return
			}
		}
		cur = cur.Next()
	}
}

// AvgForDuration returns avg value for given duration
func (c *Counter) AvgForDuration(duration float64) float64 {
	useAfter := time.Now().Add(-1 * time.Duration(duration) * time.Second)

	sum := float64(0)
	count := float64(0)

	cur := c.data.Back()
	for {
		if cur == nil {
			break
		}
		if val, ok := cur.Value.(*CounterValue); ok {
			if val.timestamp.After(useAfter) {
				sum += val.value
				count++
			} else {
				break
			}
		}
		cur = cur.Prev()
	}

	if count == 0 {
		return 0
	}

	return sum / count
}

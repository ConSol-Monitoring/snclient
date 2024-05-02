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
	unixMilli int64 // timestamp in unix milliseconds
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
		unixMilli: time.Now().UTC().UnixMilli(),
		value:     val,
	})
	c.Trim()
}

// Trim removes all entries older than now-duration
func (c *Counter) Trim() {
	trimBefore := time.Now().UTC().Add(-1 * time.Duration(c.retentionTime) * time.Second).UnixMilli()
	cur := c.data.Front()
	for {
		if cur == nil {
			break
		}
		if val, ok := cur.Value.(*CounterValue); ok {
			if val.unixMilli < trimBefore {
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
	useAfter := time.Now().UTC().Add(-1 * time.Duration(duration) * time.Second).UnixMilli()

	sum := float64(0)
	count := float64(0)

	cur := c.data.Back()
	for {
		if cur == nil {
			break
		}
		if val, ok := cur.Value.(*CounterValue); ok {
			if val.unixMilli > useAfter {
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

// GetLast returns last (latest) value
func (c *Counter) GetLast() *CounterValue {
	cur := c.data.Back()
	if val, ok := cur.Value.(*CounterValue); ok {
		return val
	}

	return nil
}

// GetAt returns first value closest to given date
func (c *Counter) GetAt(useAfter time.Time) *CounterValue {
	useAfterUnix := useAfter.UTC().UnixMilli()
	cur := c.data.Back()
	var last *CounterValue
	for {
		if val, ok := cur.Value.(*CounterValue); ok {
			if val.unixMilli < useAfterUnix {
				return last
			}
			last = val
		}
		prev := cur.Prev()
		if prev == nil {
			break
		}
		cur = prev
	}

	return last
}

type CounterAny struct {
	noCopy noCopy

	retentionTime float64
	data          *list.List
}

type CounterValueAny struct {
	unixMilli int64 // timestamp in unix milliseconds
	value     interface{}
}

// NewCounterAny creates a new CounterAny with given retention time
func NewCounterAny(retentionTime float64) *CounterAny {
	c := &CounterAny{
		data:          list.New(),
		retentionTime: retentionTime,
	}

	return c
}

// Set adds a new value with current timestamp
func (c *CounterAny) Set(val interface{}) {
	c.data.PushBack(&CounterValueAny{
		unixMilli: time.Now().UTC().UnixMilli(),
		value:     val,
	})
	c.Trim()
}

// Trim removes all entries older than now-duration
func (c *CounterAny) Trim() {
	trimBefore := time.Now().UTC().Add(-1 * time.Duration(c.retentionTime) * time.Second).UnixMilli()

	cur := c.data.Front()
	for {
		if cur == nil {
			break
		}
		if val, ok := cur.Value.(*CounterValueAny); ok {
			if val.unixMilli < trimBefore {
				c.data.Remove(cur)
			} else {
				return
			}
		}
		cur = cur.Next()
	}
}

// GetLast returns last (latest) value
func (c *CounterAny) GetLast() *CounterValueAny {
	cur := c.data.Back()
	if val, ok := cur.Value.(*CounterValueAny); ok {
		return val
	}

	return nil
}

// GetAt returns first value closest to given date
func (c *CounterAny) GetAt(useAfter time.Time) *CounterValueAny {
	useAfterUnix := useAfter.UTC().UnixMilli()
	cur := c.data.Back()
	var last *CounterValueAny
	for {
		if val, ok := cur.Value.(*CounterValueAny); ok {
			if val.unixMilli < useAfterUnix {
				return last
			}
			last = val
		}
		prev := cur.Prev()
		if prev == nil {
			break
		}
		cur = prev
	}

	return last
}

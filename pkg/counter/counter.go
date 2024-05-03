package counter

import (
	"math"
	"sync"
	"time"
)

// Counter is the container for a single timeseries of performance values
// it used a fixed size storage backend
type Counter struct {
	lock    sync.RWMutex // lock for concurrent access
	data    []Value      // array of values
	current int64        // position of last inserted value
	size    int64        // number of values for this series
}

// Value is a single entry of a Counter
type Value struct {
	UnixMilli int64 // timestamp in unix milliseconds
	Value     interface{}
}

// NewCounter creates a new Counter with given retention time and interval
func NewCounter(retentionTime, interval time.Duration) *Counter {
	// round retention and interval to milliseconds
	retentionMilli := retentionTime.Milliseconds()
	intervalMilli := interval.Milliseconds()

	// round retention time to a multiple of interval
	retention := int64(math.Ceil(float64(retentionMilli)/float64(intervalMilli))) * intervalMilli
	size := retention / intervalMilli

	return &Counter{
		lock:    sync.RWMutex{},
		data:    make([]Value, size),
		size:    size,
		current: -1,
	}
}

// Set adds a new value with current timestamp
func (c *Counter) Set(val interface{}) {
	c.lock.Lock()
	c.current++
	if c.current == c.size {
		c.current = 0
	}
	c.data[c.current].UnixMilli = time.Now().UTC().UnixMilli()
	c.data[c.current].Value = val
	c.lock.Unlock()
}

// AvgForDuration returns avg value for given duration
// only works if values are stored as float64
func (c *Counter) AvgForDuration(duration time.Duration) float64 {
	useAfter := time.Now().UTC().Add(-duration).UnixMilli()

	c.lock.RLock()
	defer c.lock.RUnlock()

	sum := float64(0)
	count := float64(0)

	idx := c.current
	if idx == -1 {
		return 0
	}
	for seen := int64(0); seen <= c.size; seen++ {
		if c.data[idx].UnixMilli > useAfter {
			if val, ok := c.data[idx].Value.(float64); ok {
				sum += val
				count++
			}
		} else {
			break
		}

		idx--
		if idx < 0 {
			idx = c.size - 1
		}
	}

	if count == 0 {
		return 0
	}

	return sum / count
}

// GetRate calculates rate for given lookback timerange
// only works if values are stored as float64
func (c *Counter) GetRate(lookback time.Duration) (res float64, ok bool) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if lookback < 0 {
		lookback *= -1
	}

	count1 := c.getLast()
	count2 := c.getAt(time.Now().Add(-lookback))
	if count1 == nil || count2 == nil {
		return res, false
	}

	if count1.UnixMilli < count2.UnixMilli {
		return res, false
	}

	durationMillis := float64(count1.UnixMilli - count2.UnixMilli)
	if durationMillis <= 0 {
		return res, false
	}

	val1, ok := count1.Value.(float64)
	if !ok {
		return res, false
	}

	val2, ok := count2.Value.(float64)
	if !ok {
		return res, false
	}

	res = ((val1 - val2) / durationMillis) * 1000

	return res, true
}

// GetLast returns last (latest) value
func (c *Counter) GetLast() *Value {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.getLast()
}

func (c *Counter) getLast() *Value {
	if c.current == -1 {
		return nil
	}

	return &c.data[c.current]
}

// GetAt returns first value closest to given date
func (c *Counter) GetAt(useAfter time.Time) *Value {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.getAt(useAfter)
}

func (c *Counter) getAt(useAfter time.Time) *Value {
	useAfterUnix := useAfter.UTC().UnixMilli()
	idx := c.current
	if idx == -1 {
		return nil
	}

	var last *Value
	for seen := int64(0); seen <= c.size; seen++ {
		val := &c.data[idx]
		if val.UnixMilli < useAfterUnix {
			return last
		}
		last = val
		idx--
		if idx < 0 {
			idx = c.size - 1
		}
	}

	return last
}

// Float64 returns value as float64
func (cv *Value) Float64() float64 {
	if val, ok := cv.Value.(float64); ok {
		return val
	}

	return 0
}

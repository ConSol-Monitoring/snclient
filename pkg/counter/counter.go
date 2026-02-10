package counter

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// Counter is the container for a single timeseries of performance values
// it used a fixed size storage backend
type Counter struct {
	lock      sync.RWMutex  // lock for concurrent access
	data      []Value       // array of values. size determined by the retention and interval
	current   int64         // position of last inserted value
	oldest    int64         // position of the earliest inserted value
	size      int64         // number of values for this series
	timesSet  int64         // number of times a value was set in this counter
	retention time.Duration // the time span this counter can hold, interval * size
	interval  time.Duration // the interval time that new values are designed to be added
}

// Value is a single entry of a Counter
type Value struct {
	UnixMilli int64 // timestamp in unix milliseconds
	Value     any
}

// NewCounter creates a new Counter with given retention time and interval
func NewCounter(retentionTime, interval time.Duration) *Counter {
	// round retention and interval to milliseconds
	retentionMili := retentionTime.Milliseconds()
	intervalMili := interval.Milliseconds()

	// round retentionMili to a multiple of interval
	retentionMiliRounded := int64(math.Ceil(float64(retentionMili)/float64(intervalMili))) * intervalMili
	size := retentionMiliRounded / intervalMili

	return &Counter{
		lock:      sync.RWMutex{},
		data:      make([]Value, size),
		size:      size,
		current:   -1,
		oldest:    -1,
		retention: time.Duration(retentionMiliRounded) * time.Millisecond,
		interval:  interval,
		timesSet:  0,
	}
}

// Set adds a new value with current timestamp
func (c *Counter) Set(val any) {
	c.lock.Lock()
	// setting a value for the first time
	if c.oldest == -1 {
		c.oldest = 0
	}
	c.current++
	if c.current == c.size {
		c.current = 0
	}
	c.data[c.current].UnixMilli = time.Now().UTC().UnixMilli()
	c.data[c.current].Value = val
	c.timesSet++
	// if we already filled the array, and started overwriting, the oldest index just got overwritten
	if c.timesSet > c.size {
		c.oldest = (c.current + 1) % c.size
	}
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

	for range c.size {
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

// GetRate calculates rate for given lookback timerange .
// only works if values are stored as float64 .
// returned result is in type <change>/s .
// It uses the unixMili timestamp that is set alongisde Value on every save.
func (c *Counter) GetRate(lookback time.Duration) (res float64, err error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if lookback < 0 {
		lookback *= -1
	}

	count1 := c.getLast()
	count2 := c.getAt(time.Now().Add(-lookback))
	if count1 == nil {
		return res, fmt.Errorf("last counter entry was nil")
	}

	if count2 == nil {
		return res, fmt.Errorf("counter entry searched at T-%d was nil", lookback)
	}

	if count1.UnixMilli < count2.UnixMilli {
		return res, fmt.Errorf("last counter entry has a lower timestamp than entry searched at T-%d", lookback)
	}

	durationMillis := float64(count1.UnixMilli - count2.UnixMilli)
	if durationMillis < 0 {
		return res, fmt.Errorf("the duration difference between the counter entries is negative: %f", durationMillis)
	}
	if durationMillis == 0 {
		return res, fmt.Errorf("the duration difference between the counter entries is 0")
	}

	val1, ok := count1.Value.(float64)
	if !ok {
		return res, fmt.Errorf("last counter entry has a non float64 value")
	}

	val2, ok := count2.Value.(float64)
	if !ok {
		return res, fmt.Errorf("counter entry searched at T-%f has a non float64 value", durationMillis)
	}

	res = ((val1 - val2) / durationMillis) * 1000

	return res, nil
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

// GetFirst returns first (earliest) value
func (c *Counter) GetFirst() *Value {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.getFirst()
}

func (c *Counter) getFirst() *Value {
	// the latest added item had index c.current
	if c.oldest == -1 {
		return nil
	}

	return &c.data[c.oldest]
}

// GetAt returns first value with >= timestamp than lowerBound
func (c *Counter) GetAt(lowerBound time.Time) *Value {
	c.lock.RLock()
	defer c.lock.RUnlock()

	return c.getAt(lowerBound)
}

// Gets the first counter that has a >= timestamp than lowerBound
func (c *Counter) getAt(lowerBound time.Time) *Value {
	useAfterUnix := lowerBound.UTC().UnixMilli()

	// the counter is not yet populated
	idx := c.current
	if idx == -1 {
		return nil
	}

	var previouslyComparedValue *Value
	for range c.size {
		currentValue := &c.data[idx]
		if currentValue.UnixMilli < useAfterUnix {
			return previouslyComparedValue
		}

		previouslyComparedValue = currentValue
		idx--
		if idx < 0 {
			idx = c.size - 1
		}
	}

	return previouslyComparedValue
}

// checks if the counter can fit the targetRetention.
// intervalExtensionCount gives the seconds to optionally extend the interval before checking.
// if intervalExtensionCount is not 0, a different error message may be returned
func (c *Counter) CheckRetention(targetRetention time.Duration, intervalExtensionCount int64) error {
	extendedRetentionRange := c.retention + time.Duration(intervalExtensionCount)*c.interval

	if extendedRetentionRange < targetRetention {
		if intervalExtensionCount == 0 {
			return fmt.Errorf("counter retention range is %f seconds, less than the target retention range of %f seconds",
				extendedRetentionRange.Seconds(), targetRetention.Seconds())
		}

		return fmt.Errorf("counter retention range is %f seconds, even when extended by %d intervals to be %f seconds, it is less than target retention range of %f seconds",
			c.interval.Seconds(), intervalExtensionCount, extendedRetentionRange.Seconds(), targetRetention.Seconds())
	}

	return nil
}

// Float64 returns value as float64
func (cv *Value) Float64() float64 {
	if val, ok := cv.Value.(float64); ok {
		return val
	}

	return 0
}

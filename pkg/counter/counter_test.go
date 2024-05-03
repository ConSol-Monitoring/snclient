package counter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCounter(t *testing.T) {
	set := NewCounterSet()
	retention := time.Millisecond * 100
	interval := time.Millisecond * 9
	set.Create("test", "key", retention, interval)

	// tests on the empty counter
	counter := set.Get("test", "key")
	val := counter.GetLast()
	assert.Nil(t, val)

	avg := counter.AvgForDuration(time.Second)
	assert.InDelta(t, 0.0, avg, 0.00001)

	rate, ok := counter.GetRate(time.Second)
	assert.False(t, ok)
	assert.InDelta(t, 0.0, rate, 0.00001)

	// insert test data via set
	for i := range 5 {
		set.Set("test", "key", float64(i))
		time.Sleep(interval)
	}

	// insert more test data directly
	for i := 5; i < 10; i++ {
		counter.Set(float64(i))
		time.Sleep(interval)
	}

	// tests filled counter
	assert.Lenf(t, counter.data, 12, "counter size")
	assert.Equal(t, []string{"key"}, set.Keys("test"))

	val = counter.GetLast()
	assert.InDelta(t, 9.0, val.Float64(), 0.00001)

	avg = counter.AvgForDuration(time.Second)
	assert.InDelta(t, 4.5, avg, 0.00001)

	rate, ok = counter.GetRate(time.Second)
	assert.True(t, ok)
	assert.GreaterOrEqualf(t, rate, 50.0, "rate should be more than 50")
	assert.LessOrEqualf(t, rate, 120.0, "rate should be less than 120")

	set.Delete("test", "key")
	assert.Emptyf(t, set.counter, "set is empty now")
}

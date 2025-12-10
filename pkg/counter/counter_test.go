package counter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.GreaterOrEqualf(t, rate, 40.0, "rate should be more than 40")
	assert.LessOrEqualf(t, rate, 120.0, "rate should be less than 120")

	set.Delete("test", "key")
	assert.Emptyf(t, set.counter, "set is empty now")
}

func TestCounter2(t *testing.T) {
	set := NewCounterSet()

	retention := time.Millisecond * 4500
	interval := time.Second
	set.Create("test", "key", retention, interval)

	// empty counter
	counter := set.Get("test", "key")
	latest := counter.getLast()
	oldest := counter.getFirst()
	assert.Nil(t, latest, "calling latest on empty counter should return nil")
	assert.Nil(t, oldest, "calling oldest on empty counter should return nil")

	// check the retention for 4 seconds
	retentionCheck1 := counter.CheckRetention(time.Second*4, 0)
	require.NoError(t, retentionCheck1, "the counter should be able to hold 4 seconds")

	// check the retention for 5 seconds
	retentionCheck2 := counter.CheckRetention(time.Second*5, 0)
	require.NoError(t, retentionCheck2, "the counter should be able to hold 5 seconds")

	// check the retention for 6 seconds
	retentionCheck3 := counter.CheckRetention(time.Second*6, 0)
	require.Error(t, retentionCheck3, "the counter should not be able to hold 6 seconds")

	// check the retention for 10 seconds
	retentionCheck4 := counter.CheckRetention(time.Second*10, 0)
	require.Error(t, retentionCheck4, "the counter should not be able to hold 10 seconds")

	// check the retention for 1 minute with 10 extensions
	retentionCheck5 := counter.CheckRetention(time.Minute, 10)
	require.Error(t, retentionCheck5, "the counter should not be able to hold 1 minute with 10 interval extensions")

	// check the retention for 1 minute with 100 extensions
	retentionCheck6 := counter.CheckRetention(time.Minute, 100)
	require.NoError(t, retentionCheck6, "the counter should be able to hold 1 minute with 10 interval extensions")

	// 1 _ _ _ _
	counter.Set(float64(1))
	latest = counter.getLast()
	oldest = counter.getFirst()
	assert.InEpsilon(t, float64(1), latest.Value, 0.001, "latest element should be 1")
	assert.InEpsilon(t, float64(1), oldest.Value, 0.001, "oldest element should be 1")

	// 1 2 _ _ _
	counter.Set(float64(2))
	latest = counter.getLast()
	oldest = counter.getFirst()
	assert.InEpsilon(t, float64(2), latest.Value, 0.001, "latest element should be 2")
	assert.InEpsilon(t, float64(1), oldest.Value, 0.001, "oldest element should be 1")

	// 1 2 3 _ _
	counter.Set(float64(3))
	latest = counter.getLast()
	oldest = counter.getFirst()
	assert.InEpsilon(t, float64(3), latest.Value, 0.001, "latest element should be 3")
	assert.InEpsilon(t, float64(1), oldest.Value, 0.001, "oldest element should be 1")

	// 1 2 3 4 _
	counter.Set(float64(4))
	latest = counter.getLast()
	oldest = counter.getFirst()
	assert.InEpsilon(t, float64(4), latest.Value, 0.001, "latest element should be 4")
	assert.InEpsilon(t, float64(1), oldest.Value, 0.001, "oldest element should be 1")

	// 1 2 3 4 5
	counter.Set(float64(5))
	latest = counter.getLast()
	oldest = counter.getFirst()
	assert.InEpsilon(t, float64(5), latest.Value, 0.001, "latest element should be 5")
	assert.InEpsilon(t, float64(1), oldest.Value, 0.001, "oldest element should be 1")

	// check the average now
	avg := counter.AvgForDuration(time.Minute)
	assert.InEpsilon(t, 3, avg, 0.001, "average of 1,2,3,4,5 is 3")

	// started overwriting from the first index, the c.oldest should update
	// 6 2 3 4 5
	counter.Set(float64(6))
	latest = counter.getLast()
	oldest = counter.getFirst()
	assert.InEpsilon(t, float64(6), latest.Value, 0.001, "latest element should be 6")
	assert.InEpsilon(t, float64(2), oldest.Value, 0.001, "oldest element should be 2")

	// 6 7 3 4 5
	counter.Set(float64(7))
	latest = counter.getLast()
	oldest = counter.getFirst()
	assert.InEpsilon(t, float64(7), latest.Value, 0.001, "latest element should be 7")
	assert.InEpsilon(t, float64(3), oldest.Value, 0.001, "oldest element should be 3")
}

package counter

import (
	"fmt"
	"time"
)

type Set struct {
	counter map[string]map[string]*Counter
}

// NewCounterSet creates a new empty Set
func NewCounterSet() *Set {
	cs := &Set{
		counter: make(map[string]map[string]*Counter),
	}

	return cs
}

// Set is a map of counters organized by category and name
func (cs *Set) Create(category, key string, duration, interval time.Duration) {
	counter := NewCounter(duration, interval)

	cat, ok := cs.counter[category]
	if !ok {
		cat = make(map[string]*Counter)
		cs.counter[category] = cat
	}
	cat[key] = counter
}

// Delete removes counter by name
func (cs *Set) Delete(category, key string) {
	cat, ok := cs.counter[category]
	if ok {
		delete(cat, key)
		// remove category if it is empty now
		if len(cs.counter[category]) == 0 {
			delete(cs.counter, category)
		}
	}
}

// Keys returns all keys for category
func (cs *Set) Keys(category string) (keys []string) {
	if cat, ok := cs.counter[category]; ok {
		for key := range cat {
			keys = append(keys, key)
		}
	}

	return keys
}

// Get returns counter by category and name
func (cs *Set) Get(category, key string) *Counter {
	if cat, ok := cs.counter[category]; ok {
		if counter, ok := cat[key]; ok {
			return counter
		}
	}

	return nil
}

// calculate rate for given lookback timerange
func (cs *Set) GetRate(category, key string, lookback time.Duration) (res float64, err error) {
	counter := cs.Get(category, key)

	if counter == nil {
		return res, fmt.Errorf("no counter found with category: %s, key: %s", category, key)
	}

	return counter.GetRate(lookback)
}

// Set inserts value at current timestamp
func (cs *Set) Set(category, key string, value any) {
	if cat, ok := cs.counter[category]; ok {
		if counter, ok := cat[key]; ok {
			counter.Set(value)

			return
		}
	}
}

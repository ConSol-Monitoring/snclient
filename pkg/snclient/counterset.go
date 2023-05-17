package snclient

type CounterSet struct {
	noCopy  noCopy
	counter map[string]map[string]*Counter
}

func NewCounerSet() *CounterSet {
	cs := &CounterSet{
		counter: make(map[string]map[string]*Counter),
	}

	return cs
}

func (cs *CounterSet) Create(category, key string, duration float64) {
	counter := NewCounter(duration)

	cat, ok := cs.counter[category]
	if !ok {
		cat = make(map[string]*Counter)
		cs.counter[category] = cat
	}
	cat[key] = counter
}

func (cs *CounterSet) Keys(category string) (keys []string) {
	if cat, ok := cs.counter[category]; ok {
		for key := range cat {
			keys = append(keys, key)
		}
	}

	return keys
}

func (cs *CounterSet) Get(category, key string) *Counter {
	if cat, ok := cs.counter[category]; ok {
		if counter, ok := cat[key]; ok {
			return counter
		}
	}

	return nil
}

func (cs *CounterSet) Set(category, key string, value float64) {
	if cat, ok := cs.counter[category]; ok {
		if counter, ok := cat[key]; ok {
			counter.Set(value)

			return
		}
	}

	log.Warnf("counter not found, must be created first (%s/%s)", category, key)
}

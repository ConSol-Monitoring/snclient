package snclient

type CounterSet struct {
	noCopy     noCopy
	counter    map[string]map[string]*Counter
	counterAny map[string]map[string]*CounterAny
}

func NewCounterSet() *CounterSet {
	cs := &CounterSet{
		counter:    make(map[string]map[string]*Counter),
		counterAny: make(map[string]map[string]*CounterAny),
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

func (cs *CounterSet) CreateAny(category, key string, duration float64) {
	counter := NewCounterAny(duration)

	cat, ok := cs.counterAny[category]
	if !ok {
		cat = make(map[string]*CounterAny)
		cs.counterAny[category] = cat
	}
	cat[key] = counter
}

func (cs *CounterSet) Delete(category, key string) {
	cat, ok := cs.counter[category]
	if ok {
		delete(cat, key)
	}
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

func (cs *CounterSet) SetAny(category, key string, value interface{}) {
	if cat, ok := cs.counterAny[category]; ok {
		if counter, ok := cat[key]; ok {
			counter.Set(value)

			return
		}
	}

	log.Warnf("counter not found, must be created first (%s/%s)", category, key)
}

func (cs *CounterSet) GetAny(category, key string) *CounterAny {
	if cat, ok := cs.counterAny[category]; ok {
		if counter, ok := cat[key]; ok {
			return counter
		}
	}

	return nil
}

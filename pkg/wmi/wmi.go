package wmi

// DefaultPoolSize is the default number of WMI queries that may run in
// parallel. A size <= 1 uses the single shared default client instead.
const DefaultPoolSize = 3

type Data struct {
	Key   string
	Value string
}

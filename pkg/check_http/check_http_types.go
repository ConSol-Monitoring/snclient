package check_http

import "errors"

type capWriter struct {
	buffer    []byte
	Cap       uint64
	size      uint64
	NoDiscard bool
}

func (w *capWriter) Write(data []byte) (int, error) {
	w.size += uint64(len(data))
	if w.size > w.Cap && w.NoDiscard {
		return 0, errors.New("could not write body buffer. buffer is full")
	}

	if w.size > w.Cap {
		q := w.Cap - uint64(len(w.buffer))
		if q != 0 {
			w.buffer = append(w.buffer, data[0:q-1]...)
		}
	} else {
		w.buffer = append(w.buffer, data...)
	}

	return len(data), nil
}

func (w *capWriter) Size() uint64 {
	return w.size
}

func (w *capWriter) Bytes() []byte {
	return w.buffer
}

//nolint:errname // The original author used it as an error type extensively
type CheckResult struct {
	resultImportance *int
	msg              string
	code             int
}

func (e *CheckResult) Error() string {
	return e.msg
}

func (e *CheckResult) Code() int {
	return e.code
}

// CheckResultPQ implements a max-heap (by exit code) of CheckResult pointers.
type CheckResultPQ []*CheckResult

func (pq *CheckResultPQ) Len() int { return len(*pq) }

// Less: In a heap, the element that is most "less" compared to others is on top.
// The highest severity (CRITICAL > WARNING > OK) is more important so it wins the "less" comparison.
// If severities are equal, check resultImportance, who has lower resultImportance is deemed more important and wins the "less" comparison.
func (pq *CheckResultPQ) Less(item1, item2 int) bool {
	if (*pq)[item1].Code() != (*pq)[item2].Code() {
		return (*pq)[item1].Code() > (*pq)[item2].Code()
	}

	if (*pq)[item1].resultImportance != nil && (*pq)[item2].resultImportance != nil {
		return *(*pq)[item1].resultImportance < *(*pq)[item2].resultImportance
	}

	return true
}

func (pq *CheckResultPQ) Swap(i, j int) {
	(*pq)[i], (*pq)[j] = (*pq)[j], (*pq)[i]
}

func (pq *CheckResultPQ) Push(x any) {
	cr, ok := x.(*CheckResult)
	if !ok {
		panic("CheckResultPQ.Push: unexpected type")
	}

	*pq = append(*pq, cr)
}

func (pq *CheckResultPQ) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]

	return item
}

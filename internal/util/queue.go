package util

import (
	"sync"
)

// Queue is a thread-safe queue
type Queue[T any] struct {
	sync.Mutex

	signalLock sync.Mutex
	signal     *sync.Cond
	notFull    *sync.Cond

	items    []T
	maxItems int
}

// NewQueue creates a new Queue with a maximum size
func NewQueue[T any](maxItems int) *Queue[T] {
	q := &Queue[T]{maxItems: maxItems}

	q.signal = sync.NewCond(&q.signalLock)
	q.notFull = sync.NewCond(&q.signalLock)
	return q
}

// Add adds an item to the queue, blocking if the queue is full
func (q *Queue[T]) Add(item T) {
	q.Lock()
	defer q.Unlock()

	// Block if the queue is full
	for len(q.items) >= q.maxItems {
		q.Unlock()
		q.signalLock.Lock()
		// wait until queue is not full
		q.notFull.Wait()
		q.signalLock.Unlock()
		q.Lock()
	}

	q.items = append(q.items, item)

	q.signalLock.Lock()
	defer q.signalLock.Unlock()
	// signal that queue is not empty
	q.signal.Signal()
}

// IsEmpty checks if the queue is empty
func (q *Queue[T]) IsEmpty() bool {
	q.Lock()
	defer q.Unlock()

	return len(q.items) == 0
}

// Grab returns the first item from the queue, blocking until the queue is not empty
func (q *Queue[T]) Grab() T {
	q.Lock()

	for len(q.items) == 0 {
		q.Unlock()

		q.signalLock.Lock()
		// wait until queue is not empty
		q.signal.Wait()
		q.signalLock.Unlock()

		q.Lock()
	}

	i := q.items[0]
	q.items = q.items[1:]

	q.Unlock()

	q.signalLock.Lock()
	defer q.signalLock.Unlock()
	// signal that queue is not full
	q.notFull.Signal()

	return i
}

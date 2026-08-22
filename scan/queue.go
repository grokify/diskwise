package scan

import (
	"container/heap"
	"sync"
)

// queueItem is one pending "expand this directory" work item.
type queueItem struct {
	node     *Node
	priority int64 // higher expands first
	seq      int64 // insertion order; breaks priority ties FIFO
	depth    int
}

// itemHeap is a max-heap on priority, with lower seq winning ties.
type itemHeap []*queueItem

func (h itemHeap) Len() int { return len(h) }
func (h itemHeap) Less(i, j int) bool {
	if h[i].priority != h[j].priority {
		return h[i].priority > h[j].priority
	}
	return h[i].seq < h[j].seq
}
func (h itemHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *itemHeap) Push(x any)   { *h = append(*h, x.(*queueItem)) }
func (h *itemHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return item
}

// priorityQueue is a thread-safe, size-prioritized work queue: workers
// blocked in Pop wake as soon as an item is pushed or the queue is
// closed. It is the mechanism behind DiskWise's prioritized descent —
// directories that look bigger (by already-counted direct bytes) are
// expanded before smaller ones discovered earlier.
type priorityQueue struct {
	mu      sync.Mutex
	cond    *sync.Cond
	items   itemHeap
	nextSeq int64
	closed  bool
}

func newPriorityQueue() *priorityQueue {
	q := &priorityQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *priorityQueue) Push(node *Node, priority int64, depth int) {
	q.mu.Lock()
	item := &queueItem{node: node, priority: priority, seq: q.nextSeq, depth: depth}
	q.nextSeq++
	heap.Push(&q.items, item)
	q.mu.Unlock()
	q.cond.Signal()
}

// Pop blocks until an item is available or the queue is closed and
// drained, in which case ok is false.
func (q *priorityQueue) Pop() (item *queueItem, ok bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}
	if len(q.items) == 0 {
		return nil, false
	}
	return heap.Pop(&q.items).(*queueItem), true
}

// Close signals that no more items will be pushed; blocked and future
// Pop calls return ok=false once the queue is drained.
func (q *priorityQueue) Close() {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	q.cond.Broadcast()
}

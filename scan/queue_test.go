package scan

import (
	"testing"
	"time"
)

// TestPriorityQueue_OrdersByPriorityDescending verifies the mechanism
// behind DiskWise's prioritized descent: items pop largest-priority
// first, regardless of push order.
func TestPriorityQueue_OrdersByPriorityDescending(t *testing.T) {
	q := newPriorityQueue()
	for _, p := range []int64{5, 1, 10, 3} {
		q.Push(&Node{}, p, 0)
	}
	q.Close()

	var got []int64
	for {
		item, ok := q.Pop()
		if !ok {
			break
		}
		got = append(got, item.priority)
	}

	want := []int64{10, 5, 3, 1}
	if len(got) != len(want) {
		t.Fatalf("popped %d items, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pop order = %v, want %v", got, want)
		}
	}
}

func TestPriorityQueue_FIFOTiebreak(t *testing.T) {
	q := newPriorityQueue()
	for _, name := range []string{"a", "b", "c"} {
		q.Push(&Node{Name: name}, 5, 0)
	}
	q.Close()

	var got []string
	for {
		item, ok := q.Pop()
		if !ok {
			break
		}
		got = append(got, item.node.Name)
	}

	want := []string{"a", "b", "c"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("pop order = %v, want %v", got, want)
		}
	}
}

func TestPriorityQueue_PopBlocksUntilPush(t *testing.T) {
	q := newPriorityQueue()
	done := make(chan *queueItem, 1)
	go func() {
		item, ok := q.Pop()
		if !ok {
			item = nil
		}
		done <- item
	}()

	select {
	case <-done:
		t.Fatal("Pop returned before any item was pushed")
	case <-time.After(50 * time.Millisecond):
	}

	q.Push(&Node{Name: "x"}, 1, 0)

	select {
	case item := <-done:
		if item == nil || item.node.Name != "x" {
			t.Fatalf("got %v, want node x", item)
		}
	case <-time.After(time.Second):
		t.Fatal("Pop did not return after Push")
	}
}

func TestPriorityQueue_PopUnblocksOnClose(t *testing.T) {
	q := newPriorityQueue()
	done := make(chan bool, 1)
	go func() {
		_, ok := q.Pop()
		done <- ok
	}()

	select {
	case <-done:
		t.Fatal("Pop returned before Close")
	case <-time.After(50 * time.Millisecond):
	}

	q.Close()

	select {
	case ok := <-done:
		if ok {
			t.Fatal("Pop should return ok=false after Close with no pending items")
		}
	case <-time.After(time.Second):
		t.Fatal("Pop did not unblock after Close")
	}
}

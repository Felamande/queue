package queue

import "testing"

func TestQueuePriority(t *testing.T) {
	q := NewPriorityQueue[int]()
	q.Push(3, 2)
	q.Push(2, 1)
	v, p, ok := q.Pop()
	if !ok {
		t.Fatal("cannot pop item")
	}
	if v != 2 {
		t.Fatal("expected value 2 get", v)
	}

	if p != 1 {
		t.Fatal("expected priority 1 get", p)

	}
}

func BenchmarkQueuePushPop(b *testing.B) {
	q := NewPriorityQueue[int]()
	for i := 0; i < b.N; i++ {
		q.Push(i, i)
		q.Pop()
	}
}

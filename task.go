package queue

import (
	"errors"
	"sync"
	"sync/atomic"
)

var LowerPriority error = errors.New("lower priority")
var HigherPriority error = errors.New("upper priority")
var EqualPriority error = errors.New("equal priority")

// var Discard error = errors.New("discard")

type WorkUnit[T any] interface {
	Value() T
	Priority() int
	QueueId() int
	CancelWorker()
	TaskId() int
}

type workUnit[T any] struct {
	elem     T
	priority int
	qId      int
	tid      int
	cancel   func()
}

func (w *workUnit[T]) Value() T {
	return w.elem
}

func (w *workUnit[T]) Priority() int {
	return w.priority
}

func (w *workUnit[T]) QueueId() int {
	return w.qId
}

func (w *workUnit[T]) TaskId() int {
	return w.tid
}

func (w *workUnit[T]) CancelWorker() {
	w.cancel()
}

type work[T, R any] struct {
	elem     T
	workerFn func(w WorkUnit[T]) (result R, err error)
}

type Result[T any] struct {
	Result T
	Error  error
}

type Task[T, R any] struct {
	workers int

	queue *Queue[work[T, R]]

	resultCh chan Result[R]

	chs []chan struct{}

	wg sync.WaitGroup
}

func NewTask[T, R any](parallelWorkers int) *Task[T, R] {
	t := &Task[T, R]{
		queue:    NewQueue[work[T, R]](),
		workers:  parallelWorkers,
		resultCh: make(chan Result[R], parallelWorkers),
		chs:      make([]chan struct{}, parallelWorkers),
	}

	for i := 0; i < parallelWorkers; i++ {
		t.chs[i] = make(chan struct{})
	}

	t.wg.Add(t.workers)

	go func() {
		for i := 0; i < t.workers; i++ {
			go func(idx int) {
				defer t.wg.Done()
				var taskId int64
			loop:
				for {
					select {
					case <-t.chs[idx]:
						return
					default:
						v, p, err := t.queue.Get()
						if errors.Is(err, ErrorComplete) {

							return
						}
						if err != nil {
							continue loop
						}
						if v.workerFn == nil {
							continue loop
						}

						atomic.AddInt64(&taskId, 1)
						r, err := v.workerFn(&workUnit[T]{elem: v.elem, priority: p, qId: idx, tid: int(taskId), cancel: func() { t.chs[idx] <- struct{}{} }})

						if err == HigherPriority {
							t.queue.Add(work[T, R]{elem: v.elem, workerFn: v.workerFn}, p-1)
						} else if err == LowerPriority {
							t.queue.Add(work[T, R]{elem: v.elem, workerFn: v.workerFn}, p+1)
						} else if err == EqualPriority {
							t.queue.Add(work[T, R]{elem: v.elem, workerFn: v.workerFn}, p)
						} else {
							t.resultCh <- Result[R]{Result: r, Error: err}
						}
					}

				}
			}(i)
		}
	}()
	return t

}

func (t *Task[T, R]) startWorker() {
}

func (t *Task[T, R]) restartWorkerOnPanic(i int) {}

func (t *Task[T, R]) Cancel() {
	for _, ch := range t.chs {
		go func() {
			ch <- struct{}{}
		}()
	}
	t.queue.Complete()
	close(t.resultCh)
}

func (t *Task[T, R]) Results() chan Result[R] {
	return t.resultCh
}

func (t *Task[T, R]) Complete() {
	t.queue.Complete()
}

func (t *Task[T, R]) Wait() {
	for range t.resultCh {
	}
}

func (t *Task[T, R]) WaitFor(f func(result Result[R]) bool) {
	for r := range t.resultCh {
		if f(r) {
			t.Cancel()
		}
	}
}

func (t *Task[T, R]) Queue(elem T, priority int, fn func(w WorkUnit[T]) (result R, err error)) {
	t.queue.Add(work[T, R]{elem, fn}, priority)
}

func (t *Task[T, R]) Len() int {
	return t.queue.Len()
}

package queue

import (
	"container/heap"
	"errors"
	"sync"
)

var (
	ErrorComplete = errors.New("queue is complete")
	ErrorEmpty    = errors.New("queue is empty")
)

// item 代表队列中的元素及其优先级
type item[T any] struct {
	value    T
	priority int
}

// priorityQueueIf 实现了一个优先级队列
type priorityQueueIf[T any] []*item[T]

// Len 返回队列长度
func (pq priorityQueueIf[T]) Len() int { return len(pq) }

// Less 比较两个元素的优先级，优先级高的在前
func (pq priorityQueueIf[T]) Less(i, j int) bool {
	return pq[i].priority < pq[j].priority // priority值越大，优先级越低
}

// Swap 交换两个元素的位置
func (pq priorityQueueIf[T]) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

// Push 将一个元素加入队列
func (pq *priorityQueueIf[T]) Push(x any) {
	*pq = append(*pq, x.(*item[T]))
}

// Pop 移除并返回队列中的最优先元素
func (pq *priorityQueueIf[T]) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

type PriorityQueue[T any] struct {
	pq   priorityQueueIf[T]
	pool sync.Pool
}

func NewPriorityQueue[T any]() *PriorityQueue[T] {
	pq := &PriorityQueue[T]{}
	pq.pool.New = func() any {
		return &item[T]{}
	}
	heap.Init(&pq.pq)
	return pq
}

func (pq *PriorityQueue[T]) Push(x T, priority int) {
	item := pq.pool.Get().(*item[T])
	item.value = x
	item.priority = priority
	heap.Push(&pq.pq, item)
}

func (pq *PriorityQueue[T]) Pop() (value T, priority int, ok bool) {
	if len(pq.pq) == 0 {
		var zero T
		return zero, 0, false
	}
	item := heap.Pop(&pq.pq).(*item[T])
	value, priority = item.value, item.priority
	pq.pool.Put(item)
	return value, priority, true
}

func (pq *PriorityQueue[T]) Len() int {
	return len(pq.pq)
}

// Queue 结构体
type Queue[T any] struct {
	mu       sync.Mutex
	cond     *sync.Cond
	pq       *PriorityQueue[T]
	complete bool
}

// NewQueue 创建一个新的优先级队列
func NewQueue[T any]() *Queue[T] {
	q := &Queue[T]{}
	q.cond = sync.NewCond(&q.mu)
	q.pq = NewPriorityQueue[T]()
	return q
}

// Add 添加元素到队列
func (q *Queue[T]) Add(value T, priority int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.pq.Push(value, priority)
	q.cond.Signal() // 通知等待的 goroutine
}

// Get 获取队列中优先级最高的元素
func (q *Queue[T]) Get() (value T, priority int, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for q.pq.Len() == 0 {
		if q.complete {
			var zero T
			return zero, 0, ErrorComplete
		}
		q.cond.Wait() // 等待新元素
	}
	v, p, _ := q.pq.Pop()
	return v, p, nil
}

func (q *Queue[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.pq.Len()
}

// Complete 标记队列为完成
func (q *Queue[T]) Complete() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.complete = true
	q.cond.Broadcast() // 唤醒所有等待的 goroutine
}

func (q *Queue[T]) Wait() {
	q.mu.Lock()
	defer q.mu.Unlock()

	for q.pq.Len() > 0 || !q.complete {
		q.cond.Wait() // Wait until the queue is empty
	}
}

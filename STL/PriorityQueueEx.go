package STL

import (
	"container/heap"
	"fmt"
	"strconv"
	"sync"
	"time"
)

const (
	lhsValueOfMaxTimerInSecond = 60
)

type PriorityQueue []*Item

type Item struct {
	Sid      interface{}
	Priority int64
	Index    int
	Data     interface{}
}

func (pq PriorityQueue) Len() int { return len(pq) }
func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].Priority < pq[j].Priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].Index = i
	pq[j].Index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*Item)
	item.Index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	item.Index = -1
	return item
}

func (pq *PriorityQueue) Init() {
	heap.Init(pq)
}

type PriorityQueueEx struct {
	mu       sync.Mutex
	dt       map[string]*Item
	pq       PriorityQueue
	kg       func(interface{}) string
	maxTimer int64
}

func (t *PriorityQueueEx) Init(maxTimer int64, k func(interface{}) string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.pq.Init()
	t.maxTimer = maxTimer
	if t.maxTimer < lhsValueOfMaxTimerInSecond {
		t.maxTimer = lhsValueOfMaxTimerInSecond
	}
	t.kg = k
	if t.kg == nil {
		t.kg = func(sid interface{}) string {
			switch ty := sid.(type) {
			case uint64:
				return strconv.FormatUint(ty, 10)
			case string:
				return ty
			default:
				typeStr := fmt.Sprintf("%T", ty)
				panic("unsupported key type" + typeStr)
			}
		}
	}
	t.dt = make(map[string]*Item)
}

func (t *PriorityQueueEx) Get(sid interface{}) interface{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := t.kg(sid)
	d, o := t.dt[key]
	if !o {
		return nil
	}
	return d
}

func (t *PriorityQueueEx) Exist(sid interface{}) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	key := t.kg(sid)
	_, o := t.dt[key]
	return o
}

func (t *PriorityQueueEx) _maxTp() int64 {
	return time.Now().Unix() + t.maxTimer
}

func (t *PriorityQueueEx) _validMaxTimer(priority int64) int64 {
	maxTp := t._maxTp()
	if priority > maxTp {
		return maxTp
	}
	return priority
}

func (t *PriorityQueueEx) _invalidTimerNode(priority int64) bool {
	maxTp := t._maxTp()
	return priority > maxTp
}

func (t *PriorityQueueEx) Push(priority int64, sid interface{}, data interface{}) bool {
	priority = t._validMaxTimer(priority)
	item := &Item{Priority: priority, Sid: sid, Index: -1, Data: data}
	t.mu.Lock()
	defer t.mu.Unlock()
	key := t.kg(sid)
	_, o := t.dt[key]
	if o {
		return false
	}
	// Push
	t.dt[key] = item
	heap.Push(&t.pq, item)
	return true
}

func (t *PriorityQueueEx) Update(priority int64, sid interface{}) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	priority = t._validMaxTimer(priority)
	key := t.kg(sid)
	d, o := t.dt[key]
	if o {
		// Fix
		d.Sid = sid
		d.Priority = priority
		heap.Fix(&t.pq, d.Index)
	}

	return o
}

func (t *PriorityQueueEx) Pop(maxPriority int64) interface{} {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.pq) == 0 {
		if len(t.dt) != 0 {
			t.dt = make(map[string]*Item)
		}
		return nil
	}

	if t.pq[0].Priority < maxPriority || t._invalidTimerNode(t.pq[0].Priority) {
		v := heap.Pop(&t.pq)
		d, o := v.(*Item)
		if o {
			key := t.kg(d.Sid)
			delete(t.dt, key)
			return d.Data
		}
	}
	return nil
}

func (t *PriorityQueueEx) Remove(sid interface{}) (bool, interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := t.kg(sid)
	d, o := t.dt[key]

	if !o {
		return false, nil
	}

	index := -1
	if d != nil {
		index = d.Index
	}

	delete(t.dt, key)

	var removed interface{} = nil

	if index >= 0 {
		removed = heap.Remove(&t.pq, index)
	}

	return true, removed
}

func (t *PriorityQueueEx) Len() (int, *Item) {
	t.mu.Lock()
	defer t.mu.Unlock()

	pqLen := len(t.pq)
	if pqLen > 0 {
		return pqLen, t.pq[0]
	}
	return len(t.pq), nil
}

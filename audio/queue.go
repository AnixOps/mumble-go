package audio

import "sync"

// Packet is a queued audio frame or packet.
type Packet struct {
	Session   uint32
	Sequence  uint64
	Payload   []byte
	Timestamp int64
}

// Queue is a small bounded FIFO for audio work.
type Queue struct {
	mu    sync.Mutex
	limit int
	items []Packet
}

func NewQueue(limit int) *Queue {
	return &Queue{limit: limit}
}

func (q *Queue) Push(p Packet) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.limit > 0 && len(q.items) >= q.limit {
		q.items = q.items[1:]
	}
	q.items = append(q.items, p)
}

func (q *Queue) Pop() (Packet, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return Packet{}, false
	}
	p := q.items[0]
	q.items = q.items[1:]
	return p, true
}

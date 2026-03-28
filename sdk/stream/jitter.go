package stream

import (
	"sync"
	"time"
)

// JitterBuffer smooths audio delivery by buffering frames and accounting
// for network jitter. Frames that arrive too late are replaced with silence.
type JitterBuffer struct {
	mu       sync.Mutex
	depth    int              // max buffer depth in frames
	maxDelay time.Duration    // max time a frame can be late before being dropped
	frames   []jitterFrame    // ordered by expected time
	silence  []byte           // pre-allocated silence frame (960 samples * 2 bytes)
	closed   bool
}

type jitterFrame struct {
	pcm      []byte
	deadline time.Time // when this frame should be played
}

// NewJitterBuffer creates a JitterBuffer with the given depth and frame duration.
func NewJitterBuffer(depth int, frameDuration time.Duration) *JitterBuffer {
	frameBytes := 960 * 2 // 20ms at 48kHz mono, 16-bit
	return &JitterBuffer{
		depth:    depth,
		maxDelay: frameDuration * time.Duration(depth+1),
		silence:  make([]byte, frameBytes),
	}
}

// Push adds a PCM frame to the buffer. The deadline indicates when the frame
// should be played. If the frame is too late (deadline + maxDelay < now), it is
// dropped.
func (j *JitterBuffer) Push(pcm []byte, deadline time.Time) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.closed {
		return
	}

	now := time.Now()

	// Drop if too late
	if deadline.Add(j.maxDelay).Before(now) {
		return
	}

	// Remove old frames that are past their deadline
	var valid []jitterFrame
	for _, f := range j.frames {
		if f.deadline.After(now) {
			valid = append(valid, f)
		}
	}
	j.frames = valid

	// Insert in sorted order (by deadline)
	frame := jitterFrame{pcm: pcm, deadline: deadline}
	inserted := false
	for i, f := range j.frames {
		if deadline.Before(f.deadline) {
			j.frames = append(j.frames[:i], append([]jitterFrame{frame}, j.frames[i:]...)...)
			inserted = true
			break
		}
	}
	if !inserted {
		j.frames = append(j.frames, frame)
	}

	// Limit buffer size
	if len(j.frames) > j.depth*2 {
		j.frames = j.frames[len(j.frames)-j.depth:]
	}
}

// Pop returns the next frame if it's due. If no frame is ready, it returns
// a silence frame. The boolean is false if the buffer is exhausted or closed.
func (j *JitterBuffer) Pop(now time.Time) ([]byte, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.closed || len(j.frames) == 0 {
		return j.silence, false
	}

	frame := j.frames[0]

	// Not yet due — return silence but keep the frame
	if frame.deadline.After(now) {
		return j.silence, true
	}

	j.frames = j.frames[1:]
	return frame.pcm, true
}

// Depth returns the current number of buffered frames.
func (j *JitterBuffer) Depth() int {
	j.mu.Lock()
	defer j.mu.Unlock()
	return len(j.frames)
}

// Close stops the buffer.
func (j *JitterBuffer) Close() {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.closed = true
	j.frames = nil
}

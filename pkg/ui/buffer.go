package ui

import (
	"strings"
	"sync"
	"github.com/loganalyzer/traceace/pkg/models"
)

// CircularBuffer is a high-performance circular buffer for log lines
type CircularBuffer struct {
	data     []*models.LogLine
	head     int
	tail     int
	size     int
	capacity int
	mu       sync.RWMutex
}

// NewCircularBuffer creates a new circular buffer with the given capacity
func NewCircularBuffer(capacity int) *CircularBuffer {
	return &CircularBuffer{
		data:     make([]*models.LogLine, capacity),
		capacity: capacity,
	}
}

// Add adds a new log line to the buffer
func (cb *CircularBuffer) Add(line *models.LogLine) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.data[cb.tail] = line
	cb.tail = (cb.tail + 1) % cb.capacity
	
	if cb.size < cb.capacity {
		cb.size++
	} else {
		// Buffer is full, advance head
		cb.head = (cb.head + 1) % cb.capacity
	}
}

// Get returns the log line at the given index (0-based from oldest)
func (cb *CircularBuffer) Get(index int) *models.LogLine {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	if index < 0 || index >= cb.size {
		return nil
	}
	
	actualIndex := (cb.head + index) % cb.capacity
	return cb.data[actualIndex]
}

// Size returns the current number of elements in the buffer
func (cb *CircularBuffer) Size() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.size
}

// GetRange returns a slice of log lines from start to end (exclusive)
func (cb *CircularBuffer) GetRange(start, end int) []*models.LogLine {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	if start < 0 || start >= cb.size || end <= start {
		return nil
	}
	
	if end > cb.size {
		end = cb.size
	}
	
	result := make([]*models.LogLine, 0, end-start)
	for i := start; i < end; i++ {
		actualIndex := (cb.head + i) % cb.capacity
		result = append(result, cb.data[actualIndex])
	}
	
	return result
}

// GetLast returns the last n log lines
func (cb *CircularBuffer) GetLast(n int) []*models.LogLine {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	if n <= 0 || cb.size == 0 {
		return nil
	}
	
	if n > cb.size {
		n = cb.size
	}
	
	start := cb.size - n
	return cb.GetRange(start, cb.size)
}

// Clear clears the buffer
func (cb *CircularBuffer) Clear() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	
	cb.head = 0
	cb.tail = 0
	cb.size = 0
	
	// Clear references for GC
	for i := range cb.data {
		cb.data[i] = nil
	}
}

// ForEach applies a function to each element in the buffer
func (cb *CircularBuffer) ForEach(fn func(*models.LogLine) bool) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	
	for i := 0; i < cb.size; i++ {
		actualIndex := (cb.head + i) % cb.capacity
		if !fn(cb.data[actualIndex]) {
			break
		}
	}
}

// ObjectPool provides pooling for LogLine objects to reduce allocations
type ObjectPool struct {
	logLinePool   sync.Pool
	stringBuilder sync.Pool
}

// NewObjectPool creates a new object pool
func NewObjectPool() *ObjectPool {
	return &ObjectPool{
		logLinePool: sync.Pool{
			New: func() interface{} {
				return &models.LogLine{
					Parsed: make(map[string]interface{}),
					Tokens: make([]models.Token, 0, 10),
				}
			},
		},
		stringBuilder: sync.Pool{
			New: func() interface{} {
				return &strings.Builder{}
			},
		},
	}
}

// GetLogLine gets a LogLine from the pool
func (p *ObjectPool) GetLogLine() *models.LogLine {
	line := p.logLinePool.Get().(*models.LogLine)
	// Reset the line
	*line = models.LogLine{
		Parsed: make(map[string]interface{}),
		Tokens: line.Tokens[:0], // Keep capacity, reset length
	}
	return line
}

// PutLogLine returns a LogLine to the pool
func (p *ObjectPool) PutLogLine(line *models.LogLine) {
	if line != nil {
		p.logLinePool.Put(line)
	}
}

// GetStringBuilder gets a StringBuilder from the pool
func (p *ObjectPool) GetStringBuilder() *strings.Builder {
	builder := p.stringBuilder.Get().(*strings.Builder)
	builder.Reset()
	return builder
}

// PutStringBuilder returns a StringBuilder to the pool
func (p *ObjectPool) PutStringBuilder(builder *strings.Builder) {
	if builder != nil {
		p.stringBuilder.Put(builder)
	}
}

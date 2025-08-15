package ui

import (
	"fmt"
	"sync"
	"time"

	"github.com/loganalyzer/traceace/pkg/models"
)

// SimpleBatcher processes logs in simple 1000-line batches
type SimpleBatcher struct {
	pendingLines [](*models.LogLine)
	batchSize    int
	mutex        sync.Mutex
	lastProcess  time.Time
	timeout      time.Duration
}

// NewSimpleBatcher creates a new simple batcher
func NewSimpleBatcher() *SimpleBatcher {
	return &SimpleBatcher{
		pendingLines: make([]*models.LogLine, 0, 1000),
		batchSize:    1000,
		timeout:      100 * time.Millisecond, // Process batch every 100ms if not full
	}
}

// AddLine adds a line to the batch and processes when batch is full
func (sb *SimpleBatcher) AddLine(line *models.LogLine, m *Model) {
	sb.mutex.Lock()
	defer sb.mutex.Unlock()
	
	sb.pendingLines = append(sb.pendingLines, line)
	
	// Process batch if it's full or timeout reached
	now := time.Now()
	if len(sb.pendingLines) >= sb.batchSize || 
	   (len(sb.pendingLines) > 0 && now.Sub(sb.lastProcess) >= sb.timeout) {
		sb.processBatch(m)
	}
}

// processBatch processes the current batch of lines
func (sb *SimpleBatcher) processBatch(m *Model) {
	if len(sb.pendingLines) == 0 {
		return
	}
	
	startTime := time.Now()
	matchedCount := 0
	
	// Process all lines in the batch at once
	for _, line := range sb.pendingLines {
		// Add to all lines buffer
		m.allLinesBuffer.Add(line)
		
		// Check if filter is active and line matches
		if m.filter.HasFilter() && m.filter.Match(line) {
			m.filteredBuffer.Add(line)
			matchedCount++
		}
	}
	
	// Show batch processing stats
	if len(sb.pendingLines) >= sb.batchSize {
		duration := time.Since(startTime)
		if duration.Milliseconds() > 0 {
			linesPerSec := int64(float64(len(sb.pendingLines)) / duration.Seconds())
			m.setStatusMessage(fmt.Sprintf("⚡ Batch: %d lines, %d matches (%dk lines/sec)", 
				len(sb.pendingLines), matchedCount, linesPerSec/1000))
		} else {
			m.setStatusMessage(fmt.Sprintf("⚡ Batch: %d lines, %d matches (instant)", 
				len(sb.pendingLines), matchedCount))
		}
	}
	
	// Clear the batch
	sb.pendingLines = sb.pendingLines[:0]
	sb.lastProcess = time.Now()
}

// ForceBatch forces processing of any remaining lines
func (sb *SimpleBatcher) ForceBatch(m *Model) {
	sb.mutex.Lock()
	defer sb.mutex.Unlock()
	sb.processBatch(m)
}

// ProcessAllExistingLines processes all existing lines in 1000-line batches
func (m *Model) ProcessAllExistingLines() error {
	if !m.filter.HasFilter() {
		m.filteredBuffer.Clear()
		return nil
	}
	
	totalLines := m.allLinesBuffer.Size()
	if totalLines == 0 {
		return nil
	}
	
	startTime := time.Now()
	m.setStatusMessage(fmt.Sprintf("⚡ Processing %d existing lines in batches...", totalLines))
	
	// Clear filtered buffer first
	m.filteredBuffer.Clear()
	
	// Process in batches of 1000 lines
	batchSize := 1000
	totalMatches := 0
	
	for start := 0; start < totalLines; start += batchSize {
		end := start + batchSize
		if end > totalLines {
			end = totalLines
		}
		
		// Process this batch
		batchMatches := 0
		for i := start; i < end; i++ {
			line := m.allLinesBuffer.Get(i)
			if line != nil && m.filter.Match(line) {
				m.filteredBuffer.Add(line)
				batchMatches++
				totalMatches++
			}
		}
		
		// Update progress for large batches
		if totalLines > 5000 && start > 0 && start%5000 == 0 {
			progress := int(float64(start) / float64(totalLines) * 100)
			m.setStatusMessage(fmt.Sprintf("⚡ Processing... %d%% (%d matches so far)", 
				progress, totalMatches))
		}
	}
	
	// Final result
	duration := time.Since(startTime)
	if duration.Milliseconds() > 0 {
		linesPerSec := int64(float64(totalLines) / duration.Seconds())
		m.setStatusMessage(fmt.Sprintf("⚡ Found %d/%d matches in %v (%dk lines/sec)", 
			totalMatches, totalLines, duration.Round(time.Millisecond), linesPerSec/1000))
	} else {
		m.setStatusMessage(fmt.Sprintf("⚡ Found %d/%d matches instantly!", 
			totalMatches, totalLines))
	}
	
	return nil
}

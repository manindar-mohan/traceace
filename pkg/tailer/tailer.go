package tailer

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/hpcloud/tail"
	"github.com/loganalyzer/traceace/pkg/models"
)

// Tailer represents a file tailer that can monitor files for changes
type Tailer struct {
	mu          sync.RWMutex
	files       map[string]*FileWatcher
	events      chan models.TailerEvent
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	rotationCheckInterval time.Duration
}

// FileWatcher represents a single file being watched
type FileWatcher struct {
	path          string
	file          *os.File
	tail          *tail.Tail
	lastOffset    int64
	lastSize      int64
	lastModTime   time.Time
	lineCounter   int
	isRotating    bool
	mu           sync.RWMutex
}

// New creates a new Tailer instance
func New(ctx context.Context) *Tailer {
	ctx, cancel := context.WithCancel(ctx)
	
	return &Tailer{
		files:                 make(map[string]*FileWatcher),
		events:                make(chan models.TailerEvent, 1000),
		ctx:                   ctx,
		cancel:                cancel,
		rotationCheckInterval: time.Second,
	}
}

// AddFile adds a file to be tailed
func (t *Tailer) AddFile(filePath string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// Check if file already being watched
	if _, exists := t.files[filePath]; exists {
		return fmt.Errorf("file %s is already being watched", filePath)
	}
	
	// Check if file exists and is readable
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("cannot access file %s: %w", filePath, err)
	}
	
	watcher := &FileWatcher{
		path: filePath,
	}
	
	// Initialize file info
	if err := watcher.updateFileInfo(); err != nil {
		return fmt.Errorf("failed to get file info for %s: %w", filePath, err)
	}
	
	// Start tailing the file
	if err := watcher.startTail(); err != nil {
		return fmt.Errorf("failed to start tailing %s: %w", filePath, err)
	}
	
	t.files[filePath] = watcher
	
	// Start monitoring this file
	t.wg.Add(1)
	go t.monitorFile(watcher)
	
	return nil
}

// RemoveFile stops watching a file
func (t *Tailer) RemoveFile(filePath string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	watcher, exists := t.files[filePath]
	if !exists {
		return fmt.Errorf("file %s is not being watched", filePath)
	}
	
	// Stop the tail
	if watcher.tail != nil {
		watcher.tail.Stop()
	}
	
	// Close the file
	if watcher.file != nil {
		watcher.file.Close()
	}
	
	delete(t.files, filePath)
	
	return nil
}

// Events returns the channel for receiving tailer events
func (t *Tailer) Events() <-chan models.TailerEvent {
	return t.events
}

// Stop stops the tailer and all file watchers
func (t *Tailer) Stop() {
	t.cancel()
	t.wg.Wait()
	close(t.events)
	
	t.mu.Lock()
	defer t.mu.Unlock()
	
	for _, watcher := range t.files {
		if watcher.tail != nil {
			watcher.tail.Stop()
		}
		if watcher.file != nil {
			watcher.file.Close()
		}
	}
}

// GetWatchedFiles returns the list of files currently being watched
func (t *Tailer) GetWatchedFiles() []string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	files := make([]string, 0, len(t.files))
	for path := range t.files {
		files = append(files, path)
	}
	return files
}

// updateFileInfo updates the file information for rotation detection
func (fw *FileWatcher) updateFileInfo() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	
	info, err := os.Stat(fw.path)
	if err != nil {
		return err
	}
	
	fw.lastSize = info.Size()
	fw.lastModTime = info.ModTime()
	
	return nil
}

// startTail starts tailing the file
func (fw *FileWatcher) startTail() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	
	config := tail.Config{
		Follow:    true,
		ReOpen:    true,
		MustExist: true,
		Poll:      true,
		Location:  &tail.SeekInfo{Offset: 0, Whence: io.SeekStart},
	}
	
	t, err := tail.TailFile(fw.path, config)
	if err != nil {
		return err
	}
	
	fw.tail = t
	return nil
}

// checkRotation checks if the file has been rotated
func (fw *FileWatcher) checkRotation() (bool, error) {
	fw.mu.RLock()
	path := fw.path
	lastSize := fw.lastSize
	fw.mu.RUnlock()
	
	info, err := os.Stat(path)
	if err != nil {
		// File might have been deleted/rotated
		return true, nil
	}
	
	currentSize := info.Size()
	
	// Check if file size decreased (likely rotated) or if file is newer than expected
	if currentSize < lastSize || info.ModTime().After(fw.lastModTime.Add(time.Minute)) {
		return true, nil
	}
	
	return false, nil
}

// handleRotation handles file rotation
func (fw *FileWatcher) handleRotation() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	
	fw.isRotating = true
	defer func() { fw.isRotating = false }()
	
	// Stop current tail
	if fw.tail != nil {
		fw.tail.Stop()
		fw.tail.Cleanup()
	}
	
	// Close current file
	if fw.file != nil {
		fw.file.Close()
	}
	
	// Update file info
	if err := fw.updateFileInfo(); err != nil {
		return err
	}
	
	// Restart tailing
	return fw.startTail()
}

// monitorFile monitors a single file for changes and rotation
func (t *Tailer) monitorFile(watcher *FileWatcher) {
	defer t.wg.Done()
	
	ticker := time.NewTicker(t.rotationCheckInterval)
	defer ticker.Stop()
	
	// Start reading lines from the tail
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()
		for {
			select {
			case line := <-watcher.tail.Lines:
				if line == nil {
					return
				}
				
				if line.Err != nil {
					select {
					case t.events <- models.TailerEvent{
						Type:    models.EventFileError,
						Source:  watcher.path,
						Error:   line.Err,
						Message: fmt.Sprintf("Error reading from %s", watcher.path),
					}:
					case <-t.ctx.Done():
						return
					}
					continue
				}
				
				watcher.mu.Lock()
				watcher.lineCounter++
				lineNum := watcher.lineCounter
				watcher.mu.Unlock()
				
				// Create log line
				logLine := &models.LogLine{
					ID:      fmt.Sprintf("%s:%d", watcher.path, lineNum),
					Source:  watcher.path,
					Raw:     line.Text,
					LineNum: lineNum,
					Offset:  watcher.lastOffset,
				}
				
				// Set timestamp to current time initially
				logLine.Timestamp = time.Now()
				
				select {
				case t.events <- models.TailerEvent{
					Type:   models.EventNewLine,
					Source: watcher.path,
					Line:   logLine,
				}:
				case <-t.ctx.Done():
					return
				}
				
			case <-t.ctx.Done():
				return
			}
		}
	}()
	
	// Monitor for rotation
	for {
		select {
		case <-ticker.C:
			// Skip rotation check if currently rotating
			watcher.mu.RLock()
			isRotating := watcher.isRotating
			watcher.mu.RUnlock()
			
			if isRotating {
				continue
			}
			
			rotated, err := watcher.checkRotation()
			if err != nil {
				select {
				case t.events <- models.TailerEvent{
					Type:    models.EventFileError,
					Source:  watcher.path,
					Error:   err,
					Message: fmt.Sprintf("Error checking rotation for %s", watcher.path),
				}:
				case <-t.ctx.Done():
					return
				}
				continue
			}
			
			if rotated {
				// Send rotation event
				select {
				case t.events <- models.TailerEvent{
					Type:    models.EventFileRotated,
					Source:  watcher.path,
					Message: fmt.Sprintf("File %s has been rotated", watcher.path),
				}:
				case <-t.ctx.Done():
					return
				}
				
				// Handle the rotation
				if err := watcher.handleRotation(); err != nil {
					select {
					case t.events <- models.TailerEvent{
						Type:    models.EventFileError,
						Source:  watcher.path,
						Error:   err,
						Message: fmt.Sprintf("Error handling rotation for %s", watcher.path),
					}:
					case <-t.ctx.Done():
						return
					}
				}
			}
			
		case <-t.ctx.Done():
			return
		}
	}
}

// TailFromStart starts tailing a file from the beginning instead of the end
func (t *Tailer) TailFromStart(filePath string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// Remove if already watching
	if watcher, exists := t.files[filePath]; exists {
		if watcher.tail != nil {
			watcher.tail.Stop()
		}
		if watcher.file != nil {
			watcher.file.Close()
		}
		delete(t.files, filePath)
	}
	
	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("cannot access file %s: %w", filePath, err)
	}
	
	watcher := &FileWatcher{
		path: filePath,
	}
	
	// Initialize file info
	if err := watcher.updateFileInfo(); err != nil {
		return fmt.Errorf("failed to get file info for %s: %w", filePath, err)
	}
	
	// Configure to start from beginning
	config := tail.Config{
		Follow:    true,
		ReOpen:    true,
		MustExist: true,
		Poll:      true,
		Location:  &tail.SeekInfo{Offset: 0, Whence: io.SeekStart},
	}
	
	tail, err := tail.TailFile(filePath, config)
	if err != nil {
		return fmt.Errorf("failed to start tailing %s: %w", filePath, err)
	}
	
	watcher.tail = tail
	t.files[filePath] = watcher
	
	// Start monitoring this file
	t.wg.Add(1)
	go t.monitorFile(watcher)
	
	return nil
}

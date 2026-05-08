package notary

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// TamperEvent 篡改/变更事件
type TamperEvent struct {
	Path      string    `json:"path"`
	EventType string    `json:"eventType"` // WRITE / REMOVE / RENAME / CHMOD
	Timestamp time.Time `json:"timestamp"`
}

// FileWatcher 文件系统监控
type FileWatcher struct {
	watcher  *fsnotify.Watcher
	mu       sync.RWMutex
	dirs     map[string]bool
	onEvent  func(TamperEvent)
	stopCh   chan struct{}
	started  bool
}

// NewFileWatcher 创建文件监控器
func NewFileWatcher() (*FileWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &FileWatcher{
		watcher: w,
		dirs:    make(map[string]bool),
		stopCh:  make(chan struct{}),
	}, nil
}

// SetOnEvent 注册事件回调
func (f *FileWatcher) SetOnEvent(cb func(TamperEvent)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.onEvent = cb
}

// WatchDir 递归监听目录
func (f *FileWatcher) WatchDir(dir string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略单点错误
		}
		if info.IsDir() {
			if !f.dirs[path] {
				if err := f.watcher.Add(path); err == nil {
					f.dirs[path] = true
				}
			}
		}
		return nil
	})
}

// Start 启动监听协程
func (f *FileWatcher) Start() {
	f.mu.Lock()
	if f.started {
		f.mu.Unlock()
		return
	}
	f.started = true
	f.mu.Unlock()

	go func() {
		for {
			select {
			case <-f.stopCh:
				return
			case ev, ok := <-f.watcher.Events:
				if !ok {
					return
				}
				f.handleEvent(ev)
			case err, ok := <-f.watcher.Errors:
				if !ok {
					return
				}
				log.Printf("⚠️ fsnotify 错误: %v", err)
			}
		}
	}()
}

func (f *FileWatcher) handleEvent(ev fsnotify.Event) {
	var eventType string
	switch {
	case ev.Op&fsnotify.Write == fsnotify.Write:
		eventType = "WRITE"
	case ev.Op&fsnotify.Remove == fsnotify.Remove:
		eventType = "REMOVE"
	case ev.Op&fsnotify.Rename == fsnotify.Rename:
		eventType = "RENAME"
	case ev.Op&fsnotify.Chmod == fsnotify.Chmod:
		// CHMOD 本身可能是我们主动 chmod 444 的合法操作，不作为篡改告警
		return
	case ev.Op&fsnotify.Create == fsnotify.Create:
		// 新建文件不是篡改，但如果是目录则递归 watch
		if fi, err := os.Stat(ev.Name); err == nil && fi.IsDir() {
			_ = f.watcher.Add(ev.Name)
			f.mu.Lock()
			f.dirs[ev.Name] = true
			f.mu.Unlock()
		}
		return
	default:
		return
	}

	te := TamperEvent{
		Path:      ev.Name,
		EventType: eventType,
		Timestamp: time.Now(),
	}

	log.Printf("🚨 证据变更告警: %s %s", te.EventType, te.Path)

	f.mu.RLock()
	cb := f.onEvent
	f.mu.RUnlock()
	if cb != nil {
		cb(te)
	}
}

// Stop 停止
func (f *FileWatcher) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.started {
		return
	}
	close(f.stopCh)
	_ = f.watcher.Close()
	f.started = false
}

// SetReadOnly 设置为只读
func SetReadOnly(path string) error {
	return os.Chmod(path, 0444)
}

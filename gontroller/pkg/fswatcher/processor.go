package fswatcher

import (
	t "gontroller/pkg/_t"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

type Monitor struct {
	watcher  *fsnotify.Watcher
	path     string
	fileChan chan<- t.ItemPath
	wg       *sync.WaitGroup
}

func NewMonitor(path string, fileChan chan<- t.ItemPath, wg *sync.WaitGroup) *Monitor {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	m := &Monitor{
		watcher:  watcher,
		path:     path,
		fileChan: fileChan,
		wg:       wg,
	}

	return m
}

func (m *Monitor) Walk() {
	defer m.wg.Done()
	filepath.Walk(m.path,
		func(p string, info os.FileInfo, err error) error {
			log.Printf("FSM.Walk -> file: %v, err: %v", p, err)
			if err != nil {
				return err
			}
			if !info.IsDir() {
				m.processFile(p)
			}
			return nil
		})
}

func (m *Monitor) Watch() {
	defer m.wg.Done()
	for {
		select {
		case event, ok := <-m.watcher.Events:
			log.Printf("FSM.Watch.Events -> event: %v, ok: %v", event, ok)
			if !ok {
				return
			}
			if event.Op&fsnotify.Create == fsnotify.Create {
				m.processFile(event.Name)
			}
		case err, ok := <-m.watcher.Errors:
			log.Printf("FSM.Watch.Errors -> event: %v, ok: %v", err, ok)
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}
}

func (m *Monitor) Close() {
	m.watcher.Close()
}

func (m *Monitor) processFile(path string) {
	log.Printf("FSM.processFile -> path: %v", path)
	// todo: check if not known
	m.fileChan <- t.ItemPath(path)
}

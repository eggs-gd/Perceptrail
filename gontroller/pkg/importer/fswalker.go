package importer

import (
	"errors"
	"gontroller/ext/utils"
	t "gontroller/pkg/_t"
	"gontroller/pkg/model"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

var dbproxy model.FilesApi

type fsMonitor struct {
	path         string
	fileChan     chan<- []t.ItemEntry
	wg           *sync.WaitGroup
	currentGroup []t.ItemEntry // current group of files
	currentRun   time.Time     // timestamp for current walker run
}

func NewFsMonitor(path string, fileChan chan<- []t.ItemEntry, wg *sync.WaitGroup) *fsMonitor {
	if dbproxy == nil {
		dbproxy = model.NewProxy()
	}

	m := &fsMonitor{
		path:         path,
		fileChan:     fileChan,
		wg:           wg,
		currentGroup: []t.ItemEntry{},
	}

	return m
}

func (m *fsMonitor) Walk() {
	defer m.wg.Done()
	m.currentRun = time.Now()
	filepath.WalkDir(m.path,
		func(path string, info os.DirEntry, err error) error {
			log.Printf("FSM.Walk -> file: %v, err: %v", path, err)
			if err != nil {
				return err
			}
			if !info.IsDir() {
				res := m.processFile(path, info)
				if res != nil {
					m.fileChan <- res
				}
			}
			return nil
		})
}

func (m *fsMonitor) Close() {
	m.currentGroup = nil
	m.wg.Done()
}

func (m *fsMonitor) processFile(path string, info os.DirEntry) []t.ItemEntry {
	log.Printf("FSM.processFile -> path: %v", path)

	item := t.NewItemEntryFromDirEntry(path, info)
	updateMimeType(&item)

	if m.tryPutInGroup(item) {
		return nil
	} else { // start new group
		group := m.currentGroup
		m.currentGroup = []t.ItemEntry{item}
		return m.getAndSaveResult(group)
	}
}

func (m *fsMonitor) tryPutInGroup(entry t.ItemEntry) bool {
	if len(m.currentGroup) == 0 {
		m.currentGroup = append(m.currentGroup, entry)
		return true
	}

	first := m.currentGroup[0]
	if filepath.Dir(first.Path) != filepath.Dir(entry.Path) {
		return false
	}

	firstParts := strings.Split(first.Name, ".")
	secondParts := strings.Split(entry.Name, ".")

	var base string = ""
	var compare string = ""

	if len(firstParts) < len(secondParts) {
		base = strings.Join(firstParts, ".")
		compare = strings.Join(secondParts[:len(firstParts)], ".")
	} else if len(firstParts) > len(secondParts) {
		base = strings.Join(secondParts, ".")
		compare = strings.Join(firstParts[:len(secondParts)], ".")
	} else {
		base = strings.Join(firstParts[:len(firstParts)-1], ".")
		compare = strings.Join(secondParts[:len(secondParts)-1], ".")
	}

	if base == compare {
		m.currentGroup = append(m.currentGroup, entry)
		sort.Slice(m.currentGroup, func(i, j int) bool {
			return ((strings.Contains(m.currentGroup[i].MimeType, "video/") && !strings.Contains(m.currentGroup[j].MimeType, "video/")) ||
				(strings.Contains(m.currentGroup[i].MimeType, "image/") && !strings.Contains(m.currentGroup[j].MimeType, "image/")) ||
				(m.currentGroup[i].MimeType == m.currentGroup[j].MimeType && m.currentGroup[i].Size >= m.currentGroup[j].Size))
		})
		return true
	}

	return false
}

func (m *fsMonitor) getAndSaveResult(group []t.ItemEntry) []t.ItemEntry {
	var dbitems []model.FileDto = make([]model.FileDto, len(group))

	var changedFiles []t.ItemEntry

	for i, item := range group {
		dbitem, err := dbproxy.GetFileByPath(item.Path)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				utils.AppendUniq(changedFiles, item)

				dbitem, err = dbproxy.CreateFile(item)
				if err != nil {
					// cant create file
					log.Printf("Error creating file: %v", err)
					continue
				}
			} else {
				// Something went wrong with db acess, probably should be panic
				log.Printf("Error retrieving file: %v", err)
				continue
			}
		}

		if dbitem.ID != 0 {
			dbitem.CheckTime = m.currentRun
			dbitems[i] = dbitem
		} else {
			// something went wrong but should be catched above on creating phase
			log.Printf("Error got empty file: %v", err)
			continue
		}

		if dbitem.LinkTo(dbitems[0]) {
			utils.AppendUniq(changedFiles, item)
		}

		if item.ModTime == dbitem.ModTime &&
			item.Size == dbitem.Size {
			continue // do nothing, skip
		} else {
			utils.AppendUniq(changedFiles, item)
		}
	}

	// ignore whole group if main file is not media
	if !strings.Contains(dbitems[0].MimeType, "video/") &&
		!strings.Contains(dbitems[0].MimeType, "image/") {
		for _, itm := range dbitems {
			itm.SetIgnored()
		}

		changedFiles = nil
	}

	dbproxy.UpdateFiles(dbitems)

	if changedFiles != nil && len(changedFiles) > 0 {
		if !slices.Contains(changedFiles, group[0]) {
			changedFiles = append([]t.ItemEntry{group[0]}, changedFiles...)
		}

		return changedFiles
	}

	return nil
}

func updateMimeType(entry *t.ItemEntry) {
	updateMimeTypeGeneric(entry)
	updateMimeTypeFromMeta(entry)
}

func updateMimeTypeGeneric(entry *t.ItemEntry) {
	if entry.MimeType != "" {
		return
	}

	ext := filepath.Ext(entry.Path)
	entry.MimeType = mime.TypeByExtension(ext)
}

func updateMimeTypeFromMeta(entry *t.ItemEntry) {
	if entry.MimeType != "" {
		return
	}

	file, err := os.Open(entry.Path)
	if err != nil {
		log.Println("Error:", err)
	}
	defer file.Close()

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		log.Println("Error:", err)
	}

	entry.MimeType = http.DetectContentType(buffer)
}

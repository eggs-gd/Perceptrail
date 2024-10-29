package importer

import (
	"errors"
	"gontroller/ext/utils"
	t "gontroller/pkg/_t"
	"gontroller/pkg/model"
	"gontroller/pkg/model/dto"
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

var filesProxy model.FilesApi

type fsMonitor struct {
	path         string
	fileChan     chan<- []dto.FileDto
	wg           *sync.WaitGroup
	currentGroup []t.ItemEntry // current group of files
	currentRun   time.Time     // timestamp for current walker run
}

func NewFsWalker(path string, fileChan chan<- []dto.FileDto, wg *sync.WaitGroup) *fsMonitor {
	if filesProxy == nil {
		filesProxy = model.NewProxy()
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
				log.Printf("FSM.processingResult -> files: %v, path: %v", res, filepath.Dir(path))
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

func (m *fsMonitor) processFile(path string, info os.DirEntry) []dto.FileDto {
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
	log.Printf("FSM.tryPutInGroup -> entry: %v", entry)
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

func (m *fsMonitor) getAndSaveResult(group []t.ItemEntry) []dto.FileDto {
	log.Printf("FSM.getAndSaveResult -> group: %v", group)
	var dbitems []dto.FileDto
	var changedFiles []*dto.FileDto

	for i, item := range group {
		dbitem, err := filesProxy.GetFileByPath(item.Path)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				dbitem, err = filesProxy.CreateFile(item)
				log.Printf("FSM.getAndSaveResult -> changedFiles add new: %v", dbitems)
				changedFiles = utils.AppendUniq(changedFiles, &dbitem)

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
			dbitems = utils.AppendUniq(dbitems, dbitem)
		} else {
			// something went wrong but should be catched above on creating phase
			log.Printf("Error got empty file: %v", err)
			continue
		}

		if dbitems[i].LinkTo(dbitems[0]) && !dbitem.IsIgnored() {
			log.Printf("FSM.getAndSaveResult -> changedFiles add linked: %v", dbitems)
			changedFiles = utils.AppendUniq(changedFiles, &dbitems[i])
		}

		if item.ModTime.UTC() == dbitems[i].ModTime.UTC() &&
			item.Size == dbitems[i].Size {
			continue // do nothing, skip
		} else if !dbitems[i].IsIgnored() {
			log.Printf("FSM.getAndSaveResult -> changedFiles ad changed: item: %v, dbitem: %v", item.ModTime, dbitems[i].ModTime)
			changedFiles = utils.AppendUniq(changedFiles, &dbitem)
		}
	}

	// ignore whole group if main file is not media
	if !strings.Contains(dbitems[0].MimeType, "video/") &&
		!strings.Contains(dbitems[0].MimeType, "image/") {
		for i := range dbitems {
			dbitems[i].SetIgnored()
		}

		changedFiles = nil
	}

	if _, err := filesProxy.UpdateFiles(dbitems); err != nil {
		panic("can't update files")
	}

	if len(changedFiles) > 0 {
		if !slices.Contains(changedFiles, &dbitems[0]) {
			changedFiles = append([]*dto.FileDto{&dbitems[0]}, changedFiles...)
		}

		result := make([]dto.FileDto, len(changedFiles))

		// Копіюємо кожен елемент зі слайса посилань у слайс значень
		for i, ptr := range changedFiles {
			if ptr != nil { // перевірка на nil для безпеки
				result[i] = *ptr
			}
		}

		// log.Printf("FSM.getAndSaveResult -> dbitems: %v", dbitems)
		// log.Printf("FSM.getAndSaveResult -> changedFiles: %v", changedFiles)
		return result
	}

	// log.Printf("FSM.getAndSaveResult -> dbitems: %v", dbitems)
	// log.Printf("FSM.getAndSaveResult -> changedFiles: %v", changedFiles)
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

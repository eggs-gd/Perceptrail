package scan

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"perceptrail/chain"
	"perceptrail/gontroller/ext/utils"
	t "perceptrail/gontroller/pkg/_t"
	"perceptrail/gontroller/pkg/model"
	"perceptrail/gontroller/pkg/model/dto"
	"slices"
	"sort"
	"strings"
	"time"

	l "perceptrail/logger"

	"gorm.io/gorm"
)

var filesProxy model.FilesApi

type inType struct {
	path string
	info os.DirEntry
}

type fsMonitor struct {
	logger *l.Logger
	ctx    context.Context
	cancel context.CancelFunc

	path         string
	currentGroup []t.ItemEntry // current group of files
	currentRun   time.Time     // timestamp for current walker run
}

func NewFsWalker(path string, chout chan<- []*dto.FileDto, logger *l.Logger) chain.Processor {
	if filesProxy == nil {
		filesProxy = model.NewProxy(logger.Named("DB"))
	}

	m := &fsMonitor{
		logger:       logger,
		path:         path,
		currentGroup: []t.ItemEntry{},
	}

	return chain.NewEntryPoint(chout, m)
}

func (m *fsMonitor) Start(chin chan<- inType, ctx context.Context) {
	m.ctx, m.cancel = context.WithCancel(ctx)
	defer m.Stop()

	m.currentRun = time.Now()
	err := filepath.WalkDir(m.path, func(path string, info os.DirEntry, err error) error {
		m.logger.Info("walkDirFunc", l.String("file", path), l.Error(err))
		select {
		case <-m.ctx.Done():
			return m.ctx.Err()

		default:
			if err != nil {
				return err
			}
			if !info.IsDir() {
				chin <- inType{path, info}
			}
			return nil
		}
	})

	if err != nil && !errors.Is(err, context.Canceled) {
		m.logger.Error("FsMonitor Error", l.Error(err))
	}
}

func (m *fsMonitor) finalizeWalk() {
	// todo process last file
	// todo process removed files
}

func (m *fsMonitor) Stop() {
	m.finalizeWalk()
	m.cancel()
}

func (m *fsMonitor) Decorate(in inType) ([]*dto.FileDto, error) {
	m.logger.Info("processFile", l.String("path", in.path))

	item := newItemEntryFromDirEntry(in.path, in.info)
	updateMimeType(&item, m.logger)

	if m.tryPutInGroup(item) {
		return nil, chain.ErrSkippedItem
	} else { // start new group
		group := m.currentGroup
		m.currentGroup = []t.ItemEntry{item}
		res := m.entryToFile(group)
		if res == nil {
			return nil, chain.ErrSkippedItem

		}
		return res, nil
	}
}

func (m *fsMonitor) tryPutInGroup(entry t.ItemEntry) bool {
	m.logger.Info("tryPutInGroup", l.Any("entry", entry))
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

func (m *fsMonitor) entryToFile(group []t.ItemEntry) []*dto.FileDto {
	m.logger.Info("entryToFile", l.Any("group", group))
	var dbitems []*dto.FileDto
	var changedFiles []*dto.FileDto

	for i, item := range group {
		dbitem, err := filesProxy.GetFileByPath(item.Path)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				dbitem, err = filesProxy.CreateFile(item)
				m.logger.Info("entryToFile", l.Any("changedFiles add new", dbitem))
				changedFiles = utils.AppendUniq(changedFiles, dbitem)

				if err != nil {
					// cant create file
					m.logger.Error("Error creating file", l.Error(err))
					continue
				}
			} else {
				// Something went wrong with db access, probably should be panic
				m.logger.Error("Error retrieving file", l.Error(err))
				continue
			}
		}

		if dbitem.ID != 0 {
			dbitem.CheckTime = m.currentRun
			dbitems = utils.AppendUniq(dbitems, dbitem)
		} else {
			// something went wrong but should be catched above on creating phase
			m.logger.Error("Error got empty file", l.Error(err))
			continue
		}

		if dbitems[i].LinkTo(dbitems[0]) && !dbitem.IsIgnored() {
			m.logger.Info("entryToFile", l.Any("changedFiles add linked", dbitems))
			changedFiles = utils.AppendUniq(changedFiles, dbitems[i])
		}

		if item.ModTime.UTC() == dbitems[i].ModTime.UTC() &&
			item.Size == dbitems[i].Size {
			continue // do nothing, skip
		} else if !dbitems[i].IsIgnored() {
			m.logger.Info("entryToFile -> changedFiles ad changed", l.Any("item", item.ModTime), l.Any("dbItem", dbitems[i].ModTime))
			changedFiles = utils.AppendUniq(changedFiles, dbitem)
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
		m.logger.Panic("can't update files")
	}

	if len(changedFiles) > 0 {
		if !slices.Contains(changedFiles, dbitems[0]) {
			changedFiles = append([]*dto.FileDto{dbitems[0]}, changedFiles...)
		}

		return changedFiles
	}

	return nil
}

func updateMimeType(entry *t.ItemEntry, logger *l.Logger) {
	updateMimeTypeGeneric(entry)
	updateMimeTypeFromMeta(entry, logger)
}

func updateMimeTypeGeneric(entry *t.ItemEntry) {
	if entry.MimeType != "" {
		return
	}

	ext := filepath.Ext(entry.Path)
	entry.MimeType = mime.TypeByExtension(ext)
}

func updateMimeTypeFromMeta(entry *t.ItemEntry, logger *l.Logger) {
	if entry.MimeType != "" {
		return
	}

	file, err := os.Open(entry.Path)
	if err != nil {
		logger.Error("updateMimeTypeFromMeta", l.Error(err))
	}
	defer file.Close()

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		logger.Error("updateMimeTypeFromMeta", l.Error(err))
	}

	entry.MimeType = http.DetectContentType(buffer)
}

func newItemEntryFromDirEntry(path string, dirEntry os.DirEntry) t.ItemEntry {
	i := t.ItemEntry{
		Path: path,
		Name: dirEntry.Name(),
	}

	info, err := dirEntry.Info()
	if err == nil {
		i.Size = info.Size()
		i.ModTime = info.ModTime()
	}
	return i
}

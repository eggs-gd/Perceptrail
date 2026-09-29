package scan

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"perceptrail/gontroller/pkg/model/dto"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/eggs-gd/perceplib/api"
	"github.com/eggs-gd/perceplib/chain"

	l "github.com/eggs-gd/perceplib/logger"

	"gorm.io/gorm"
)

type inType struct {
	path string
	info os.DirEntry
}

type fsMonitor struct {
	logger *l.Logger
	ctx    context.Context
	cancel context.CancelFunc

	path         string
	currentGroup []dto.ItemEntry // current group of files
	currentRun   time.Time       // timestamp for current walker run
}

func NewFsWalker(path string, chout chan<- []*dto.FileDto, logger *l.Logger) chain.Processor {

	m := &fsMonitor{
		logger:       logger,
		path:         path,
		currentGroup: []dto.ItemEntry{},
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
		return
	}

	// Groups are emitted when the next group starts; the marker flushes the last one
	select {
	case chin <- inType{}:
	case <-m.ctx.Done():
	}
}

func (m *fsMonitor) finalizeWalk() {
	// todo process removed files
}

func (m *fsMonitor) Stop() {
	m.finalizeWalk()
	m.cancel()
}

func (m *fsMonitor) Decorate(in inType) ([]*dto.FileDto, error) {
	if in.info == nil { // end of walk: emit the last group
		group := m.currentGroup
		m.currentGroup = []dto.ItemEntry{}
		if res := m.entryToFile(group); res != nil {
			return res, nil
		}
		return nil, chain.ErrSkippedItem
	}

	m.logger.Info("processFile", l.String("path", in.path))

	if shouldSkipPath(in.path) {
		return nil, chain.ErrSkippedItem
	}

	item := newItemEntryFromDirEntry(in.path, in.info)
	updateMimeType(&item, m.logger)

	if m.tryPutInGroup(item) {
		return nil, chain.ErrSkippedItem
	} else { // start new group
		group := m.currentGroup
		m.currentGroup = []dto.ItemEntry{item}
		res := m.entryToFile(group)
		if res == nil {
			return nil, chain.ErrSkippedItem

		}
		return res, nil
	}
}

func (m *fsMonitor) tryPutInGroup(entry dto.ItemEntry) bool {
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

func (m *fsMonitor) entryToFile(group []dto.ItemEntry) []*dto.FileDto {
	m.logger.Info("entryToFile", l.Any("group", group))
	var dbitems []*dto.FileDto
	var changedFiles []*dto.FileDto

	if len(group) == 0 {
		return nil
	}

	for _, item := range group {
		dbitem, err := filesProxy.GetFileByPath(item.Path)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				dbitem, err = filesProxy.CreateFile(item)
				m.logger.Info("entryToFile", l.Any("changedFiles add new", dbitem))
				changedFiles = api.AppendUniq(changedFiles, dbitem)

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
			dbitems = api.AppendUniq(dbitems, dbitem)
		} else {
			// something went wrong but should be catched above on creating phase
			m.logger.Error("Error got empty file", l.Error(err))
			continue
		}

		if dbitem.LinkTo(dbitems[0]) && !dbitem.IsIgnored() {
			m.logger.Info("entryToFile", l.Any("changedFiles add linked", dbitems))
			changedFiles = api.AppendUniq(changedFiles, dbitem)
		}

		if item.ModTime.UTC() == dbitem.ModTime.UTC() &&
			item.Size == dbitem.Size {
			continue // do nothing, skip
		}

		// Changed on disk: store the fresh size/time, or every scan sees it as changed again
		// (and HashShort is computed from the stale size)
		m.logger.Info("entryToFile -> changedFiles ad changed", l.Any("item", item.ModTime), l.Any("dbItem", dbitem.ModTime))
		dbitem.Size = item.Size
		dbitem.ModTime = item.ModTime
		dbitem.MimeType = item.MimeType
		if !dbitem.IsIgnored() {
			changedFiles = api.AppendUniq(changedFiles, dbitem)
		}
	}

	if len(dbitems) == 0 {
		return nil
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

func updateMimeType(entry *dto.ItemEntry, logger *l.Logger) {
	updateMimeTypeGeneric(entry)
	updateMimeTypeFromMeta(entry, logger)
}

func updateMimeTypeGeneric(entry *dto.ItemEntry) {
	if entry.MimeType != "" {
		return
	}

	ext := filepath.Ext(entry.Path)
	entry.MimeType = mime.TypeByExtension(ext)
}

func updateMimeTypeFromMeta(entry *dto.ItemEntry, logger *l.Logger) {
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

func newItemEntryFromDirEntry(path string, dirEntry os.DirEntry) dto.ItemEntry {
	i := dto.ItemEntry{
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

// Apple Photos internals: .THM posters and generated derivatives are not gallery sources.
func shouldSkipPath(path string) bool {
	lower := strings.ToLower(path)
	if strings.HasSuffix(lower, ".thm") {
		return true
	}
	if strings.Contains(lower, ".photoslibrary/resources/") {
		return true
	}
	return false
}

package model

import (
	"perceptrail/gontroller/internal/model/dto"

	pubsub "github.com/eggs-gd/go-pub-sub"

	"gorm.io/gorm"
)

// The model's writes. Each write is the unexported method of its name on tx — read,
// decide, write, in the writer's transaction; a write calls other writes directly
// (tx has no public ones). Each write is submitted as a command (an Op): the
// public method of its name submits and waits for its own result (Do); its Command
// lets a caller submit and go on — fire and forget, or its own results through a
// Client — and its Done is every result, as an event.

// Arguments and results of the writes with more than one: a command carries one value
type (
	ValidateGroupArgs struct {
		Files []*dto.FileDto
		Hash  string
	}
	ValidateAssetArgs struct {
		Key   string
		Files []*dto.FileDto
		Hash  string
	}
	MetaArgs struct {
		Key, Value string
	}
	GoneResult struct {
		Deleted, Dirty int
	}
)

// commands: every write as a command, run by the writer. A write takes one
// argument and gives one result (pubsub.None where it has none): its method on tx
// is the command's function as it is.
type commands struct {
	clearHashes   *pubsub.Op[*gorm.DB, pubsub.None, int64]
	createItem    *pubsub.Op[*gorm.DB, *dto.FileDto, *dto.ItemDto]
	updateItem    *pubsub.Op[*gorm.DB, *dto.ItemDto, *dto.ItemDto]
	deleteItem    *pubsub.Op[*gorm.DB, *dto.ItemDto, pubsub.None]
	gone          *pubsub.Op[*gorm.DB, []*dto.FileDto, GoneResult]
	ignore        *pubsub.Op[*gorm.DB, []*dto.FileDto, pubsub.None]
	validateGroup *pubsub.Op[*gorm.DB, ValidateGroupArgs, *dto.ItemDto]
	validateAsset *pubsub.Op[*gorm.DB, ValidateAssetArgs, *dto.ItemDto]
	publish       *pubsub.Op[*gorm.DB, *dto.ItemDto, *dto.ItemDto]
	markRework    *pubsub.Op[*gorm.DB, []string, int64]
	createFile    *pubsub.Op[*gorm.DB, dto.ItemEntry, *dto.FileDto]
	updateFile    *pubsub.Op[*gorm.DB, *dto.FileDto, *dto.FileDto]
	updateFiles   *pubsub.Op[*gorm.DB, []*dto.FileDto, []*dto.FileDto]
	createFiles   *pubsub.Op[*gorm.DB, []dto.ItemEntry, []*dto.FileDto]
	saveStats     *pubsub.Op[*gorm.DB, []*dto.FileDto, pubsub.None]
	deleteFiles   *pubsub.Op[*gorm.DB, []*dto.FileDto, pubsub.None]
	unignoreFiles *pubsub.Op[*gorm.DB, pubsub.None, int64]
	setMeta       *pubsub.Op[*gorm.DB, MetaArgs, pubsub.None]
}

func newCommands(p *Proxy) commands {
	return commands{
		clearHashes:   command(p, pubsub.Frame, (*tx).clearHashes),
		createItem:    command(p, pubsub.Frame, (*tx).createItem),
		updateItem:    command(p, pubsub.Frame, (*tx).updateItem),
		deleteItem:    command(p, pubsub.Frame, (*tx).deleteItem),
		gone:          command(p, pubsub.Frame, (*tx).gone),
		ignore:        command(p, pubsub.Frame, (*tx).ignore),
		validateGroup: command(p, pubsub.Frame, (*tx).validateGroup),
		validateAsset: command(p, pubsub.Frame, (*tx).validateAsset),
		publish:       command(p, pubsub.Frame, (*tx).publish),
		markRework:    command(p, pubsub.Frame, (*tx).markRework),
		createFile:    command(p, pubsub.Frame, (*tx).createFile),
		updateFile:    command(p, pubsub.Frame, (*tx).updateFile),
		updateFiles:   command(p, pubsub.Frame, (*tx).updateFiles),
		createFiles:   command(p, pubsub.Frame, (*tx).createFiles),
		saveStats:     command(p, pubsub.Frame, (*tx).saveStats),
		deleteFiles:   command(p, pubsub.Frame, (*tx).deleteFiles),
		unignoreFiles: command(p, pubsub.Frame, (*tx).unignoreFiles),
		setMeta:       command(p, pubsub.Frame, (*tx).setMeta),
	}
}

// Each write's two faces, in pairs: the public method of its name submits and waits
// for its own result; its Command submits and goes on — in the view of its shape
// (a Message without a result, a Signal without an argument).

func (p *Proxy) ClearHashes() (int64, error) {
	return p.clearHashes.Do(pubsub.None{})
}

func (p *Proxy) ClearHashesCommand() pubsub.Signal[int64] {
	return pubsub.SignalOf(p.clearHashes)
}

func (p *Proxy) CreateItem(file *dto.FileDto) (*dto.ItemDto, error) {
	return p.createItem.Do(file)
}

func (p *Proxy) CreateItemCommand() pubsub.Command[*dto.FileDto, *dto.ItemDto] {
	return p.createItem
}

func (p *Proxy) UpdateItem(item *dto.ItemDto) (*dto.ItemDto, error) {
	return p.updateItem.Do(item)
}

func (p *Proxy) UpdateItemCommand() pubsub.Command[*dto.ItemDto, *dto.ItemDto] {
	return p.updateItem
}

func (p *Proxy) DeleteItem(item *dto.ItemDto) error {
	_, err := p.deleteItem.Do(item)
	return err
}

func (p *Proxy) DeleteItemCommand() pubsub.Message[*dto.ItemDto] {
	return pubsub.MessageOf(p.deleteItem)
}

func (p *Proxy) Gone(files []*dto.FileDto) (deleted, dirty int, err error) {
	r, err := p.gone.Do(files)
	return r.Deleted, r.Dirty, err
}

func (p *Proxy) GoneCommand() pubsub.Command[[]*dto.FileDto, GoneResult] {
	return p.gone
}

func (p *Proxy) Ignore(files []*dto.FileDto) error {
	_, err := p.ignore.Do(files)
	return err
}

func (p *Proxy) IgnoreCommand() pubsub.Message[[]*dto.FileDto] {
	return pubsub.MessageOf(p.ignore)
}

func (p *Proxy) ValidateGroup(files []*dto.FileDto, hash string) (*dto.ItemDto, error) {
	return p.validateGroup.Do(ValidateGroupArgs{files, hash})
}

func (p *Proxy) ValidateGroupCommand() pubsub.Command[ValidateGroupArgs, *dto.ItemDto] {
	return p.validateGroup
}

func (p *Proxy) ValidateAsset(key string, files []*dto.FileDto, hash string) (*dto.ItemDto, error) {
	return p.validateAsset.Do(ValidateAssetArgs{key, files, hash})
}

func (p *Proxy) ValidateAssetCommand() pubsub.Command[ValidateAssetArgs, *dto.ItemDto] {
	return p.validateAsset
}

func (p *Proxy) Publish(item *dto.ItemDto) (*dto.ItemDto, error) {
	return p.publish.Do(item)
}

func (p *Proxy) PublishCommand() pubsub.Command[*dto.ItemDto, *dto.ItemDto] {
	return p.publish
}

func (p *Proxy) MarkRework(guids []string) (int64, error) {
	return p.markRework.Do(guids)
}

func (p *Proxy) MarkReworkCommand() pubsub.Command[[]string, int64] {
	return p.markRework
}

func (p *Proxy) CreateFile(entry dto.ItemEntry) (*dto.FileDto, error) {
	return p.createFile.Do(entry)
}

func (p *Proxy) CreateFileCommand() pubsub.Command[dto.ItemEntry, *dto.FileDto] {
	return p.createFile
}

func (p *Proxy) UpdateFile(file *dto.FileDto) (*dto.FileDto, error) {
	return p.updateFile.Do(file)
}

func (p *Proxy) UpdateFileCommand() pubsub.Command[*dto.FileDto, *dto.FileDto] {
	return p.updateFile
}

func (p *Proxy) UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error) {
	return p.updateFiles.Do(files)
}

func (p *Proxy) UpdateFilesCommand() pubsub.Command[[]*dto.FileDto, []*dto.FileDto] {
	return p.updateFiles
}

func (p *Proxy) CreateFiles(entries []dto.ItemEntry) ([]*dto.FileDto, error) {
	return p.createFiles.Do(entries)
}

func (p *Proxy) CreateFilesCommand() pubsub.Command[[]dto.ItemEntry, []*dto.FileDto] {
	return p.createFiles
}

func (p *Proxy) SaveStats(files []*dto.FileDto) error {
	_, err := p.saveStats.Do(files)
	return err
}

func (p *Proxy) SaveStatsCommand() pubsub.Message[[]*dto.FileDto] {
	return pubsub.MessageOf(p.saveStats)
}

func (p *Proxy) DeleteFiles(files []*dto.FileDto) error {
	_, err := p.deleteFiles.Do(files)
	return err
}

func (p *Proxy) DeleteFilesCommand() pubsub.Message[[]*dto.FileDto] {
	return pubsub.MessageOf(p.deleteFiles)
}

func (p *Proxy) UnignoreFiles() (int64, error) {
	return p.unignoreFiles.Do(pubsub.None{})
}

func (p *Proxy) UnignoreFilesCommand() pubsub.Signal[int64] {
	return pubsub.SignalOf(p.unignoreFiles)
}

func (p *Proxy) SetMeta(key, value string) error {
	_, err := p.setMeta.Do(MetaArgs{key, value})
	return err
}

func (p *Proxy) SetMetaCommand() pubsub.Message[MetaArgs] {
	return pubsub.MessageOf(p.setMeta)
}

// command: a write as a command — it runs in the writer's transaction, under its own
// savepoint (run)
func command[A, R any](p *Proxy, class pubsub.Class, fn func(t *tx, arg A) (R, error)) *pubsub.Op[*gorm.DB, A, R] {
	return pubsub.New(p.writer, class, func(db *gorm.DB, arg A) (R, error) {
		var r R
		err := p.run(db, func(t *tx) (err error) {
			r, err = fn(t, arg)
			return err
		})
		return r, err
	})
}

package model

import (
	"perceptrail/gontroller/internal/model/dto"

	pubsub "github.com/eggs-gd/go-pub-sub"

	"gorm.io/gorm"
)

// The model's writes. Each rule is the unexported method of its name on tx — read,
// decide, write, in the writer's transaction; a rule calls other rules directly
// (tx has no public writes). Each rule is a topic (an Op): the
// public method of its name submits and waits for its own result (Do); its Topic
// lets a caller submit and go on, picking its results from a subscription.

// Arguments and results of the rules with more than one: a topic carries one value
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

// rules: every write rule as a topic, run by the writer. A rule takes one argument
// and gives one result (pubsub.None where it has none): its method on tx is the
// topic's function as it is.
type rules struct {
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

func newRules(p *Proxy) rules {
	return rules{
		clearHashes:   rule(p, pubsub.Frame, (*tx).clearHashes),
		createItem:    rule(p, pubsub.Frame, (*tx).createItem),
		updateItem:    rule(p, pubsub.Frame, (*tx).updateItem),
		deleteItem:    rule(p, pubsub.Frame, (*tx).deleteItem),
		gone:          rule(p, pubsub.Frame, (*tx).gone),
		ignore:        rule(p, pubsub.Frame, (*tx).ignore),
		validateGroup: rule(p, pubsub.Frame, (*tx).validateGroup),
		validateAsset: rule(p, pubsub.Frame, (*tx).validateAsset),
		publish:       rule(p, pubsub.Frame, (*tx).publish),
		markRework:    rule(p, pubsub.Frame, (*tx).markRework),
		createFile:    rule(p, pubsub.Frame, (*tx).createFile),
		updateFile:    rule(p, pubsub.Frame, (*tx).updateFile),
		updateFiles:   rule(p, pubsub.Frame, (*tx).updateFiles),
		createFiles:   rule(p, pubsub.Frame, (*tx).createFiles),
		saveStats:     rule(p, pubsub.Frame, (*tx).saveStats),
		deleteFiles:   rule(p, pubsub.Frame, (*tx).deleteFiles),
		unignoreFiles: rule(p, pubsub.Frame, (*tx).unignoreFiles),
		setMeta:       rule(p, pubsub.Frame, (*tx).setMeta),
	}
}

// Each rule's two faces, in pairs: the public method of its name submits and waits
// for its own result; its Topic submits and goes on, results by subscription — in
// the view of its shape (a Message without a result, a Signal without an argument).

func (p *Proxy) ClearHashes() (int64, error) {
	return p.rules.clearHashes.Do(pubsub.None{})
}

func (p *Proxy) ClearHashesTopic() pubsub.Signal[int64] {
	return pubsub.SignalOf(p.rules.clearHashes)
}

func (p *Proxy) CreateItem(file *dto.FileDto) (*dto.ItemDto, error) {
	return p.rules.createItem.Do(file)
}

func (p *Proxy) CreateItemTopic() pubsub.Topic[*dto.FileDto, *dto.ItemDto] {
	return p.rules.createItem
}

func (p *Proxy) UpdateItem(item *dto.ItemDto) (*dto.ItemDto, error) {
	return p.rules.updateItem.Do(item)
}

func (p *Proxy) UpdateItemTopic() pubsub.Topic[*dto.ItemDto, *dto.ItemDto] {
	return p.rules.updateItem
}

func (p *Proxy) DeleteItem(item *dto.ItemDto) error {
	_, err := p.rules.deleteItem.Do(item)
	return err
}

func (p *Proxy) DeleteItemTopic() pubsub.Message[*dto.ItemDto] {
	return pubsub.MessageOf(p.rules.deleteItem)
}

func (p *Proxy) Gone(files []*dto.FileDto) (deleted, dirty int, err error) {
	r, err := p.rules.gone.Do(files)
	return r.Deleted, r.Dirty, err
}

func (p *Proxy) GoneTopic() pubsub.Topic[[]*dto.FileDto, GoneResult] {
	return p.rules.gone
}

func (p *Proxy) Ignore(files []*dto.FileDto) error {
	_, err := p.rules.ignore.Do(files)
	return err
}

func (p *Proxy) IgnoreTopic() pubsub.Message[[]*dto.FileDto] {
	return pubsub.MessageOf(p.rules.ignore)
}

func (p *Proxy) ValidateGroup(files []*dto.FileDto, hash string) (*dto.ItemDto, error) {
	return p.rules.validateGroup.Do(ValidateGroupArgs{files, hash})
}

func (p *Proxy) ValidateGroupTopic() pubsub.Topic[ValidateGroupArgs, *dto.ItemDto] {
	return p.rules.validateGroup
}

func (p *Proxy) ValidateAsset(key string, files []*dto.FileDto, hash string) (*dto.ItemDto, error) {
	return p.rules.validateAsset.Do(ValidateAssetArgs{key, files, hash})
}

func (p *Proxy) ValidateAssetTopic() pubsub.Topic[ValidateAssetArgs, *dto.ItemDto] {
	return p.rules.validateAsset
}

func (p *Proxy) Publish(item *dto.ItemDto) (*dto.ItemDto, error) {
	return p.rules.publish.Do(item)
}

func (p *Proxy) PublishTopic() pubsub.Topic[*dto.ItemDto, *dto.ItemDto] {
	return p.rules.publish
}

func (p *Proxy) MarkRework(guids []string) (int64, error) {
	return p.rules.markRework.Do(guids)
}

func (p *Proxy) MarkReworkTopic() pubsub.Topic[[]string, int64] {
	return p.rules.markRework
}

func (p *Proxy) CreateFile(entry dto.ItemEntry) (*dto.FileDto, error) {
	return p.rules.createFile.Do(entry)
}

func (p *Proxy) CreateFileTopic() pubsub.Topic[dto.ItemEntry, *dto.FileDto] {
	return p.rules.createFile
}

func (p *Proxy) UpdateFile(file *dto.FileDto) (*dto.FileDto, error) {
	return p.rules.updateFile.Do(file)
}

func (p *Proxy) UpdateFileTopic() pubsub.Topic[*dto.FileDto, *dto.FileDto] {
	return p.rules.updateFile
}

func (p *Proxy) UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error) {
	return p.rules.updateFiles.Do(files)
}

func (p *Proxy) UpdateFilesTopic() pubsub.Topic[[]*dto.FileDto, []*dto.FileDto] {
	return p.rules.updateFiles
}

func (p *Proxy) CreateFiles(entries []dto.ItemEntry) ([]*dto.FileDto, error) {
	return p.rules.createFiles.Do(entries)
}

func (p *Proxy) CreateFilesTopic() pubsub.Topic[[]dto.ItemEntry, []*dto.FileDto] {
	return p.rules.createFiles
}

func (p *Proxy) SaveStats(files []*dto.FileDto) error {
	_, err := p.rules.saveStats.Do(files)
	return err
}

func (p *Proxy) SaveStatsTopic() pubsub.Message[[]*dto.FileDto] {
	return pubsub.MessageOf(p.rules.saveStats)
}

func (p *Proxy) DeleteFiles(files []*dto.FileDto) error {
	_, err := p.rules.deleteFiles.Do(files)
	return err
}

func (p *Proxy) DeleteFilesTopic() pubsub.Message[[]*dto.FileDto] {
	return pubsub.MessageOf(p.rules.deleteFiles)
}

func (p *Proxy) UnignoreFiles() (int64, error) {
	return p.rules.unignoreFiles.Do(pubsub.None{})
}

func (p *Proxy) UnignoreFilesTopic() pubsub.Signal[int64] {
	return pubsub.SignalOf(p.rules.unignoreFiles)
}

func (p *Proxy) SetMeta(key, value string) error {
	_, err := p.rules.setMeta.Do(MetaArgs{key, value})
	return err
}

func (p *Proxy) SetMetaTopic() pubsub.Message[MetaArgs] {
	return pubsub.MessageOf(p.rules.setMeta)
}

// rule: a rule as a topic — it runs in the writer's transaction, under its own
// savepoint (run)
func rule[A, R any](p *Proxy, class pubsub.Class, fn func(t *tx, arg A) (R, error)) *pubsub.Op[*gorm.DB, A, R] {
	return pubsub.New(p.writer, class, func(db *gorm.DB, arg A) (R, error) {
		var r R
		err := p.run(db, func(t *tx) (err error) {
			r, err = fn(t, arg)
			return err
		})
		return r, err
	})
}

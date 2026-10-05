package model

import (
	"perceptrail/gontroller/internal/model/dto"

	pubsub "github.com/eggs-gd/go-pub-sub"

	"gorm.io/gorm"
)

// The model's writes. Each rule is the unexported method of its name — read,
// decide, write, on the Proxy bound to the writer's transaction; a rule calls
// other rules directly, never a public write. Each rule is a topic (an Op): the
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

// rules: every write rule as a topic, run by the writer
type rules struct {
	clearHashes   *pubsub.Op[*gorm.DB, struct{}, int64]
	createItem    *pubsub.Op[*gorm.DB, *dto.FileDto, *dto.ItemDto]
	updateItem    *pubsub.Op[*gorm.DB, *dto.ItemDto, *dto.ItemDto]
	deleteItem    *pubsub.Op[*gorm.DB, *dto.ItemDto, struct{}]
	gone          *pubsub.Op[*gorm.DB, []*dto.FileDto, GoneResult]
	ignore        *pubsub.Op[*gorm.DB, []*dto.FileDto, struct{}]
	validateGroup *pubsub.Op[*gorm.DB, ValidateGroupArgs, *dto.ItemDto]
	validateAsset *pubsub.Op[*gorm.DB, ValidateAssetArgs, *dto.ItemDto]
	publish       *pubsub.Op[*gorm.DB, *dto.ItemDto, *dto.ItemDto]
	markRework    *pubsub.Op[*gorm.DB, []string, int64]
	createFile    *pubsub.Op[*gorm.DB, dto.ItemEntry, *dto.FileDto]
	updateFile    *pubsub.Op[*gorm.DB, *dto.FileDto, *dto.FileDto]
	updateFiles   *pubsub.Op[*gorm.DB, []*dto.FileDto, []*dto.FileDto]
	createFiles   *pubsub.Op[*gorm.DB, []dto.ItemEntry, []*dto.FileDto]
	saveStats     *pubsub.Op[*gorm.DB, []*dto.FileDto, struct{}]
	deleteFiles   *pubsub.Op[*gorm.DB, []*dto.FileDto, struct{}]
	unignoreFiles *pubsub.Op[*gorm.DB, struct{}, int64]
	setMeta       *pubsub.Op[*gorm.DB, MetaArgs, struct{}]
}

func newRules(p *Proxy) rules {
	none := func(err error) (struct{}, error) { return struct{}{}, err }
	return rules{
		clearHashes: rule(p, pubsub.Frame, func(q *Proxy, _ struct{}) (int64, error) { return q.clearHashes() }),
		createItem:  rule(p, pubsub.Frame, (*Proxy).createItem),
		updateItem:  rule(p, pubsub.Frame, (*Proxy).updateItem),
		deleteItem: rule(p, pubsub.Frame, func(q *Proxy, item *dto.ItemDto) (struct{}, error) {
			return none(q.deleteItem(item))
		}),
		gone: rule(p, pubsub.Frame, func(q *Proxy, files []*dto.FileDto) (GoneResult, error) {
			deleted, dirty, err := q.gone(files)
			return GoneResult{deleted, dirty}, err
		}),
		ignore: rule(p, pubsub.Frame, func(q *Proxy, files []*dto.FileDto) (struct{}, error) { return none(q.ignore(files)) }),
		validateGroup: rule(p, pubsub.Frame, func(q *Proxy, a ValidateGroupArgs) (*dto.ItemDto, error) {
			return q.validateGroup(a.Files, a.Hash)
		}),
		validateAsset: rule(p, pubsub.Frame, func(q *Proxy, a ValidateAssetArgs) (*dto.ItemDto, error) {
			return q.validateAsset(a.Key, a.Files, a.Hash)
		}),
		publish:     rule(p, pubsub.Frame, (*Proxy).publish),
		markRework:  rule(p, pubsub.Frame, (*Proxy).markRework),
		createFile:  rule(p, pubsub.Frame, (*Proxy).createFile),
		updateFile:  rule(p, pubsub.Frame, (*Proxy).updateFile),
		updateFiles: rule(p, pubsub.Frame, (*Proxy).updateFiles),
		createFiles: rule(p, pubsub.Frame, (*Proxy).createFiles),
		saveStats: rule(p, pubsub.Frame, func(q *Proxy, files []*dto.FileDto) (struct{}, error) {
			return none(q.saveStats(files))
		}),
		deleteFiles: rule(p, pubsub.Frame, func(q *Proxy, files []*dto.FileDto) (struct{}, error) {
			return none(q.deleteFiles(files))
		}),
		unignoreFiles: rule(p, pubsub.Frame, func(q *Proxy, _ struct{}) (int64, error) { return q.unignoreFiles() }),
		setMeta: rule(p, pubsub.Frame, func(q *Proxy, a MetaArgs) (struct{}, error) {
			return none(q.setMeta(a.Key, a.Value))
		}),
	}
}

// Synchronous: submit and wait for its own result

func (p *Proxy) ClearHashes() (int64, error) { return do(p, p.rules.clearHashes, struct{}{}) }

func (p *Proxy) CreateItem(file *dto.FileDto) (*dto.ItemDto, error) {
	return do(p, p.rules.createItem, file)
}

func (p *Proxy) UpdateItem(item *dto.ItemDto) (*dto.ItemDto, error) {
	return do(p, p.rules.updateItem, item)
}

func (p *Proxy) DeleteItem(item *dto.ItemDto) error {
	_, err := do(p, p.rules.deleteItem, item)
	return err
}

func (p *Proxy) Gone(files []*dto.FileDto) (deleted, dirty int, err error) {
	r, err := do(p, p.rules.gone, files)
	return r.Deleted, r.Dirty, err
}

func (p *Proxy) Ignore(files []*dto.FileDto) error {
	_, err := do(p, p.rules.ignore, files)
	return err
}

func (p *Proxy) ValidateGroup(files []*dto.FileDto, hash string) (*dto.ItemDto, error) {
	return do(p, p.rules.validateGroup, ValidateGroupArgs{files, hash})
}

func (p *Proxy) ValidateAsset(key string, files []*dto.FileDto, hash string) (*dto.ItemDto, error) {
	return do(p, p.rules.validateAsset, ValidateAssetArgs{key, files, hash})
}

func (p *Proxy) Publish(item *dto.ItemDto) (*dto.ItemDto, error) {
	return do(p, p.rules.publish, item)
}

func (p *Proxy) MarkRework(guids []string) (int64, error) { return do(p, p.rules.markRework, guids) }

func (p *Proxy) CreateFile(entry dto.ItemEntry) (*dto.FileDto, error) {
	return do(p, p.rules.createFile, entry)
}

func (p *Proxy) UpdateFile(file *dto.FileDto) (*dto.FileDto, error) {
	return do(p, p.rules.updateFile, file)
}

func (p *Proxy) UpdateFiles(files []*dto.FileDto) ([]*dto.FileDto, error) {
	return do(p, p.rules.updateFiles, files)
}

func (p *Proxy) CreateFiles(entries []dto.ItemEntry) ([]*dto.FileDto, error) {
	return do(p, p.rules.createFiles, entries)
}

func (p *Proxy) SaveStats(files []*dto.FileDto) error {
	_, err := do(p, p.rules.saveStats, files)
	return err
}

func (p *Proxy) DeleteFiles(files []*dto.FileDto) error {
	_, err := do(p, p.rules.deleteFiles, files)
	return err
}

func (p *Proxy) UnignoreFiles() (int64, error) { return do(p, p.rules.unignoreFiles, struct{}{}) }

func (p *Proxy) SetMeta(key, value string) error {
	_, err := do(p, p.rules.setMeta, MetaArgs{key, value})
	return err
}

// Asynchronous: the rule's topic — submit and go on, results by subscription

func (p *Proxy) ClearHashesTopic() pubsub.Topic[struct{}, int64] { return p.rules.clearHashes }
func (p *Proxy) CreateItemTopic() pubsub.Topic[*dto.FileDto, *dto.ItemDto] {
	return p.rules.createItem
}
func (p *Proxy) UpdateItemTopic() pubsub.Topic[*dto.ItemDto, *dto.ItemDto] {
	return p.rules.updateItem
}
func (p *Proxy) DeleteItemTopic() pubsub.Topic[*dto.ItemDto, struct{}] { return p.rules.deleteItem }
func (p *Proxy) GoneTopic() pubsub.Topic[[]*dto.FileDto, GoneResult]   { return p.rules.gone }
func (p *Proxy) IgnoreTopic() pubsub.Topic[[]*dto.FileDto, struct{}]   { return p.rules.ignore }
func (p *Proxy) ValidateGroupTopic() pubsub.Topic[ValidateGroupArgs, *dto.ItemDto] {
	return p.rules.validateGroup
}
func (p *Proxy) ValidateAssetTopic() pubsub.Topic[ValidateAssetArgs, *dto.ItemDto] {
	return p.rules.validateAsset
}
func (p *Proxy) PublishTopic() pubsub.Topic[*dto.ItemDto, *dto.ItemDto] { return p.rules.publish }
func (p *Proxy) MarkReworkTopic() pubsub.Topic[[]string, int64]         { return p.rules.markRework }
func (p *Proxy) CreateFileTopic() pubsub.Topic[dto.ItemEntry, *dto.FileDto] {
	return p.rules.createFile
}
func (p *Proxy) UpdateFileTopic() pubsub.Topic[*dto.FileDto, *dto.FileDto] {
	return p.rules.updateFile
}
func (p *Proxy) UpdateFilesTopic() pubsub.Topic[[]*dto.FileDto, []*dto.FileDto] {
	return p.rules.updateFiles
}
func (p *Proxy) CreateFilesTopic() pubsub.Topic[[]dto.ItemEntry, []*dto.FileDto] {
	return p.rules.createFiles
}
func (p *Proxy) SaveStatsTopic() pubsub.Topic[[]*dto.FileDto, struct{}] { return p.rules.saveStats }
func (p *Proxy) DeleteFilesTopic() pubsub.Topic[[]*dto.FileDto, struct{}] {
	return p.rules.deleteFiles
}
func (p *Proxy) UnignoreFilesTopic() pubsub.Topic[struct{}, int64] { return p.rules.unignoreFiles }
func (p *Proxy) SetMetaTopic() pubsub.Topic[MetaArgs, struct{}]    { return p.rules.setMeta }

// rule: a rule as a topic — it runs in the writer's transaction, under its own
// savepoint (run)
func rule[A, R any](p *Proxy, class pubsub.Class, fn func(q *Proxy, arg A) (R, error)) *pubsub.Op[*gorm.DB, A, R] {
	return pubsub.New(p.writer, class, func(tx *gorm.DB, arg A) (R, error) {
		var r R
		err := p.run(tx, func(q *Proxy) (err error) {
			r, err = fn(q, arg)
			return err
		})
		return r, err
	})
}

// do: a public write — submitted, waited for. Never from inside a rule: the writer
// would wait for itself; a rule calls the rule (the unexported method) instead.
func do[A, R any](p *Proxy, op *pubsub.Op[*gorm.DB, A, R], arg A) (R, error) {
	if p.inRule {
		panic("model: a public write called inside a rule — call the rule itself")
	}
	return op.Do(arg)
}

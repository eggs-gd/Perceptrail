package model

import (
	"errors"

	"perceptrail/gontroller/internal/model/dto"

	pubsub "github.com/eggs-gd/go-pub-sub"
	"gorm.io/gorm"
)

// Meta: the library's own settings, key → value (the epoch of the delta sync, the
// fingerprint's version…).

// MetaArgs: a setting and its value
type MetaArgs struct {
	Key, Value string
}

// GetMeta returns "" for an unknown key
func (q query) GetMeta(key string) (string, error) {
	var m dto.MetaDto
	err := q.db.Where("key = ?", key).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return m.Value, err
}

func (t *tx) setMeta(m MetaArgs) (pubsub.None, error) {
	return pubsub.None{}, t.db.Save(&dto.MetaDto{Key: m.Key, Value: m.Value}).Error
}

// metaCommands: the settings' writes as commands
type metaCommands struct {
	setMeta op[MetaArgs, pubsub.None]
}

func newMetaCommands(p *Proxy) metaCommands {
	return metaCommands{
		setMeta: command(p, pubsub.Frame, (*tx).setMeta),
	}
}

func (p *Proxy) SetMeta(key, value string) error {
	_, err := p.setMeta.Do(MetaArgs{key, value})
	return err
}

func (p *Proxy) SetMetaCommand() pubsub.Message[MetaArgs] {
	return pubsub.MessageOf(p.setMeta)
}

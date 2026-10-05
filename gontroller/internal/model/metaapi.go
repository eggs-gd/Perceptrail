package model

import (
	"errors"
	pubsub "github.com/eggs-gd/go-pub-sub"

	"perceptrail/gontroller/internal/model/dto"

	"gorm.io/gorm"
)

type MetaApi interface {
	// GetMeta returns "" for an unknown key
	GetMeta(key string) (string, error)
	SetMeta(key, value string) error
}

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

package model

import (
	"errors"

	"perceptrail/gontroller/pkg/model/dto"

	"gorm.io/gorm"
)

type MetaApi interface {
	// GetMeta returns "" for an unknown key
	GetMeta(key string) (string, error)
	SetMeta(key, value string) error
}

func (p *Proxy) GetMeta(key string) (string, error) {
	var m dto.MetaDto
	err := p.db.Where("key = ?", key).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return m.Value, err
}

func (p *Proxy) SetMeta(key, value string) error {
	return p.db.Save(&dto.MetaDto{Key: key, Value: value}).Error
}

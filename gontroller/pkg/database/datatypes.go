package model

import "gorm.io/gorm"

type Proxy struct {
	db *gorm.DB
}

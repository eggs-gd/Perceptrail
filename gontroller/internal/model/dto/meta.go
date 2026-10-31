package dto

// MetaDto is a key/value of the DB itself (e.g. which version of a classifier
// produced the stored data)
type MetaDto struct {
	Key   string `gorm:"primaryKey"`
	Value string
}

func (MetaDto) TableName() string {
	return "meta"
}

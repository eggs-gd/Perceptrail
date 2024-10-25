package model

type ValidationApi interface {
}

type ItemsApi interface {
	CreateItem(item ItemDto) error
	UpdateItem(item ItemDto) error
}

package transcode

import "perceptrail/gontroller/pkg/model/dto"

// Item: what the transcoders work on — an item and its files with their roles, as
// the DB keeps them (the expensive stage is fed from the DB, not by the import)
type Item struct {
	Item  *dto.ItemDto
	Files []*dto.FileDto
}

// kind: what the asset is (the model's rule)
func (it *Item) kind() string {
	source := ""
	if it.Item != nil {
		source = it.Item.Kind
	}
	return dto.AssetKind(source, it.Files)
}

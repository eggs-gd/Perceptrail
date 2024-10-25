package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	t "gontroller/pkg/_t"
	"sort"
	"time"

	"gorm.io/gorm"
)

type ValidationResult int

const (
	NewFile ValidationResult = iota
	KnownFile
	MovedFile
	ModifiedFile
)

func (p *Proxy) GetHash(path string, rawExif t.RawExif, fileSizeBytes uint64, dateTime time.Time) string {
	h := sha256.New()

	// Write the path to the hash
	h.Write([]byte(path))

	// Write the file size to the hash
	h.Write([]byte(fmt.Sprintf("%d", fileSizeBytes)))

	// Include the date/time of the shot
	h.Write([]byte(dateTime.Format(time.RFC3339)))

	// Process the EXIF data
	var keys []string
	for k := range rawExif {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		h.Write([]byte(k))
		h.Write([]byte(rawExif[k]))
	}

	return hex.EncodeToString(h.Sum(nil))
}

// Validate and update DB records
func (p *Proxy) ValidateFile(data t.RawExif) error {
	//var item Item

	var path = string(data[t.PerceptrailPathFieldName])

	// Check if the item exists in the database
	item, err := p.GetItemPathHash(path, hashShort) //Where("path = ?", path).Or("hash_short = ?", hashShort).First(&item).Error

	if err == nil {
		// Found the same item, check if it's moved or modified
		if item.Path == path && item.HashShort == hashShort {
			// Same path, same hash, do nothing
			return nil
		} else if item.HashShort == hashShort && item.Path != path {
			// Item is moved, update path
			item.Path = path
			p.UpdateItem(item)
			fmt.Println("Item moved, path updated:", item.GUID)
		} else if item.Path == path && item.HashShort != hashShort {
			// Same path, different hash, file changed
			item.HashShort = hashShort
			item.HashFull = hashFull
			item.Exif = exif

			p.UpdateItem(item)
			fmt.Println("File changed, EXIF/thumbnails need regeneration:", item.GUID)
		}
	} else if err == gorm.ErrRecordNotFound {
		// New item, create a new record
		newItem := ItemDto{
			GUID:      generateGUID(), // Assume there's a GUID generation function
			Path:      path,
			HashShort: hashShort,
			HashFull:  hashFull,
			Exif:      exif,
		}
		p.CreateItem(item)
		fmt.Println("New item created:", newItem.GUID)
	} else {
		return err
	}

	return nil
}

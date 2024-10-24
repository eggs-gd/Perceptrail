package fswatcher

import (
	"fmt"
	"os"
	"path/filepath"

	"hash/fnv"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Item represents a media item in the database
type Item struct {
	GUID      string `gorm:"primaryKey"`
	Path      string
	HashShort uint32
	HashFull  string
	Exif      string // Placeholder for EXIF data
}

// DB setup
func setupDB() (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open("media_library.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Automatically create tables based on the Item struct
	db.AutoMigrate(&Item{})

	return db, nil
}

// hashFunc calculates a simple hash of the path, exif and size
func hashFunc(path string, exif string, size int64) uint32 {
	h := fnv.New32a()
	h.Write([]byte(path + exif + fmt.Sprintf("%d", size)))
	return h.Sum32()
}

// Walk through filesystem, extract exif and validate
func validateFiles(root string, db *gorm.DB) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Step 1: Extract EXIF data (mocked for simplicity)
		exif := extractExif(path)
		size := info.Size()

		// Step 2: Calculate hashes
		hashShort := hashFunc(path, exif, size)
		hashFull := fmt.Sprintf("%x", hashShort) // Simplified hash for now

		// Step 3: Validator logic
		return validateFile(db, path, hashShort, hashFull, exif)
	})
}

// Validate and update DB records
func validateFile(db *gorm.DB, path string, hashShort uint32, hashFull, exif string) error {
	var item Item

	// Check if the item exists in the database
	err := db.Where("path = ?", path).Or("hash_short = ?", hashShort).First(&item).Error

	if err == nil {
		// Found the same item, check if it's moved or modified
		if item.Path == path && item.HashShort == hashShort {
			// Same path, same hash, do nothing
			return nil
		} else if item.HashShort == hashShort && item.Path != path {
			// Item is moved, update path
			item.Path = path
			db.Save(&item)
			fmt.Println("Item moved, path updated:", item.GUID)
		} else if item.Path == path && item.HashShort != hashShort {
			// Same path, different hash, file changed
			item.HashShort = hashShort
			item.HashFull = hashFull
			item.Exif = exif
			db.Save(&item)
			fmt.Println("File changed, EXIF/thumbnails need regeneration:", item.GUID)
		}
	} else if err == gorm.ErrRecordNotFound {
		// New item, create a new record
		newItem := Item{
			GUID:      generateGUID(), // Assume there's a GUID generation function
			Path:      path,
			HashShort: hashShort,
			HashFull:  hashFull,
			Exif:      exif,
		}
		db.Create(&newItem)
		fmt.Println("New item created:", newItem.GUID)
	} else {
		return err
	}

	return nil
}

// Placeholder for EXIF extraction logic
func extractExif(path string) string {
	// Simulate extraction of EXIF data (in practice, use a real EXIF library)
	return "image/jpeg"
}

// Placeholder for GUID generation
func generateGUID() string {
	return fmt.Sprintf("guid-%d", fnv.New32a().Sum32()) // Simplified GUID generator
}

func main1() {
	// Initialize the database
	db, err := setupDB()
	if err != nil {
		fmt.Println("Failed to connect to database:", err)
		return
	}

	// Start validating files
	root := "./media"
	if err := validateFiles(root, db); err != nil {
		fmt.Println("Error while walking files:", err)
	}
}

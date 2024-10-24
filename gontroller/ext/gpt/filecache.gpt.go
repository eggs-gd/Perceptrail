package fswatcher

import (
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	bolt "go.etcd.io/bbolt"
)

const cacheBucket = "FileCache"

type FileCache struct {
	db *bolt.DB
}

func NewFileCache(dbPath string) *FileCache {
	db, err := bolt.Open(dbPath, 0600, &bolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatal(err)
	}
	// Створюємо бакет (таблицю) для кешу, якщо його ще немає
	db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(cacheBucket))
		return err
	})

	return &FileCache{db: db}
}

func (fc *FileCache) Close() {
	fc.db.Close()
}

// Функція для додавання файлу в кеш
func (fc *FileCache) AddFile(path string, hash string) error {
	return fc.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(cacheBucket))
		return b.Put([]byte(path), []byte(hash))
	})
}

// Перевіряємо, чи файл вже був оброблений
func (fc *FileCache) IsProcessed(path string, hash string) bool {
	var cachedHash []byte
	fc.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(cacheBucket))
		cachedHash = b.Get([]byte(path))
		return nil
	})

	return string(cachedHash) == hash
}

// Хешування файлу
func hashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}

func main() {
	cache := NewFileCache("filecache.db")
	defer cache.Close()

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			hash, err := hashFile(path)
			if err != nil {
				return err
			}

			if !cache.IsProcessed(path, hash) {
				fmt.Println("New or changed file:", path)
				cache.AddFile(path, hash)
			}
		}
		return nil
	})
	if err != nil {
		fmt.Println("Error walking the path:", err)
	}
}

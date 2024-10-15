package items

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

// Структура для відповіді із файлами в папці
type FolderContent struct {
	Files []string `json:"files"`
}

// Функція для обробки запитів до конкретної папки
func ScanFolder(w http.ResponseWriter, r *http.Request) {
	// Читаємо параметр 'path' з URL
	folderPath := r.URL.Query().Get("path")
	if folderPath == "" {
		http.Error(w, "Parameter 'path' is required", http.StatusBadRequest)
		return
	}

	// Перевірка чи папка існує
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		http.Error(w, "Folder does not exist", http.StatusNotFound)
		return
	}

	// Отримання списку файлів
	var files []string
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		http.Error(w, "Failed to scan folder", http.StatusInternalServerError)
		return
	}

	// Формуємо відповідь
	response := FolderContent{Files: files}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

package images

import (
	"log"

	"github.com/h2non/bimg"
)

func processImage(inputPath string, outputPath string) {
	// Читання зображення
	buffer, err := bimg.Read(inputPath)
	if err != nil {
		log.Fatalf("failed to read image: %v", err)
	}

	// Отримання інформації про зображення
	imageInfo, err := bimg.Metadata(buffer)
	if err != nil {
		log.Fatalf("failed to get image metadata: %v", err)
	}

	// Отримання оригінальних розмірів
	originalWidth := imageInfo.Size.Width
	originalHeight := imageInfo.Size.Height
	log.Printf("Original size: %dx%d", originalWidth, originalHeight)

	// Масштабування до 720p по висоті (зберігаючи пропорції)
	newHeight := 720
	newWidth := (originalWidth * newHeight) / originalHeight
	newImage, err := bimg.NewImage(buffer).Resize(newWidth, newHeight)
	if err != nil {
		log.Fatalf("failed to resize image: %v", err)
	}

	// Збереження зображення у форматі WebP
	newImage, err = bimg.NewImage(newImage).Process(bimg.Options{
		Type:    bimg.WEBP,
		Quality: 90,
	})
	if err != nil {
		log.Fatalf("failed to process image: %v", err)
	}

	// Запис зображення
	err = bimg.Write(outputPath, newImage)
	if err != nil {
		log.Fatalf("failed to save image: %v", err)
	}

	log.Printf("Image saved to %s", outputPath)
}

func main() {
	inputFile := "input.jpg"
	outputFile := "output.webp"
	processImage(inputFile, outputFile)
}

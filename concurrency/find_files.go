package concurrency

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FindFiles buduje potok (pipeline) z trzech etapów:
//
//	findFiles -> filterByExtension -> filterByContent
//
// Zasada, którą pokazuje ten przykład: kanał zamyka WYŁĄCZNIE nadawca i tylko
// wtedy, gdy jest jedynym nadawcą. Kiedy nadawców jest wielu (tutaj dwie
// goroutines wołające findFiles), czekamy na nich przez sync.WaitGroup
// i zamykamy kanał dopiero po ich zakończeniu.
func FindFiles() {
	files := make(chan string, 10)
	filesWithExtension := make(chan string)
	filesWithContent := make(chan string)

	// Etap 1: dwóch nadawców pisze do tego samego kanału, więc żaden z nich
	// nie może go zamknąć samodzielnie - robi to osobna goroutine po wg.Wait().
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		findFiles("./common", files)
	}()
	go func() {
		defer wg.Done()
		findFiles("./concurrency", files)
	}()
	go func() {
		wg.Wait()
		close(files)
	}()

	// Etapy 2 i 3: każdy ma dokładnie jednego nadawcę, więc zamyka swój kanał sam.
	go func() {
		defer close(filesWithExtension)
		filterByExtension(files, filesWithExtension, ".go")
	}()
	go func() {
		defer close(filesWithContent)
		filterByContent(filesWithExtension, filesWithContent, "package concurrency")
	}()

	// Pętla kończy się, gdy ostatni etap zamknie kanał - bez sztucznego timeoutu.
	for file := range filesWithContent {
		fmt.Println(file)
	}
}

func findFiles(path string, files chan<- string) {
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files <- path
		}
		return nil
	})
	// panic w goroutine ubija cały proces i nie da się go przechwycić
	// w funkcji wywołującej - logujemy i kończymy tylko ten etap.
	if err != nil {
		log.Printf("Błąd przeszukiwania %s: %v", path, err)
	}
}

func filterByExtension(files <-chan string, filesWithExtension chan<- string, extension string) {
	for file := range files {
		if strings.HasSuffix(file, extension) {
			filesWithExtension <- file
		}
	}
}

func filterByContent(filesWithExtension <-chan string, filesWithContent chan<- string, text string) {
	for file := range filesWithExtension {
		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		if strings.Contains(string(content), text) {
			filesWithContent <- file
		}
	}
}

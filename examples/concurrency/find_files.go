package concurrency

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FindFiles builds a three-stage pipeline:
//
//	findFiles -> filterByExtension -> filterByContent
//
// The rule this example demonstrates: ONLY the sender closes a channel, and only
// when it is the sole sender. When there are several senders (here: two goroutines
// calling findFiles) we wait for them with a sync.WaitGroup and close the channel
// only once they have all finished.
func FindFiles() {
	files := make(chan string, 10)
	filesWithExtension := make(chan string)
	filesWithContent := make(chan string)

	// Stage 1: two senders write to the same channel, so neither may close it on
	// its own - a separate goroutine does that after wg.Wait().
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		findFiles("./examples/common", files)
	}()
	go func() {
		defer wg.Done()
		findFiles("./examples/concurrency", files)
	}()
	go func() {
		wg.Wait()
		close(files)
	}()

	// Stages 2 and 3: each has exactly one sender, so each closes its own channel.
	go func() {
		defer close(filesWithExtension)
		filterByExtension(files, filesWithExtension, ".go")
	}()
	go func() {
		defer close(filesWithContent)
		filterByContent(filesWithExtension, filesWithContent, "package concurrency")
	}()

	// The loop ends when the last stage closes its channel - no artificial timeout.
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
	// A panic in a goroutine takes down the whole process and cannot be recovered
	// by the caller - so we log and end just this stage.
	if err != nil {
		log.Printf("error walking %s: %v", path, err)
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

package examples

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
)

func Grep() {
	if len(os.Args) < 3 {
		log.Fatalf("Usage: grep pattern path")
	}

	pattern, err := regexp.Compile(os.Args[1])
	if err != nil {
		log.Fatalf("Niepoprawne wyrażenie regularne %q: %v", os.Args[1], err)
	}

	path := os.Args[2]

	// Przekazujemy *regexp.Regexp, a nie kopię wartości: kopiowanie Regexp jest
	// odradzane (unieważnia wewnętrzny cache automatu dopasowania - z tego
	// powodu Regexp.Copy zostało oznaczone jako deprecated w Go 1.12).
	if err := filepath.Walk(path, search(pattern)); err != nil {
		log.Fatalf("Error walking on path %s: %v", path, err)
	}
}

func search(pattern *regexp.Regexp) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			// log.Fatalf tutaj wywoływało os.Exit(1): pomijało wszystkie defery
			// (w tym zamknięcie pliku), czyniło poniższy return martwym kodem
			// i przerywało cały przebieg z powodu jednego nieczytelnego pliku.
			// Zwracamy nil, żeby pominąć plik i iść dalej.
			log.Printf("Pomijam %q: %v", path, err)
			return nil
		}
		defer func() {
			if err := file.Close(); err != nil {
				log.Printf("Błąd zamykania %q: %v", path, err)
			}
		}()

		scanner := bufio.NewScanner(file)
		lineNumber := 0
		for scanner.Scan() {
			line := scanner.Text()
			lineNumber++
			if pattern.MatchString(line) {
				fmt.Printf("%s (line: %d): %s\n", path, lineNumber, line)
			}
		}
		// Wcześniej było tu `return err` - ale err to parametr WalkFunc,
		// nadpisany przez os.Open powyżej i w tym miejscu zawsze nil, więc
		// funkcja tylko udawała, że propaguje błędy. Właściwy błąd pochodzi
		// ze skanera (I/O albo wiersz dłuższy niż 64 KiB - łatwe do trafienia
		// w pliku binarnym).
		return scanner.Err()
	}
}

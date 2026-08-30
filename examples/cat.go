package examples

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
)

func Cat() {
	numberLines := flag.Bool("n", false, "Number lines")
	numberNonEmptyLines := flag.Bool("nb", false, "Number non empty lines")
	flag.Parse()
	paths := flag.Args()

	if len(paths) == 0 || (*numberLines && *numberNonEmptyLines) {
		// Błąd użycia idzie na stderr i kończy się kodem != 0 - inaczej skrypt
		// wywołujący cat nie odróżni pomyłki od poprawnego, pustego wyniku.
		fmt.Fprintln(os.Stderr, "Usage: cat [-n|-nb] path, path ...")
		os.Exit(2)
	}

	// Jeden printer na cały przebieg: numeracja jest ciągła przez wszystkie
	// pliki, tak jak w cat(1).
	printerFn := printerFactory(*numberLines, *numberNonEmptyLines)

	for _, path := range paths {
		fmt.Printf("File: %s\n", path)
		if err := cat(path, printerFn); err != nil {
			log.Fatalf("Błąd odczytu %q: %v", path, err)
		}
	}
}

// printer dostaje sam wiersz - licznik jest stanem wewnętrznym konkretnej
// implementacji. Wcześniej licznik żył w funkcji cat i rósł dla KAŻDEGO
// wiersza, a wariant -nb jedynie ukrywał numer przy pustych wierszach.
// Dla wejścia "a\n\nb\n" dawało to 1, (pusty), 3 zamiast poprawnego 1, (pusty), 2.
type printer = func(string)

func printerFactory(numberLines, numberNonEmptyLines bool) printer {
	lineNumber := 0
	switch {
	case numberLines:
		return func(line string) {
			lineNumber++
			fmt.Printf("%6d: %s\n", lineNumber, line)
		}
	case numberNonEmptyLines:
		return func(line string) {
			if line == "" {
				fmt.Println(line)
				return
			}
			lineNumber++
			fmt.Printf("%6d: %s\n", lineNumber, line)
		}
	default:
		return func(line string) {
			fmt.Println(line)
		}
	}
}

func cat(path string, printerFn printer) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	// log.Fatal w defer wywołuje os.Exit i pomija wszystkie pozostałe defery,
	// więc błąd zamknięcia tylko logujemy.
	defer func() {
		if err := file.Close(); err != nil {
			log.Printf("Błąd zamykania %q: %v", path, err)
		}
	}()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		printerFn(scanner.Text())
	}
	// Scan zwraca false także przy błędzie I/O oraz przy wierszu dłuższym niż
	// bufio.MaxScanTokenSize (64 KiB) - bez tego obcięte wyjście wyglądałoby
	// jak poprawnie odczytany plik.
	return scanner.Err()
}

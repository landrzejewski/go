package examples

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func Find() {
	path := flag.String("p", "", "Start path")
	name := flag.String("n", "", "Name to match")
	fileType := flag.String("t", "", "Type to match (file, dir, symlink)")

	flag.Parse()

	if *name == "" || *fileType == "" {
		flag.Usage()
		return
	}

	switch *fileType {
	case "file", "dir", "symlink":
	default:
		log.Fatalf("Nieznany typ %q, dozwolone: file, dir, symlink", *fileType)
	}

	if *path == "" {
		*path = "."
	}

	if err := filepath.Walk(*path, onElement(*fileType, *name)); err != nil {
		log.Fatalf("Błąd przeszukiwania %q: %v", *path, err)
	}
}

func onElement(fileType, name string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !strings.Contains(info.Name(), name) {
			return nil
		}
		// Uwaga: wcześniej pierwszy case brzmiał `case fileType:`, czyli
		// porównywał zmienną samą ze sobą - pasował ZAWSZE, przez co gałęzie
		// "dir" i "symlink" były martwym kodem, a -t dir działało jak -t file.
		switch fileType {
		case "file":
			if info.Mode().IsRegular() {
				fmt.Println(path)
			}
		case "dir":
			if info.Mode().IsDir() {
				fmt.Println(path)
			}
		case "symlink":
			if info.Mode()&os.ModeSymlink == os.ModeSymlink {
				fmt.Println(path)
			}
		}
		return nil
	}
}

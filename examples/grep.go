package examples

import (
	"bufio"
	"fmt"
	"io/fs"
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
		log.Fatalf("invalid regular expression %q: %v", os.Args[1], err)
	}

	path := os.Args[2]

	// We pass a *regexp.Regexp, not a copy of the value: copying a Regexp is
	// discouraged (it invalidates the internal match-automaton cache - which is
	// why Regexp.Copy was deprecated in Go 1.12).
	//
	// WalkDir rather than Walk: recommended since Go 1.16, because the callback
	// receives an fs.DirEntry instead of an fs.FileInfo and so avoids a stat(2)
	// call per entry.
	if err := filepath.WalkDir(path, search(pattern)); err != nil {
		log.Fatalf("Error walking on path %s: %v", path, err)
	}
}

func search(pattern *regexp.Regexp) fs.WalkDirFunc {
	return func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// Skip what we cannot read rather than aborting the whole walk.
			log.Printf("skipping %q: %v", path, err)
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			// log.Fatalf here used to call os.Exit(1): it skipped every defer
			// (including closing the file), made the return below dead code, and
			// aborted the entire run because of one unreadable file. Return nil
			// to skip the file and keep going.
			log.Printf("skipping %q: %v", path, err)
			return nil
		}
		defer func() {
			if err := file.Close(); err != nil {
				log.Printf("error closing %q: %v", path, err)
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
		// This used to be `return err` - but err is the WalkDirFunc parameter,
		// shadowed by os.Open above and always nil at this point, so the function
		// only pretended to propagate errors. The real error comes from the
		// scanner (I/O, or a line longer than 64 KiB - easy to hit in a binary
		// file).
		return scanner.Err()
	}
}

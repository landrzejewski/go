// Command go-training is the entry point for this Go training repository.
//
// The language course lives in the basics package, one module per topic:
//
//	go run .        # list every module
//	go run . 004    # run a single module
//	go run . all    # run the whole course, in order
//
// The other directories hold the exercise solutions listed in notes.md: concurrency/ for the
// goroutine material, examples/ for the CLI, REST, chat, budget and flat-file-database exercises,
// and common/ for the shared generic helpers.
package main

import (
	"fmt"
	"os"

	"training.pl/go/basics"
)

func main() {
	args := os.Args[1:]

	if len(args) == 0 {
		usage()
		return
	}

	for _, arg := range args {
		switch arg {
		case "all":
			basics.RunAll()
		case "-h", "--help", "help", "list":
			usage()
		default:
			module, ok := basics.Find(arg)
			if !ok {
				fmt.Fprintf(os.Stderr, "unknown module %q\n\n", arg)
				usage()
				os.Exit(1)
			}
			fmt.Printf("############ Module %s — %s ############\n\n", module.ID, module.Title)
			module.Run()
		}
	}
}

func usage() {
	fmt.Println("Go training — language modules")
	fmt.Println()
	fmt.Println("usage:")
	fmt.Println("  go run .        list the modules")
	fmt.Println("  go run . <id>   run one module, e.g. `go run . 004`")
	fmt.Println("  go run . all    run every module in order")
	fmt.Println()
	fmt.Println("modules:")
	for _, m := range basics.Modules {
		fmt.Printf("  %-5s %s\n", m.ID, m.Title)
	}
	fmt.Println()
	fmt.Println("other demos in this repo (see notes.md for the exercise statements):")
	fmt.Println("  concurrency.Barbers()     sleeping barber problem")
	fmt.Println("  concurrency.Run()         goroutines, channels, select")
	fmt.Println("  examples.RestApi()        Gin + PostgreSQL REST API (needs docker-compose up)")
	fmt.Println("  examples.Cat()            cat(1) clone with -n / -nb")
	fmt.Println("  budget.*                  household budget CLI with JSON persistence")
	fmt.Println("  db.DatabaseTest()         flat-file binary database")
	fmt.Println("  chat.Server / chat.Client TCP chat")
}

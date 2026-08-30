// Command go-training is the entry point for this Go training repository.
//
// The language course lives in the basics package, one module per topic:
//
//	go run .        # list every module
//	go run . 004    # run a single module
//	go run . all    # run the whole course, in order
//
// The exercise solutions listed in notes.md all live under examples/: examples/concurrency/ for
// the goroutine material, examples/common/ for the shared generic helpers, and examples/ itself
// for the CLI, REST, chat, budget and flat-file-database exercises.
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
	fmt.Println("  these are library packages - call them from a test or a scratch main,")
	fmt.Println("  except the chat, which is two real commands:")
	fmt.Println()
	fmt.Println("    examples/concurrency  Barbers(), Run(), FindFiles(), ProducerConsumer*()")
	fmt.Println("    examples/common       Stack[T], Add[T], gob helpers")
	fmt.Println("    examples              RestApi() (needs docker-compose up), Cat(), Find(), Grep(),")
	fmt.Println("                          Echo(), TicTacToe(), MonetaryAmount(), Reflect()")
	fmt.Println("    examples/budget       household budget entries with JSON persistence")
	fmt.Println("    examples/db           DatabaseTest() - flat-file binary database")
	fmt.Println()
	fmt.Println("    go run ./examples/chat/server    TCP chat server")
	fmt.Println("    go run ./examples/chat/client    TCP chat client")
}

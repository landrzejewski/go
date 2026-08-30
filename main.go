// Command go-training is the entry point for this Go training repository.
//
// The language course lives in the basics package, one module per topic:
//
//	go run .        # list every module
//	go run . 004    # run a single module
//	go run . all    # run the whole course, in order
//
// The exercise solutions listed in notes.md live under examples/: the
// examples package itself holds the CLI tools, the REST API, tic-tac-toe and
// MonetaryAmount; examples/concurrency the goroutine material;
// examples/common the shared generic helpers; and the sub-packages
// examples/chat, examples/budget and examples/db the chat, household budget
// and flat-file database exercises.
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
	fmt.Println("    examples              Echo(), Cat(), Find(), Grep(), TicTacToe(), Reflect(),")
	fmt.Println("                          MonetaryAmountDemo(), RestAPI() (needs docker-compose up)")
	fmt.Println("    examples/concurrency  Barbers(), ChannelsDemo(), FindFiles(pattern, roots...),")
	fmt.Println("                          ProducerConsumerClassic(), ProducerConsumerChannels(),")
	fmt.Println("                          Barrier, Semaphore")
	fmt.Println("    examples/common       Stack[T], Add[T Number], ToBytes/FromBytes (gob)")
	fmt.Println("    examples/budget       New(path), ParseEntry(args), Load/Add/Print/Save")
	fmt.Println("    examples/db           Open(path, gen), DatabaseTest(), DatabaseExercise() (HTTP on :8080)")
	fmt.Println()
	fmt.Println("    go run ./examples/chat/server    TCP chat server")
	fmt.Println("    go run ./examples/chat/client    TCP chat client")
}

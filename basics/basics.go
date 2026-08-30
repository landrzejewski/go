// Package basics is a complete, runnable tour of the Go language, built as training material.
//
// Each module is numbered and carries a banner, a Markdown theory block per section, and one
// runnable demo function per section. Read a module top to bottom, then run it and match the
// output to the code:
//
//	go run .        # list the modules
//	go run . 004    # run one module
//	go run . all    # run all of them, in order
//
// # Why every package-level name carries an mNNN prefix
//
// In Rust each module is its own namespace, so every file can have a private fn run(). Go is
// different: all files of a package share ONE flat namespace. Because basics is a single package
// spread over many files, a package-level add in module 005 would collide with a package-level add
// in module 010. Every package-level identifier is therefore prefixed with its module number.
// Identifiers local to a function need no prefix and stay idiomatic — and that is most of the code.
package basics

import "fmt"

// Module is one numbered unit of the course.
type Module struct {
	ID    string // "001a", "004", "016" — as passed on the command line
	Title string
	Run   func()
}

// Modules lists every module of the course, in teaching order.
var Modules = []Module{
	{"001a", "Comments, Packages, Variables, Zero Values, Scope, Shadowing", Run001a},
	{"001b", "Constants, iota, Package Variables, init", Run001b},
	{"002a", "Basic Types: numbers, booleans, strings, runes", Run002a},
	{"002b", "Composite Types: arrays, slices, maps, structs, defined types", Run002b},
	{"003", "Operators: arithmetic, comparison, logical, bitwise, precedence", Run003},
	{"004", "Control Flow: if, switch, for, range, labels, goto, defer", Run004},
	{"005", "Functions: returns, variadics, values, closures, panic/recover", Run005},
	{"006", "Pointers and Memory: value semantics, escape analysis, aliasing, unsafe", Run006},
	{"007", "Structs and Methods: receivers, method sets, embedding, tags", Run007},
	{"008", "Interfaces: implicit satisfaction, the nil trap, type switches, composition", Run008},
	{"009", "Errors: sentinels, wrapping, custom types, errors.Is/As/AsType/Join", Run009},
	{"010", "Generics: type parameters, constraints, inference, generic methods (1.27)", Run010},
	{"011", "Concurrency: goroutines, channels, select, sync, context, patterns", Run011},
	{"012", "Iterators and Collections: range-over-func, iter, slices, maps, cmp", Run012},
	{"013", "Standard Library: fmt, strings, time, os/io, json + json/v2, slog, uuid", Run013},
	{"014", "Testing: table-driven tests, benchmarks, fuzzing, examples, synctest", Run014},
	{"015", "Modules and Tooling: go.mod, build constraints, embed, vet, profiling", Run015},
	{"016", "What Changed, Go 1.21 to 1.27: the delta since you last learned Go", Run016},
	{"017", "CLI Applications: flag, subcommands, validation, env vars, configuration", Run017},
	{"018", "HTTP Services: handlers, routing, middleware, errors, graceful shutdown", Run018},
	{"019", "database/sql and the Repository Pattern: pool, scanning, transactions", Run019},
	{"020", "GORM: models, AutoMigrate, CRUD, Preload and N+1, associations, hooks", Run020},
	{"021", "Publishing Libraries and CLI Tools: API design, versioning, releases", Run021},
}

// Find returns the module with the given ID.
func Find(id string) (Module, bool) {
	for _, m := range Modules {
		if m.ID == id {
			return m, true
		}
	}
	return Module{}, false
}

// RunAll runs every module in teaching order.
func RunAll() {
	for i, m := range Modules {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("############ Module %s — %s ############\n\n", m.ID, m.Title)
		m.Run()
	}
}

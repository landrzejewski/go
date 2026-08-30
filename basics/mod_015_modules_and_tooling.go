package basics

import (
	"fmt"
	"go/build"
	"runtime"
	"runtime/debug"
	"strings"
)

/*
# Module 015 — Modules, Build Constraints and Tooling

Go ships one toolchain, and it does everything: build, test, format, vet, document, profile, manage
dependencies, and fetch its own newer versions. There is no separate build tool, no plugin system
and — apart from `go.mod` — no configuration file. That uniformity is a large part of why Go
codebases feel similar to each other.
*/

// =================================================================================================
// Section 1: Modules and go.mod
// =================================================================================================

/*
## Modules and go.mod

- A **module** is a tree of packages with a `go.mod` at its root. The module path is its identity
  and the prefix of every package import path inside it. This repository's is `training.pl/go`, so
  this package is imported as `training.pl/go/basics`.
- `go.mod` has a small set of directives:

	module training.pl/go        // the module path
	go 1.27                      // the LANGUAGE VERSION, not the toolchain version
	toolchain go1.27.0           // the minimum toolchain, added automatically when needed
	require ( ... )              // direct and, marked `// indirect`, transitive dependencies
	replace old => new           // redirect a path, e.g. to a local checkout
	exclude mod v1.2.3           // never select this version
	retract v1.0.1               // (in your own module) tell users this release is bad
	tool golang.org/x/tools/...  // Go 1.24: a tool dependency

- **The `go` line is a language-version selector**, and this is subtle: it decides which language
  features are available *per module*. A module saying `go 1.21` gets the pre-1.22 loop-variable
  semantics even when built with Go 1.27. That is how Go keeps its compatibility promise while
  still changing the language.
- **`go.sum`** records a cryptographic hash of every module version used. It is not a lock file —
  `go.mod` already pins versions — it is a **tamper check**. Commit both.
- **Minimal Version Selection (MVS)** is Go's answer to dependency resolution: the version chosen is
  the **highest version anyone in the graph explicitly asks for**, and no higher. There is no
  solver, the result is reproducible without a lock file, and adding a dependency cannot silently
  upgrade an unrelated one.
- **Semantic import versioning**: `v2` and above put the major version **in the import path** —
  `github.com/foo/bar/v2`. So `v1` and `v2` are different packages and can coexist in one build.
  This is why `math/rand/v2` and `encoding/json/v2` are named that way.
- **`internal/`** is enforced by the toolchain: only code rooted at the parent of an `internal`
  directory may import it.
- The commands: `go mod init`, `go mod tidy` (add what is used, remove what is not — run it before
  every commit), `go mod why`, `go mod graph`, `go mod verify`, `go mod vendor`, `go list -m all`.
*/

func m015Modules() {
	fmt.Println("--- Section 1: Modules and go.mod ---")

	// debug.ReadBuildInfo exposes the module graph of the running binary.
	if info, ok := debug.ReadBuildInfo(); ok {
		fmt.Printf("  main module: %s\n", info.Main.Path)
		fmt.Printf("  built with:  %s\n", info.GoVersion)
		fmt.Printf("  direct and indirect dependencies: %d\n", len(info.Deps))
		shown := 0
		for _, dep := range info.Deps {
			if shown >= 4 {
				break
			}
			fmt.Printf("    %-40s %s\n", dep.Path, dep.Version)
			shown++
		}
		// The build settings record how the binary was produced - VCS revision included.
		for _, s := range info.Settings {
			switch s.Key {
			case "GOARCH", "GOOS", "vcs.revision", "-race", "vcs.modified":
				fmt.Printf("    build setting %-14s = %s\n", s.Key, s.Value)
			}
		}
	}

	fmt.Println()
	fmt.Println("  the `go` line is a LANGUAGE version selector, per module:")
	fmt.Println("    a module saying `go 1.21` keeps pre-1.22 loop-variable semantics")
	fmt.Println("    even when compiled by Go 1.27 - that is the compatibility promise")
	fmt.Println("  go.sum is a tamper check, not a lock file - go.mod already pins versions")
	fmt.Println("  MVS picks the HIGHEST version anyone explicitly requires, and no higher")
	fmt.Println("  v2+ goes in the import path: github.com/foo/bar/v2, math/rand/v2, encoding/json/v2")
	fmt.Println()
	fmt.Println("  go mod tidy      <- run before every commit")
	fmt.Println("  go mod why -m X  <- why is this dependency here?")
	fmt.Println("  go list -m all   <- the full selected version graph")
}

// =================================================================================================
// Section 2: Dependencies, Tools and Workspaces
// =================================================================================================

/*
## Dependencies, Tools and Workspaces

### Getting dependencies

	go get example.com/pkg@latest     add or upgrade
	go get example.com/pkg@v1.2.3     a specific version
	go get example.com/pkg@none       remove
	go get -u ./...                   upgrade everything (be careful)

Since **Go 1.18**, `go get` no longer builds or installs packages — that is `go install
pkg@version`, which builds in module-aware mode without touching your `go.mod`. (Go 1.22 went one
step further and dropped `go get` outside a module in legacy GOPATH mode.)

### Tool dependencies (Go 1.24)

Pinning a code generator or linter used to need the `tools.go` hack: a file with a `//go:build
tools` constraint importing packages purely so `go mod tidy` would keep them. Go 1.24 replaced it
with a real directive:

	go get -tool golang.org/x/tools/cmd/stringer   # adds a `tool` line to go.mod
	go tool stringer -type=Weekday                 # runs the pinned version
	go tool                                        # lists what is available

Every contributor and CI runner now gets the same tool version, from the same file that pins
everything else.

### Workspaces (Go 1.18)

A `go.work` file lets several modules be developed together without `replace` directives polluting
`go.mod`:

	go work init ./api ./worker ./shared
	go work use ./newmodule

`go.work` is **local development state** and is normally **not committed** — `replace` in `go.mod`
was the old approach and had the fatal flaw that it shipped to your users.

### Vendoring

`go mod vendor` copies every dependency into `vendor/`. If that directory exists, builds use it
automatically. It is worth it for hermetic or air-gapped builds, and a nuisance otherwise.

### The toolchain line

Since Go 1.21 the `toolchain` directive lets a module require a newer Go than the one invoked, and
the installed `go` command will **download and run it automatically**. `GOTOOLCHAIN=local` disables
that.
*/

func m015Dependencies() {
	fmt.Println("\n--- Section 2: Dependencies, Tools and Workspaces ---")

	fmt.Printf("  this toolchain: %s on %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  GOPATH: %s\n", build.Default.GOPATH)

	fmt.Println()
	fmt.Println("  go get pkg@latest / @v1.2.3 / @none")
	fmt.Println("  go install pkg@version   <- installs a BINARY, without touching go.mod")
	fmt.Println("  (since Go 1.18 `go get` no longer builds or installs packages)")
	fmt.Println()
	fmt.Println("  Go 1.24 tool dependencies replace the old tools.go hack:")
	fmt.Println("    go get -tool golang.org/x/tools/cmd/stringer")
	fmt.Println("    go tool stringer -type=m001bWeekday")
	fmt.Println("    go tool                       <- list the pinned tools")
	fmt.Println()
	fmt.Println("  workspaces, for developing several modules together:")
	fmt.Println("    go work init ./api ./worker ./shared")
	fmt.Println("  go.work is LOCAL state and is normally not committed - unlike a `replace`")
	fmt.Println("  directive in go.mod, which would ship to your users")
	fmt.Println()
	fmt.Println("  a nested go.mod CUTS its subtree out of the parent module: `go build ./...`")
	fmt.Println("  at the root stops descending into it, and `go list ./...` never lists it -")
	fmt.Println("  which is exactly how a nested module goes stale unnoticed. Use a workspace")
	fmt.Println("  (or one module) unless the subtree is really published on its own.")
	fmt.Println()
	fmt.Println("  GOTOOLCHAIN=auto (the default) downloads a newer Go if go.mod asks for one;")
	fmt.Println("  GOTOOLCHAIN=local refuses to")
}

// =================================================================================================
// Section 3: Build Constraints and go:embed
// =================================================================================================

/*
## Build Constraints and go:embed

### Build constraints

A `//go:build` line before the package clause, separated from it by a blank line, decides whether a
file is compiled at all:

	//go:build linux && amd64
	//go:build (linux || darwin) && !cgo
	//go:build go1.27
	//go:build integration

- It is a boolean expression over **build tags**: `GOOS` values, `GOARCH` values, `go1.N` version
  tags, `cgo`, `race`, `unix`, and any custom tag passed with `-tags`.
- The syntax is `//go:build` with **no space** after the slashes. The older `// +build` form is
  deprecated; `gofmt` keeps them in sync while both exist.
- **Filename suffixes are an implicit constraint**: `file_linux.go`, `file_windows_amd64.go`,
  `file_test.go`. This is usually cleaner than a `//go:build` line for simple OS splits — see
  `examples/chat/client/platform_unix.go` and `platform_windows.go` in this repository.
- The common uses: platform-specific implementations, gating integration tests behind
  `-tags integration`, and excluding experimental code from the default build.

### go:embed

	//go:embed static/index.html
	var indexHTML string

	//go:embed templates/*.tmpl
	var templates embed.FS

- `//go:embed` (Go 1.16) compiles files **into the binary** at build time. The variable must be at
  **package level** and of type `string`, `[]byte`, or `embed.FS`.
- `embed.FS` satisfies `fs.FS`, so it works directly with `http.FileServer`, `template.ParseFS` and
  anything else taking a filesystem.
- Restrictions: the paths are relative to the source file, cannot use `..`, cannot escape the
  module, and by default do not include files beginning with `.` or `_` — use `all:` to include
  them. Embedding a directory that does not exist is a **compile error**, which is the point: a
  missing asset fails the build rather than the deployment.
- Importing `embed` is required even when only `string` or `[]byte` is used; the blank import
  `_ "embed"` is the idiom for that case.
*/

func m015BuildAndEmbed() {
	fmt.Println("\n--- Section 3: Build Constraints and go:embed ---")

	fmt.Printf("  this build: GOOS=%s GOARCH=%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  build tags active for this platform include: %s\n",
		strings.Join(m015ActiveTags(), ", "))

	fmt.Println()
	fmt.Println("  //go:build linux && amd64          <- no space after the slashes")
	fmt.Println("  //go:build (linux || darwin) && !cgo")
	fmt.Println("  //go:build integration             <- go test -tags integration")
	fmt.Println("  it must come BEFORE the package clause, with a blank line between")
	fmt.Println()
	fmt.Println("  a filename suffix is an implicit constraint, and is usually cleaner:")
	fmt.Println("    platform_unix.go / platform_windows.go  <- as in examples/chat/client/")
	fmt.Println()
	fmt.Println("  //go:embed compiles files INTO the binary:")
	fmt.Println("    //go:embed static/index.html")
	fmt.Println("    var indexHTML string")
	fmt.Println("    //go:embed templates/*.tmpl")
	fmt.Println("    var templates embed.FS        <- satisfies fs.FS")
	fmt.Println("  a missing file is a COMPILE error, so a missing asset fails the build,")
	fmt.Println("  not the deployment")
}

func m015ActiveTags() []string {
	tags := []string{runtime.GOOS, runtime.GOARCH}
	switch runtime.GOOS {
	case "linux", "darwin", "freebsd", "openbsd", "netbsd", "dragonfly", "solaris", "aix":
		tags = append(tags, "unix")
	}
	tags = append(tags, fmt.Sprintf("go1.%d and every earlier go1.N", m015MinorVersion()))
	return tags
}

func m015MinorVersion() int {
	v := runtime.Version() // e.g. "go1.27.0"
	v = strings.TrimPrefix(v, "go1.")
	minor, _, _ := strings.Cut(v, ".")
	n := 0
	for _, r := range minor {
		if r < '0' || r > '9' {
			break
		}
		n = n*10 + int(r-'0')
	}
	return n
}

// =================================================================================================
// Section 4: The go Command
// =================================================================================================

/*
## The go Command

	go run .                    build and run, without leaving a binary
	go build ./...              build every package; ./... means "this tree"
	go install ./cmd/app        build and place the binary in $GOBIN
	go test ./...               test everything
	go vet ./...                report likely mistakes
	go fmt ./...                format (a wrapper around gofmt -l -w)
	go doc net/http Handler     documentation, in the terminal
	go doc -http=:6060          Go 1.25: serve the docs locally in a browser
	go clean -cache -testcache  when you suspect a stale cache
	go env / go env -w          inspect and persist environment settings
	go version -m ./binary      the module graph baked into a built binary
	go generate ./...           run //go:generate directives

Points worth knowing:

- **Builds are cached** aggressively, and so are **test results**: a passing test with unchanged
  inputs is not re-run, and prints `(cached)`. `-count=1` forces a real run.
- **Cross-compilation is a one-liner**: `GOOS=linux GOARCH=arm64 go build`. No toolchain to install,
  because the standard library is compiled on demand. `CGO_ENABLED=0` gives a fully static binary,
  which is why Go containers can be built `FROM scratch`.
- **`go build -ldflags="-s -w"`** strips the symbol table and DWARF data, typically cutting binary
  size by a quarter. `-ldflags="-X main.version=1.2.3"` injects a value into a string variable at
  link time — the standard way to stamp a version.
- **Profile-guided optimisation (PGO)** (Go 1.21): drop a `default.pgo` next to `main` and the
  compiler uses it automatically. Typical gains are 2-7% for no code changes.
- **`go generate`** runs `//go:generate` comments. It is not part of `go build`: you run it and
  commit the output. Combined with the Go 1.24 `tool` directive, the generator's version is pinned.
- **`GODEBUG`** enables and disables specific behaviour changes, which is what makes the Go 1
  compatibility promise workable in practice: when a release changes a behaviour, it also adds a
  `GODEBUG` setting to restore the old one. The `go` line in `go.mod` sets the defaults.
*/

func m015GoCommand() {
	fmt.Println("\n--- Section 4: The go Command ---")

	fmt.Println("  go run . / go build ./... / go test ./... / go vet ./... / go fmt ./...")
	fmt.Println("  go doc -http=:6060          <- Go 1.25: the docs in a browser, offline")
	fmt.Println("  go version -m ./binary      <- the module graph baked into a built binary")
	fmt.Println()
	fmt.Println("  builds AND test results are cached; a cached test prints `(cached)`")
	fmt.Println("  use -count=1 to force a real run")
	fmt.Println()
	fmt.Printf("  cross-compiling from %s/%s needs no extra toolchain:\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println("    GOOS=linux   GOARCH=arm64 go build ./...")
	fmt.Println("    GOOS=windows GOARCH=amd64 go build ./...")
	fmt.Println("    CGO_ENABLED=0 go build    <- a fully static binary, for FROM scratch images")
	fmt.Println()
	fmt.Println("  go build -ldflags=\"-s -w\"                 <- strip symbols, ~25% smaller")
	fmt.Println("  go build -ldflags=\"-X main.version=1.2.3\" <- stamp a version at link time")
	fmt.Println()
	fmt.Println("  a default.pgo next to main enables profile-guided optimisation (Go 1.21):")
	fmt.Println("  typically 2-7% faster for no code changes")

	// GODEBUG, read at run time.
	fmt.Println()
	fmt.Println("  GODEBUG restores an old behaviour when a release changes one:")
	fmt.Println("    GODEBUG=httplaxcontentlength=1, gotypesalias=0, asynctimerchan=1, ...")
	fmt.Println("  the `go` line in go.mod sets the defaults, which is what makes the")
	fmt.Println("  Go 1 compatibility promise workable in practice")
}

// =================================================================================================
// Section 5: Formatting, Vetting and Linting
// =================================================================================================

/*
## Formatting, Vetting and Linting

### gofmt

There is **one** Go style, and `gofmt` enforces it. It is not configurable — no options for indent
width, brace placement or line length — and that is the feature: no project has a style debate, and
every Go file in the world looks the same. Tabs for indentation, spaces for alignment.

`gofmt -l .` lists unformatted files (use this in CI), `gofmt -w .` rewrites them, `gofmt -d .`
shows a diff. **`goimports`** additionally adds and removes imports and is what most editors run on
save. `gofmt -r 'a[b:len(a)] -> a[b:]'` applies a rewrite rule across a codebase.

### go vet

`go vet` reports things that compile but are almost certainly wrong. It runs automatically as part
of `go test`. The checks that earn their keep:

  - **printf** — verb/argument mismatches, including in your own `...f` wrappers
  - **copylocks** — a `sync.Mutex` copied by value
  - **lostcancel** — a `context.CancelFunc` not called on every path
  - **loopclosure** — capturing a loop variable (now mostly moot, post-1.22)
  - **structtag** — a malformed struct tag
  - **nilfunc**, **unmarshal**, **unusedresult**

`shadow` and `fieldalignment` are *not* part of `go vet`'s built-in suite: they ship in
`golang.org/x/tools` and are run through `go vet -vettool=$(which shadow)`.

Every one of those found a real mistake while this training package was being written.

### Linters

`staticcheck` is the highest-value third-party tool: it finds dead code, useless assignments,
misused standard-library APIs and simplifiable expressions. `golangci-lint` bundles dozens of
linters behind one config and one pass, and is the usual CI choice. `errcheck` insists that every
returned error is handled.

Start with `gofmt` + `go vet` + `staticcheck`; add more only when a real bug motivates it.
*/

func m015FormattingAndLinting() {
	fmt.Println("\n--- Section 5: Formatting, Vetting and Linting ---")

	fmt.Println("  gofmt is NOT configurable, and that is the feature:")
	fmt.Println("    no indent-width option, no brace-placement option, no line-length option")
	fmt.Println("    tabs indent, spaces align, and every Go file in the world looks the same")
	fmt.Println()
	fmt.Println("  gofmt -l .   list unformatted files   <- put this in CI")
	fmt.Println("  gofmt -w .   rewrite in place")
	fmt.Println("  goimports    also adds and removes imports (what your editor runs on save)")
	fmt.Println()
	fmt.Println("  go vet runs automatically as part of `go test`. The checks that earn their keep:")
	for _, c := range []struct{ name, what string }{
		{"printf", "verb/argument mismatches, including in your own ...f wrappers"},
		{"copylocks", "a sync.Mutex copied by value"},
		{"lostcancel", "a context.CancelFunc not called on every path"},
		{"structtag", "a malformed struct tag"},
		{"unusedresult", "discarding the result of append, fmt.Sprintf, errors.New"},
		{"loopclosure", "capturing a loop variable (mostly moot since Go 1.22)"},
	} {
		fmt.Printf("    %-13s %s\n", c.name, c.what)
	}
	fmt.Println("  every one of those caught a real mistake while this package was written")
	fmt.Println()
	fmt.Println("  staticcheck is the highest-value third-party tool;")
	fmt.Println("  golangci-lint bundles dozens of linters behind one pass, for CI")
	fmt.Println("  start with gofmt + go vet + staticcheck and add more only when a bug demands it")
}

// =================================================================================================
// Section 6: Profiling and Diagnostics
// =================================================================================================

/*
## Profiling and Diagnostics

Go's runtime is unusually observable, and every tool below ships with the toolchain.

### pprof

	go test -cpuprofile cpu.out -memprofile mem.out -bench .
	go tool pprof -http=:8080 cpu.out

Or, in a running server, `import _ "net/http/pprof"` and hit `/debug/pprof/`. The profiles are
`profile` (CPU), `heap`, `goroutine`, `allocs`, `block`, `mutex`, `threadcreate`. The **goroutine**
profile is the fastest way to find a leak: it shows every goroutine and exactly where it is blocked.

### Execution tracing

	go test -trace trace.out ./...
	go tool trace trace.out

The trace shows scheduling, GC, syscalls and goroutine lifetimes on a timeline. **Flight recording**
(`runtime/trace.NewFlightRecorder`, Go 1.25) keeps a rolling in-memory window and lets you dump the
last few seconds *when something goes wrong* — which is what you want for a rare production stall.

### Runtime knobs

  - **`GOMAXPROCS`** — OS threads running Go code. Since **Go 1.25** it is **container-aware**: the
    runtime reads the cgroup CPU limit, so a pod limited to 2 CPUs on a 64-core node no longer sets
    it to 64. This alone fixed a great deal of accidental over-parallelism.
  - **`GOGC`** — heap growth target (default 100 = collect when the heap doubles).
  - **`GOMEMLIMIT`** (Go 1.19) — a soft memory ceiling. Setting `GOGC=off` with a `GOMEMLIMIT` is a
    good container configuration.
  - **`GODEBUG=gctrace=1`** — one line per GC cycle, on stderr.
  - **`GOTRACEBACK=all`** — dump every goroutine's stack on a crash.

### Other

`runtime/metrics` is the modern, stable metrics interface (prefer it to `runtime.ReadMemStats`).
`runtime/debug` gives `ReadBuildInfo`, `SetGCPercent`, `SetMemoryLimit` and `PrintStack`.
*/

func m015Profiling() {
	fmt.Println("\n--- Section 6: Profiling and Diagnostics ---")

	fmt.Println("  go test -cpuprofile cpu.out -memprofile mem.out -bench . ./basics/")
	fmt.Println("  go tool pprof -http=:8080 cpu.out")
	fmt.Println("  import _ \"net/http/pprof\"   <- /debug/pprof/ in a running server")
	fmt.Println("  the goroutine profile is the fastest way to find a leak:")
	fmt.Println("  it shows every goroutine and exactly where it is blocked")
	fmt.Println()
	fmt.Println("  go test -trace trace.out ./... && go tool trace trace.out")
	fmt.Println("  runtime/trace.NewFlightRecorder (Go 1.25) keeps a rolling window,")
	fmt.Println("  so you can dump the last few seconds WHEN something goes wrong")
	fmt.Println()

	// Live runtime numbers.
	fmt.Printf("  GOMAXPROCS=%d  NumCPU=%d  NumGoroutine=%d\n",
		runtime.GOMAXPROCS(0), runtime.NumCPU(), runtime.NumGoroutine())
	fmt.Printf("  GOGC default is 100 (collect when the heap doubles); current SetGCPercent=%d\n",
		m015CurrentGCPercent())
	fmt.Println("  since Go 1.25 GOMAXPROCS is CONTAINER-AWARE: it reads the cgroup CPU limit,")
	fmt.Println("  so a pod limited to 2 CPUs on a 64-core node no longer gets 64")
	fmt.Println()
	fmt.Println("  GOMEMLIMIT (Go 1.19) sets a soft ceiling; GOGC=off plus GOMEMLIMIT is a")
	fmt.Println("  good container configuration")
	fmt.Println("  GODEBUG=gctrace=1   one line per GC cycle")
	fmt.Println("  GOTRACEBACK=all     every goroutine's stack on a crash")
	fmt.Println()
	fmt.Println("  prefer runtime/metrics to runtime.ReadMemStats for anything long-lived")
}

func m015CurrentGCPercent() int {
	current := debug.SetGCPercent(100) // returns the previous value
	debug.SetGCPercent(current)        // put it back immediately
	return current
}

// =================================================================================================
// Section 7: Project Layout and Conventions
// =================================================================================================

/*
## Project Layout and Conventions

There is **no official project layout**. The widely-cited `golang-standards/project-layout` is not
endorsed by the Go team, and for most projects it is far more structure than is warranted.

What the toolchain actually enforces is only: one package per directory, `internal/` visibility,
`_test.go` suffixes, and filename build constraints. Everything else is convention.

The conventions that are genuinely worth following:

	cmd/<name>/main.go   one directory per binary, when there are several
	internal/            code this module alone may import - enforced by the compiler
	pkg/                 a convention some projects use for public library code; often unnecessary
	testdata/            ignored by the go tool; where fixtures and fuzz corpora live
	docs/, api/          plain documentation and schemas

- **Start flat.** A small module with `main.go` and a few packages beside it is completely idiomatic
  — which is exactly the shape of this repository. Add directories when a package genuinely has a
  separate reason to change.
- **Name packages for what they provide, not what they contain.** `http`, not `httputils`. Avoid
  `util`, `common`, `helpers`, `base`, `misc` — a package that could hold anything ends up holding
  everything, and it becomes an import-cycle magnet. (`examples/common/` in this repository is a mild example
  of exactly that.)
- **Package names are lower case, one word, no underscores, no plurals.** The name is repeated at
  every call site, so `strconv.Quote` beats `string_conversion_utils.Quote`.
- **Avoid stutter**: in package `user`, the type is `user.User` at worst — never `user.UserModel`.
  `bytes.Buffer`, not `bytes.BytesBuffer`.
- **Interfaces belong with the consumer**, not the producer (module 008). That single rule removes
  most of the "where do I put this interface" question.
- Keep `main` tiny: parse flags, wire dependencies, call into a package, handle the error. Anything
  in `main` cannot be imported or tested from elsewhere.
*/

func m015ProjectLayout() {
	fmt.Println("\n--- Section 7: Project Layout and Conventions ---")

	fmt.Println("  there is NO official project layout - golang-standards/project-layout is")
	fmt.Println("  not endorsed by the Go team and is usually far more structure than warranted")
	fmt.Println()
	fmt.Println("  the toolchain enforces only: one package per directory, internal/ visibility,")
	fmt.Println("  the _test.go suffix, and filename build constraints. The rest is convention.")
	fmt.Println()
	fmt.Println("  cmd/<name>/main.go   one directory per binary, when there are several")
	fmt.Println("  internal/            importable only from this module - compiler-enforced")
	fmt.Println("  testdata/            ignored by the go tool; fixtures and fuzz corpora")
	fmt.Println()
	fmt.Println("  START FLAT. This repository is a good example: main.go plus a few packages.")
	fmt.Println()
	fmt.Println("  name packages for what they PROVIDE, not what they contain:")
	fmt.Println("    good: http, strconv, slices, bytes")
	fmt.Println("    bad:  util, common, helpers, base, misc")
	fmt.Println("  a package that could hold anything ends up holding everything, and becomes")
	fmt.Println("  an import-cycle magnet - `examples/common/` in this repo is a mild example")
	fmt.Println()
	fmt.Println("  avoid stutter: bytes.Buffer, not bytes.BytesBuffer; user.User at worst")
	fmt.Println("  put interfaces with the CONSUMER, not the producer (module 008)")
	fmt.Println("  keep main tiny: flags, wiring, one call, error handling")
}

// Run015 runs every section of module 015 in order.
func Run015() {
	m015Modules()
	m015Dependencies()
	m015BuildAndEmbed()
	m015GoCommand()
	m015FormattingAndLinting()
	m015Profiling()
	m015ProjectLayout()
}

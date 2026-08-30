package basics

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

/*
# Module 021 — Publishing Libraries and CLI Tools

The last module of the course. Modules 017–020 built an application; this one is about giving it to
other people — as an importable library, or as a binary they can install.

Go's distribution model is unusual and worth understanding before you publish anything: **there is
no registry and no publish command**. A module is identified by its own URL, and "publishing" is
`git tag` followed by `git push`. The proxy discovers it from there. Nobody can take your name,
nobody can delete your release, and — the flip side — nobody can fix your mistake either.
*/

// =================================================================================================
// Section 1: What Makes a Module Publishable
// =================================================================================================

/*
## What Makes a Module Publishable

- **The module path is a URL.** `module github.com/you/thing` means the code lives at that address,
  and that is how `go get` finds it. There is no separate registration step, and no central
  namespace to claim.
- Consequences worth internalising:
    - the path must be **fetchable** — a public repository, or a private one plus `GOPRIVATE`
    - **renaming the repository breaks every importer**, because the path *is* the identity
    - a **vanity path** (`go.uber.org/zap`) works by serving an HTML `<meta name="go-import">` tag
      that points at the real repository; that is the only indirection the toolchain offers
- **The repository root need not be the module root**, but it is much simpler when it is. A module
  in a subdirectory is imported as `github.com/you/repo/sub` and tagged `sub/v1.2.3`.
- Practical requirements before the first tag:
    - `go.mod` with the final module path — changing it later is a breaking change
    - a **LICENSE** file. Without one, the default is "all rights reserved", and pkg.go.dev will
      display a warning that discourages use.
    - a **README** — pkg.go.dev shows it, and it is the first thing a potential user reads
    - `go.sum` committed, so builds are verifiable
- **`internal/` is your friend.** Anything under `internal/` is unimportable from outside your
  module (module 015, Section 1) and is therefore *not* part of your API. Start with almost
  everything internal and promote deliberately: you can always export more, never less.
*/

func m021WhatIsPublishable() {
	fmt.Println("--- Section 1: What Makes a Module Publishable ---")

	if info, ok := debug.ReadBuildInfo(); ok {
		fmt.Printf("  this module's path: %s\n", info.Main.Path)
		fmt.Printf("  its version:        %s\n", m021OrDefault(info.Main.Version, "(devel)"))
	}
	fmt.Println("  `training.pl/go` is not fetchable, so this course could not be published as is -")
	fmt.Println("  a real module path must resolve to a repository")

	fmt.Println()
	fmt.Println("  there is NO registry and NO publish command:")
	fmt.Println("    the module path IS the URL, and `git tag` IS the release")
	fmt.Println("  so: renaming the repository breaks every importer")
	fmt.Println("      a vanity path (go.uber.org/zap) works via a <meta name=\"go-import\"> tag")
	fmt.Println()
	fmt.Println("  before the first tag: final module path, LICENSE, README, committed go.sum")
	fmt.Println("  start with almost everything under internal/ - you can export more later,")
	fmt.Println("  but you can never un-export without breaking someone")
}

func m021OrDefault(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

// =================================================================================================
// Section 2: Designing an API You Can Live With
// =================================================================================================

/*
## Designing an API You Can Live With

Once someone imports your package, every exported identifier is a promise. The design rules that
matter most are the ones this course has already taught, applied with the knowledge that you cannot
take them back.

  - **Minimise the surface.** Every exported name is API. A package with four exported functions
    can be reworked; one with forty cannot. `internal/` is how you keep the rest private.
  - **Accept interfaces, return structs** (module 008). Accepting `io.Reader` lets callers pass
    anything; returning a concrete struct lets you add methods later without breaking them.
    Returning an *interface* freezes its method set forever.
  - **Make the zero value useful** (module 001a, Section 4). `var b bytes.Buffer` works;
    `sync.Mutex` works. A type that needs a constructor is a type every caller must remember to
    construct correctly.
  - **Use functional options for anything optional** (module 005, Section 5). Adding a parameter to
    a function is a breaking change; adding an `Option` is not. This is the single most valuable
    forward-compatibility technique in Go.
  - **Return concrete error types or documented sentinels** (module 009), and say in the doc comment
    which errors a function can return. Callers will write `errors.Is` against them, and that makes
    them API too.
  - **Take a `context.Context` as the first parameter** in anything that blocks (module 011,
    Section 5). Adding it later is a breaking change.
  - **Add a `_ struct{}` field** to a struct you may want to extend, or keep it uncomparable
    (module 007, Section 6), so that callers cannot depend on positional literals or on `==`.
  - **Do not export a package-level mutable variable.** It is global state you can never reclaim.

The forward-compatible shapes are worth memorising, because the difference is invisible until the
day you need it:

	func New(host string, port int, timeout time.Duration) *Client   // adding a parameter breaks
	func New(host string, opts ...Option) *Client                    // adding an option does not

	func Parse(s string) (*Result, error)                            // can gain methods
	func Parse(s string) (Resulter, error)                           // method set frozen forever
*/

// m021Client shows the forward-compatible shape: required arguments positional, everything else
// an Option, and a useful zero value behind the constructor.
type m021Client struct {
	host    string
	port    int
	retries int
	verbose bool
}

type m021Option func(*m021Client)

func m021WithPort(p int) m021Option    { return func(c *m021Client) { c.port = p } }
func m021WithRetries(n int) m021Option { return func(c *m021Client) { c.retries = n } }
func m021WithVerbose() m021Option      { return func(c *m021Client) { c.verbose = true } }

// m021NewClient returns a *struct*, not an interface, so methods can be added without breaking
// anyone who already uses it.
func m021NewClient(host string, opts ...m021Option) *m021Client {
	c := &m021Client{host: host, port: 443, retries: 3} // defaults live here
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *m021Client) String() string {
	return fmt.Sprintf("host=%s port=%d retries=%d verbose=%t", c.host, c.port, c.retries, c.verbose)
}

func m021APIDesign() {
	fmt.Println("\n--- Section 2: Designing an API You Can Live With ---")

	fmt.Printf("  defaults:          %v\n", m021NewClient("api.example.com"))
	fmt.Printf("  with options:      %v\n",
		m021NewClient("api.example.com", m021WithPort(8443), m021WithRetries(5), m021WithVerbose()))
	fmt.Println("  adding a fourth option later breaks NOBODY; adding a fourth parameter")
	fmt.Println("  to New(host, port, timeout) breaks EVERY caller")

	fmt.Println()
	fmt.Println("  the rules, all of which this course already taught:")
	fmt.Println("    minimise the exported surface        every exported name is a promise")
	fmt.Println("    accept interfaces, return structs    a returned interface freezes its methods")
	fmt.Println("    make the zero value useful           var b bytes.Buffer just works")
	fmt.Println("    functional options for the optional  the only additive way to grow a signature")
	fmt.Println("    document which errors you return     callers will errors.Is against them")
	fmt.Println("    ctx as the first parameter           adding it later is breaking")
	fmt.Println("    no exported mutable package vars     global state you can never reclaim")
}

// =================================================================================================
// Section 3: Documentation Is Part of the API
// =================================================================================================

/*
## Documentation Is Part of the API

pkg.go.dev renders your doc comments automatically, and for most users that page **is** your
library. There is nothing to configure — the same rules from module 001a, Section 1 apply:

  - a comment immediately above a declaration, starting with the identifier's name
  - a **package comment** above `package x`, conventionally in a `doc.go`. This is the page's
    opening paragraph and deserves real effort: say what the package is for, and show the smallest
    complete usage.
  - since Go 1.19: `# Headings`, lists, indented code blocks, and `[Name]` / `[pkg.Name]` links
  - a `Deprecated:` paragraph marks an identifier deprecated; editors and linters surface it

**`Example` functions are the highest-value documentation you can write** (module 014, Section 5),
because they are compiled and *run* by `go test`:

  - they appear on pkg.go.dev next to the identifier they name, with a Run button
  - the `// Output:` comment is verified, so the documentation **cannot go stale**
  - written in the external test package (`package x_test`), they can only use the exported API,
    which means they also *test your API's ergonomics* — if the example is awkward, so is the API

Read your own docs before tagging: **`go doc -http=:6060`** (Go 1.25) serves the whole thing locally,
exactly as pkg.go.dev will render it. It costs a minute and catches the paragraph that made sense
only to you.
*/

func m021Documentation() {
	fmt.Println("\n--- Section 3: Documentation Is Part of the API ---")

	fmt.Println("  pkg.go.dev renders doc comments automatically - for most users that page")
	fmt.Println("  IS your library, so the package comment is the most-read text you will write")
	fmt.Println()
	fmt.Println("  go doc training.pl/go/basics          read it in the terminal")
	fmt.Println("  go doc -http=:6060                    Go 1.25: the full site, locally, offline")
	fmt.Println("  read your own docs BEFORE tagging - it catches the paragraph that made sense")
	fmt.Println("  only to you")
	fmt.Println()
	fmt.Println("  Example functions are the best documentation you can write:")
	fmt.Println("    they run under `go test`, so the // Output: comment cannot go stale")
	fmt.Println("    they appear on pkg.go.dev with a Run button")
	fmt.Println("    in package x_test they can only use the EXPORTED API - so a clumsy example")
	fmt.Println("    is telling you the API itself is clumsy")
	fmt.Println("    this course has three: Example_reverse, Example_parseKV,")
	fmt.Println("    Example_parseKVUnordered in mod_014_testing_test.go")
	fmt.Println()
	fmt.Println("  mark retirement with a Deprecated: paragraph - editors and linters surface it:")
	fmt.Println("    // Deprecated: use NewClient instead. Dial will be removed in v3.")
}

// =================================================================================================
// Section 4: Semantic Versioning and the /v2 Rule
// =================================================================================================

/*
## Semantic Versioning and the /v2 Rule

Go enforces semantic versioning at the level of the *import path*, which is unusual and is the
single most confusing thing about publishing Go modules.

	v0.x.y   anything may change; the compatibility promise does not apply yet
	v1.x.y   stable: patch = fixes, minor = additive only, no breaking changes ever
	v2.0.0+  a breaking change — and the import path MUST change

**The /v2 rule**: from v2 onward, the major version becomes part of the module path.

	module github.com/you/thing/v2      // in go.mod
	import "github.com/you/thing/v2"    // in every importer

The tag is still `v2.0.0`. This is *semantic import versioning*, and the reason for it is that it
lets **v1 and v2 coexist in one build** — a large dependency graph can migrate one package at a
time instead of all at once. It is a real benefit, paid for with a genuinely awkward rule.

Two ways to lay out v2, both legitimate:

  - **the major-subdirectory approach** — copy the code into `v2/` and tag `v2.0.0`. Both versions
    stay buildable from one branch.
  - **the branch approach** — a `v2` branch with the path updated in `go.mod`, tagged there.

`v0` deserves a mention: it is the honest place to be while an API is still moving, and the
toolchain treats **every v0 minor bump as potentially breaking**. Do not rush to v1 — but once you
tag v1.0.0 you have promised not to break anything, and Go's tooling will hold you to it.

**What counts as breaking** is broader than people expect: removing or renaming anything exported,
changing a signature, adding a method to an *interface* you export, changing a struct so that
positional literals break, and changing documented behaviour. Adding a *field* to a struct is
breaking too if callers used a positional literal — which is why `_ struct{}` fields exist.
*/

func m021Versioning() {
	fmt.Println("\n--- Section 4: Semantic Versioning and the /v2 Rule ---")

	fmt.Println("  v0.x.y   anything may change; every minor bump is potentially breaking")
	fmt.Println("  v1.x.y   stable: patch = fixes, minor = additive only")
	fmt.Println("  v2.0.0+  breaking - and the IMPORT PATH must change")
	fmt.Println()
	fmt.Println("  the /v2 rule:")
	fmt.Println("    module github.com/you/thing/v2      <- go.mod")
	fmt.Println("    import \"github.com/you/thing/v2\"    <- every importer")
	fmt.Println("    git tag v2.0.0                      <- the tag keeps the plain form")
	fmt.Println("  the point: v1 and v2 can coexist in ONE build, so a big dependency graph")
	fmt.Println("  migrates one package at a time")
	fmt.Println()
	fmt.Println("  the standard library follows its own rule: math/rand/v2, encoding/json/v2")

	fmt.Println()
	fmt.Println("  what counts as BREAKING is broader than people expect:")
	for _, change := range []struct{ what, breaking string }{
		{"removing or renaming an exported name", "yes"},
		{"changing a function signature", "yes"},
		{"adding a method to an exported INTERFACE", "yes - implementers break"},
		{"adding a method to an exported STRUCT", "no"},
		{"adding a field to an exported struct", "yes, if callers use positional literals"},
		{"adding a new exported function", "no"},
		{"changing documented behaviour", "yes, even with the same signature"},
		{"adding a variadic Option parameter", "no - that is the whole point"},
	} {
		fmt.Printf("    %-42s %s\n", change.what, change.breaking)
	}
}

// =================================================================================================
// Section 5: Publishing, the Proxy, and retract
// =================================================================================================

/*
## Publishing, the Proxy, and retract

Publishing is two commands:

	git tag v1.2.3
	git push origin v1.2.3

That is all. The first `go get github.com/you/thing@v1.2.3` makes the module proxy fetch and cache
it, and pkg.go.dev indexes it shortly after. `GOPROXY=proxy.golang.org` is the default, with
`direct` as a fallback for anything the proxy cannot reach.

### The consequence nobody warns you about: releases are immutable

The proxy **caches a version permanently**, and the checksum database (`sum.golang.org`) records
its hash forever. So:

  - **deleting or moving a tag does not un-publish it.** The proxy already has it, and a moved tag
    produces a checksum mismatch that breaks builds loudly — `SECURITY ERROR` in the output.
  - the only correct response to a bad release is **a new version**, plus `retract`
  - **a leaked secret in a tagged commit is public forever.** Rotate it; do not try to erase it.

### retract

	// go.mod, in a LATER release
	retract (
	    v1.2.3            // contains a data-loss bug
	    [v1.0.0, v1.1.0]  // accidentally published from the wrong branch
	)

`retract` tells the toolchain not to select those versions and to warn anyone who has them pinned.
It must appear in a **newer** release than the ones it retracts — retracting v1.2.3 requires
publishing v1.2.4. Retracting a version does not remove it; nothing removes it.

### Private modules

`GOPRIVATE=github.com/mycorp/*` turns off the proxy and the checksum database for matching paths, so
`go get` fetches straight from the VCS. `GONOPROXY` and `GONOSUMDB` split the two behaviours when
you need them separately.
*/

func m021Publishing() {
	fmt.Println("\n--- Section 5: Publishing, the Proxy, and retract ---")

	fmt.Println("  publishing is two commands:")
	fmt.Println("    git tag v1.2.3")
	fmt.Println("    git push origin v1.2.3")
	fmt.Println("  the proxy fetches on first use; pkg.go.dev indexes shortly after")
	fmt.Println()
	fmt.Println("  RELEASES ARE IMMUTABLE:")
	fmt.Println("    the proxy caches a version permanently and sum.golang.org records its hash")
	fmt.Println("    deleting a tag does NOT un-publish it")
	fmt.Println("    MOVING a tag produces a checksum mismatch and breaks builds loudly")
	fmt.Println("    a secret in a tagged commit is public forever - rotate it, do not erase it")
	fmt.Println()
	fmt.Println("  the only correct fix for a bad release is a NEW version plus retract:")
	fmt.Println("    // in v1.2.4's go.mod")
	fmt.Println("    retract (")
	fmt.Println("        v1.2.3            // contains a data-loss bug")
	fmt.Println("        [v1.0.0, v1.1.0]  // published from the wrong branch")
	fmt.Println("    )")
	fmt.Println("  retract must live in a NEWER release than what it retracts")
	fmt.Println()
	fmt.Println("  private modules: GOPRIVATE=github.com/mycorp/*  (skips proxy AND sumdb)")
	fmt.Println("                   GONOPROXY / GONOSUMDB split the two when you need them apart")
	fmt.Println()
	fmt.Println("  verify what you are about to depend on:")
	fmt.Println("    go mod verify            checksums match what is recorded")
	fmt.Println("    go list -m -versions M   every published version")
	fmt.Println("    go mod why -m M          why this dependency is in the graph")
}

// =================================================================================================
// Section 6: Shipping a CLI Tool
// =================================================================================================

/*
## Shipping a CLI Tool

A Go binary is **statically linked and has no runtime dependency** — no interpreter, no shared
libraries, no installed VM. That is the single biggest practical advantage Go has for command-line
tools, and it shapes how you ship them.

### Layout

	cmd/mytool/main.go      the binary; keep it tiny (module 017, Section 7)
	internal/...            everything that is not API
	*.go                    the importable library, if there is one

`go install github.com/you/thing/cmd/mytool@latest` installs from source into `$GOBIN` — no
package manager, no root, no install script. It is why so many Go tools document exactly one
installation command.

### Cross-compiling

	GOOS=linux   GOARCH=amd64 go build -o dist/mytool-linux-amd64   ./cmd/mytool
	GOOS=darwin  GOARCH=arm64 go build -o dist/mytool-darwin-arm64  ./cmd/mytool
	GOOS=windows GOARCH=amd64 go build -o dist/mytool.exe           ./cmd/mytool

No toolchain to install for each target; the standard library is compiled on demand.
`CGO_ENABLED=0` guarantees a fully static binary — necessary for `FROM scratch` containers and for
running on a distribution with a different libc. `go tool dist list` prints every supported pair.

### Stamping the version

	go build -ldflags="-s -w -X main.version=1.2.3" ./cmd/mytool

`-X` sets a **string variable** at link time; `-s -w` strip the symbol table and DWARF data, usually
cutting a quarter off the binary.

But for a module built with `go install`, you often do not need `-X` at all:
**`debug.ReadBuildInfo()`** already carries the version, the VCS revision and whether the tree was
dirty — stamped automatically since Go 1.18. Read it instead of maintaining a variable.

### Release automation

**GoReleaser** is the de-facto standard: cross-compiles every target, builds archives, checksums,
Homebrew formulae, Docker images and a GitHub release from one `.goreleaser.yaml`. For a tool
anyone else uses, it is worth the afternoon.
*/

func m021ShippingCLI() {
	fmt.Println("\n--- Section 6: Shipping a CLI Tool ---")

	fmt.Println("  a Go binary is statically linked with no runtime dependency - that is the")
	fmt.Println("  biggest practical advantage Go has for command-line tools")
	fmt.Println()
	fmt.Println("  layout:  cmd/mytool/main.go   the binary, kept tiny (module 017 §7)")
	fmt.Println("           internal/...         everything that is not API")
	fmt.Println("  install: go install github.com/you/thing/cmd/mytool@latest")
	fmt.Println()
	fmt.Printf("  cross-compiling from %s/%s needs no extra toolchain:\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println("    GOOS=linux   GOARCH=amd64 go build -o dist/mytool-linux-amd64  ./cmd/mytool")
	fmt.Println("    GOOS=darwin  GOARCH=arm64 go build -o dist/mytool-darwin-arm64 ./cmd/mytool")
	fmt.Println("    GOOS=windows GOARCH=amd64 go build -o dist/mytool.exe          ./cmd/mytool")
	fmt.Println("    CGO_ENABLED=0 for a fully static binary (FROM scratch images)")
	fmt.Println("    go tool dist list prints every supported GOOS/GOARCH pair")
	fmt.Println()
	fmt.Println("  stamping a version:")
	fmt.Println("    go build -ldflags=\"-s -w -X main.version=1.2.3\" ./cmd/mytool")
	fmt.Println("    -X sets a string variable at link time; -s -w strip ~25% off the binary")

	// --- What the toolchain already recorded, with no -X at all ---
	fmt.Println()
	fmt.Println("  ...but debug.ReadBuildInfo() already knows, with no -X needed:")
	if info, ok := debug.ReadBuildInfo(); ok {
		fmt.Printf("    module   %s %s\n", info.Main.Path, m021OrDefault(info.Main.Version, "(devel)"))
		fmt.Printf("    built by %s\n", info.GoVersion)
		interesting := map[string]bool{
			"vcs": true, "vcs.revision": true, "vcs.time": true, "vcs.modified": true,
			"GOOS": true, "GOARCH": true, "-ldflags": true, "CGO_ENABLED": true,
		}
		for _, s := range info.Settings {
			if interesting[s.Key] {
				value := s.Value
				if s.Key == "vcs.revision" && len(value) > 12 {
					value = value[:12]
				}
				fmt.Printf("    %-14s %s\n", s.Key, value)
			}
		}
	}
	fmt.Println("  (this binary was built by `go run`, so the VCS stamps are omitted -")
	fmt.Println("   `go build` records the revision and whether the tree was dirty)")

	fmt.Println()
	fmt.Println("  GoReleaser automates the rest: every target, archives, checksums, Homebrew")
	fmt.Println("  formulae, Docker images and a GitHub release from one .goreleaser.yaml")
}

// =================================================================================================
// Section 7: A Release Checklist
// =================================================================================================

/*
## A Release Checklist

Everything this course has covered, in the order you would actually apply it before tagging a
release. Nothing here is exotic; the value is in doing it every time.

	CORRECTNESS
	  gofmt -l .                      formatted                        (015 §5)
	  go vet ./...                    no reported mistakes             (015 §5)
	  staticcheck ./...               no reported mistakes             (015 §5)
	  go test ./...                   green                            (014)
	  go test -race ./...             green under the race detector    (011 §7, 014)
	  go test -count=1 ./...          green with the cache defeated    (014 §7)

	API
	  go doc -http=:6060              read your own documentation      (021 §3)
	  every exported name documented, starting with its own name       (001a §1)
	  Example functions for the main entry points                      (014 §5)
	  nothing exported that could be internal/                         (015 §1)
	  optional parameters are Options, not positional                  (005 §5, 021 §2)

	MODULE
	  go mod tidy                     no unused or missing requires    (015 §2)
	  go.sum committed                                                  (015 §1)
	  LICENSE and README present                                        (021 §1)
	  module path final, /vN if major >= 2                              (021 §4)
	  retract any earlier bad release                                   (021 §5)

	RELEASE
	  git tag vX.Y.Z && git push origin vX.Y.Z                          (021 §5)
	  go install <path>@vX.Y.Z        verify it installs cleanly        (021 §6)

And the two rules that outlast every checklist:

 1. **Releases are immutable.** Think before you tag; fix forward, never sideways.
 2. **Every exported identifier is a promise.** The smallest API you can get away with is the one
    you will be happiest with in two years.
*/

func m021ReleaseChecklist() {
	fmt.Println("\n--- Section 7: A Release Checklist ---")

	groups := []struct {
		name  string
		items [][2]string
	}{
		{"CORRECTNESS", [][2]string{
			{"gofmt -l .", "formatted (015 §5)"},
			{"go vet ./...", "no reported mistakes (015 §5)"},
			{"staticcheck ./...", "no reported mistakes (015 §5)"},
			{"go test ./...", "green (014)"},
			{"go test -race ./...", "green under the race detector (011 §7)"},
			{"go test -count=1 ./...", "green with the cache defeated (014 §7)"},
		}},
		{"API", [][2]string{
			{"go doc -http=:6060", "read your own documentation (021 §3)"},
			{"every exported name documented", "starting with its own name (001a §1)"},
			{"Example functions", "for the main entry points (014 §5)"},
			{"nothing exported that could be internal/", "(015 §1)"},
			{"optional parameters are Options", "not positional (005 §5)"},
		}},
		{"MODULE", [][2]string{
			{"go mod tidy", "no unused or missing requires (015 §2)"},
			{"go.sum committed", "builds are verifiable (015 §1)"},
			{"LICENSE and README present", "(021 §1)"},
			{"module path final, /vN if major >= 2", "(021 §4)"},
			{"retract any earlier bad release", "(021 §5)"},
		}},
		{"RELEASE", [][2]string{
			{"git tag vX.Y.Z && git push origin vX.Y.Z", "(021 §5)"},
			{"go install <path>@vX.Y.Z", "verify it installs cleanly (021 §6)"},
		}},
	}

	for _, g := range groups {
		fmt.Printf("  %s\n", g.name)
		for _, item := range g.items {
			fmt.Printf("    %-42s %s\n", item[0], item[1])
		}
	}

	fmt.Println()
	fmt.Println("  and the two rules that outlast every checklist:")
	fmt.Println("    1. releases are immutable - think before you tag, and fix FORWARD")
	fmt.Println("    2. every exported identifier is a promise - the smallest API you can get")
	fmt.Println("       away with is the one you will be happiest with in two years")

	fmt.Println()
	fmt.Println(strings.Repeat("-", 78))
	fmt.Println("  That is the end of the course: 23 modules, from `package main` to a tagged")
	fmt.Println("  release. Run `go run .` for the index, and see notes.md for the exercises.")
}

// Run021 runs every section of module 021 in order.
func Run021() {
	m021WhatIsPublishable()
	m021APIDesign()
	m021Documentation()
	m021Versioning()
	m021Publishing()
	m021ShippingCLI()
	m021ReleaseChecklist()
}

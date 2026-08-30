package basics

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"math/rand/v2"
	"os"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
	"unique"
	"uuid"
	"weak"
)

/*
# Module 016 — What Changed, Go 1.21 to Go 1.27

Go's compatibility promise means nothing you learned stops working — but a great deal of what you
*should* write has changed since 1.20. If you learned Go before then, this module is the delta.

Each section is one release. The **language** changes are the ones that matter most, because they
change what idiomatic code looks like; the library additions mostly replace things you used to
hand-write.

Every claim here was checked against the toolchain this module is compiled with; the code runs.
*/

// =================================================================================================
// Section 1: Go 1.21 — min, max, clear, and the collection packages
// =================================================================================================

/*
## Go 1.21

### Language

  - **`min`, `max` and `clear` are builtins.** `min`/`max` are variadic and work on any ordered
    type, including strings. `clear` zeroes a slice or empties a map.
  - **Package initialisation order** was made precisely specified rather than merely conventional.
  - **Type inference improved**: a generic function can now be assigned to a variable of function
    type, and methods can be used as values, with the type arguments inferred.

### Library

  - **`slices`, `maps`, `cmp`** graduated from `golang.org/x/exp` into the standard library — the
    biggest single addition to everyday Go in years (module 012).
  - **`log/slog`**: structured logging, in the standard library at last (module 013).
  - **`sync.OnceFunc`, `OnceValue`, `OnceValues`**: generic lazy initialisation.
  - **`errors.ErrUnsupported`**: a standard sentinel for "not supported here".
  - `context.WithoutCancel`, `context.AfterFunc`, `context.WithDeadlineCause`.

### Toolchain

  - **Profile-guided optimisation (PGO)** became generally available: drop a `default.pgo` beside
    `main` and get 2-7% for free.
  - The **`toolchain` directive**: a module can require a newer Go, and the `go` command downloads
    it automatically.

**What to stop writing**: your own `Min`/`Max`, `sort.Slice`, `golang.org/x/exp/slices`,
`golang.org/x/exp/constraints.Ordered`, and a `sync.Once` plus a package-level variable.
*/

func m016Go121() {
	fmt.Println("--- Section 1: Go 1.21 ---")

	// min, max and clear as builtins.
	fmt.Printf("  builtins: min(3,1,2)=%d max(3,1,2)=%d min(\"pear\",\"apple\")=%q\n",
		min(3, 1, 2), max(3, 1, 2), min("pear", "apple"))

	nums := []int{1, 2, 3}
	clear(nums)
	m := map[string]int{"a": 1, "b": 2}
	clear(m)
	fmt.Printf("  clear: slice -> %v (zeroed, length kept), map -> %v (emptied)\n", nums, m)

	// slices, maps and cmp - the everyday workhorses now.
	s := []int{3, 1, 2}
	slices.Sort(s)
	fmt.Printf("  slices.Sort: %v   (replaces sort.Slice and its closure)\n", s)
	fmt.Printf("  cmp.Ordered replaces golang.org/x/exp/constraints.Ordered\n")

	// sync.OnceValue: lazy initialisation with no package-level variable.
	config := sync.OnceValue(func() string { return "loaded once, lazily" })
	fmt.Printf("  sync.OnceValue: %q / %q\n", config(), config())

	// slog.
	fmt.Println("  log/slog: structured logging in the standard library (module 013)")

	// errors.ErrUnsupported.
	err := fmt.Errorf("compress on this platform: %w", errors.ErrUnsupported)
	fmt.Printf("  errors.ErrUnsupported: Is()=%t\n", errors.Is(err, errors.ErrUnsupported))

	fmt.Println("  STOP WRITING: your own Min/Max, sort.Slice, x/exp/slices, x/exp/constraints")
}

// =================================================================================================
// Section 2: Go 1.22 — the loop variable fix and range over int
// =================================================================================================

/*
## Go 1.22

### Language — the most consequential change in a decade

  - **Loop variables are now per-iteration.** `for i := range n` creates a *new* `i` each time
    round, so a closure or goroutine capturing it sees that iteration's value. Before 1.22 every
    closure shared one variable and saw the final value — the single most common Go bug ever
    written, and the reason `i := i` appeared at the top of so many loop bodies.
  - This is gated on the **`go` line in `go.mod`**: a module declaring `go 1.21` or earlier keeps
    the old behaviour even under a new toolchain. That is how a language change ships without
    breaking anyone.
  - **`range` over an integer**: `for i := range 10` and `for range 3`. It removes most three-clause
    loops from real code.

### Library

  - **`math/rand/v2`**: the first `v2` package in the standard library. Better algorithms
    (PCG, ChaCha8), no global-lock bottleneck, `N[T]` for a bounded random of any integer type, and
    the mistake-prone `Read` and `Seed` removed.
  - `slices.Concat`, `cmp.Or`.
  - Enhanced **`net/http` routing patterns**: `mux.HandleFunc("GET /items/{id}", h)`, with methods
    and wildcards in the pattern, and `r.PathValue("id")` to read them. That removed the main
    reason to reach for a third-party router.

**What to stop writing**: `i := i` at the top of a loop body, `for i := 0; i < n; i++` when you
only need the count, `math/rand` in new code, and a router dependency for simple method-and-path
matching.
*/

func m016Go122() {
	fmt.Println("\n--- Section 2: Go 1.22 ---")

	// The loop variable fix.
	var closures []func() int
	for i := range 3 {
		closures = append(closures, func() int { return i })
	}
	got := make([]int, 0, 3)
	for _, c := range closures {
		got = append(got, c())
	}
	fmt.Printf("  closures over the loop variable: %v\n", got)
	fmt.Println("    since Go 1.22 this is [0 1 2]; before it was [3 3 3], and the fix was `i := i`")
	fmt.Println("    it is gated on the go.mod `go` line, so old modules keep the old behaviour")

	// range over an integer.
	fmt.Print("  range over an int: ")
	for i := range 5 {
		fmt.Print(i, " ")
	}
	fmt.Print("| repeat 3 times: ")
	for range 3 {
		fmt.Print("* ")
	}
	fmt.Println()

	// math/rand/v2.
	r := rand.New(rand.NewPCG(42, 1024)) // deterministic, for reproducible output here
	fmt.Printf("  math/rand/v2: IntN(100)=%d Float64()=%.4f N[int64](1000)=%d\n",
		r.IntN(100), r.Float64(), r.N(int64(1000)))
	fmt.Println("    v2 removed Seed and Read, uses PCG/ChaCha8, and has no global lock")

	// slices.Concat and cmp.Or.
	fmt.Printf("  slices.Concat: %v   cmp.Or(\"\", \"fallback\"): %q\n",
		slices.Concat([]int{1}, []int{2, 3}), cmp.Or("", "fallback"))

	fmt.Println("  net/http routing patterns: mux.HandleFunc(\"GET /items/{id}\", h)")
	fmt.Println("    plus r.PathValue(\"id\") - no third-party router needed for this")
}

// =================================================================================================
// Section 3: Go 1.23 — iterators
// =================================================================================================

/*
## Go 1.23

### Language

  - **`range` over a function.** `for x := range f` where `f` is `func(yield func(V) bool)`. This is
    the extension point the language had been missing: any sequence — a tree walk, a file's lines, a
    database cursor, an infinite series — can now be consumed by an ordinary `for` loop.

### Library

  - **`iter`**: `Seq[V]`, `Seq2[K, V]`, and `Pull`/`Pull2` to convert a push iterator into a pull
    one (module 012).
  - **Iterator functions everywhere**: `slices.All`, `Values`, `Backward`, `Collect`, `Sorted`,
    `SortedFunc`, `Chunk`, `Repeat`; `maps.Keys`, `Values`, `All`, `Collect`, `Insert`.
    Note that `maps.Keys` returns an **iterator**, not a slice — unlike the `x/exp` version it
    replaced, which caught a lot of people out.
  - **`unique`**: `Make[T]`/`Handle[T]` interning, so many equal values share one allocation and
    compare by pointer.
  - **Timer changes**: an unreferenced `time.Timer` or `Ticker` is now garbage-collected even if it
    has not fired, and its channel is unbuffered. This quietly fixed the long-standing `time.After`
    leak in hot loops (gated on the `go` line, with `GODEBUG=asynctimerchan=1` to revert — a knob
    that Go 1.27 has now removed).

### Toolchain

  - **Telemetry**, opt-in via `go telemetry on`.
  - `go build -cover` for coverage of whole programs, not just tests.

**What to stop writing**: callback-based iteration (`ForEach(func(v T) bool)`), a goroutine plus a
channel just to produce a sequence, and `maps.Keys` expecting a slice.
*/

func m016Go123() {
	fmt.Println("\n--- Section 3: Go 1.23 ---")

	// range over a function.
	fmt.Print("  range over a function: ")
	for v := range m012Count(5) {
		fmt.Print(v, " ")
	}
	fmt.Println()

	// The iterator bridges.
	s := []string{"c", "a", "b"}
	fmt.Printf("  slices.Sorted(slices.Values(s)) = %v\n", slices.Sorted(slices.Values(s)))
	fmt.Print("  slices.Backward: ")
	for _, v := range slices.Backward(s) {
		fmt.Print(v, " ")
	}
	fmt.Println()

	m := map[string]int{"b": 2, "a": 1}
	fmt.Printf("  maps.Keys returns an ITERATOR, not a slice: %T\n", maps.Keys(m))
	fmt.Printf("    so wrap it: slices.Sorted(maps.Keys(m)) = %v\n", slices.Sorted(maps.Keys(m)))

	// unique.
	h1 := unique.Make("a repeated string value")
	h2 := unique.Make("a repeated string value")
	fmt.Printf("  unique.Make: two equal values share one allocation, h1 == h2 is %t\n", h1 == h2)

	fmt.Println("  timers: an unfired, unreferenced Timer/Ticker is now collected,")
	fmt.Println("    which fixed the classic time.After leak in a hot loop")
	fmt.Println("  STOP WRITING: callback iteration, a goroutine+channel just to make a sequence")
}

// =================================================================================================
// Section 4: Go 1.24 — generic aliases, os.Root, b.Loop, tool
// =================================================================================================

/*
## Go 1.24

### Language

  - **Generic type aliases**: `type Set[T comparable] = map[T]struct{}` (module 010, Section 6).
    Aliases could not take type parameters before, which made refactoring generic code awkward.

### Library

  - **`os.Root`**: open a directory and confine every subsequent operation to it. Path traversal is
    refused **by the operating system**, not by string inspection — the right answer to a whole
    class of vulnerability. Use it whenever a path comes from outside your program.
  - **`weak`**: `weak.Pointer[T]`, a reference the garbage collector may clear. It is what makes
    caches that must not retain their contents possible, and what `unique` uses internally.
  - **`testing.B.Loop`**: `for b.Loop()` replaces `for range b.N`. It excludes setup from the timing
    automatically and — importantly — the compiler cannot optimise the loop body away, which the old
    form permitted, producing benchmarks that silently measured nothing.
  - **`json:",omitzero"`**: omit a field when it is the type's zero value. `omitempty` never omitted
    a zero struct or a zero `time.Time`; `omitzero` does.
  - `crypto/mlkem` (post-quantum), `runtime.AddCleanup` (a better `SetFinalizer`),
    `strings.SplitSeq` and friends returning iterators.

### Runtime and toolchain

  - **Swiss-table maps**: a new map implementation, typically 30% faster for lookups and using less
    memory. **No API changed** — only the constant factors.
  - **The `tool` directive in `go.mod`**: `go get -tool`, then `go tool <name>`. It replaces the
    `tools.go` build-tag hack, so everyone gets the same version of every code generator and linter.
  - `go test -json` improvements, and cgo build-time gains.

**What to stop writing**: manual `filepath.Clean` + prefix checks for path safety, `for range b.N`,
a `tools.go` file, and `omitempty` where you meant "when it is zero".
*/

func m016Go124() {
	fmt.Println("\n--- Section 4: Go 1.24 ---")

	// Generic aliases.
	set := m002bSet[string]{"a": {}, "b": {}}
	fmt.Printf("  generic alias `type Set[T comparable] = map[T]struct{}`: %d entries\n", len(set))

	// os.Root.
	dir, _ := os.MkdirTemp("", "m016-*")
	defer os.RemoveAll(dir)
	_ = os.WriteFile(dir+"/ok.txt", []byte("safe"), 0o600)
	if root, err := os.OpenRoot(dir); err == nil {
		defer root.Close()
		_, inErr := root.Open("ok.txt")
		_, outErr := root.Open("../../../etc/passwd")
		fmt.Printf("  os.Root: inside=%v  escape=%v\n", inErr, outErr)
		fmt.Println("    the escape is refused by the OS - use it for any externally supplied path")
	}

	// weak pointers.
	value := &m016Payload{Name: "collectable"}
	wp := weak.Make(value)
	fmt.Printf("  weak.Pointer: Value() while still reachable = %v\n", wp.Value() != nil)
	fmt.Println("    once nothing else refers to it, the GC may clear the weak pointer")

	// b.Loop.
	fmt.Println("  testing.B.Loop: `for b.Loop()` replaces `for range b.N`")
	fmt.Println("    it excludes setup from the timing and stops the compiler eliding the body")

	// omitzero.
	fmt.Println("  json:\",omitzero\" finally omits a zero struct or a zero time.Time")

	// Swiss tables and the tool directive.
	fmt.Println("  Swiss-table maps: ~30% faster lookups, less memory, no API change")
	fmt.Println("  the `tool` directive in go.mod replaces the tools.go build-tag hack")

	// strings iterators added in 1.24.
	fmt.Print("  strings.SplitSeq (an iterator, no slice allocated): ")
	for part := range strings.SplitSeq("a,b,c", ",") {
		fmt.Printf("%q ", part)
	}
	fmt.Println()
}

type m016Payload struct{ Name string }

// =================================================================================================
// Section 5: Go 1.25 — synctest, WaitGroup.Go, container-aware GOMAXPROCS
// =================================================================================================

/*
## Go 1.25

### Language

  - The **core type** notion was removed from the specification and replaced with more explicit
    rules. No code changes; the spec simply became easier to reason about.

### Library

  - **`testing/synctest`**: a fake clock and deterministic scheduling for concurrent tests. A
    one-hour timeout test runs in microseconds and cannot flake, and a goroutine still running at
    the end becomes a test failure. This is the most useful testing addition in years (module 014).
  - **`sync.WaitGroup.Go(f)`**: `Add(1)`, `go f()` and `defer Done()` in a single call. It makes the
    classic "`Add` inside the goroutine" race impossible to write.
  - **`reflect.TypeAssert[T]`**: a typed, allocation-free assertion from a `reflect.Value`.
  - **`os.Root`** gained `ReadFile`, `WriteFile`, `MkdirAll`, `RemoveAll`, `Rename`, `Symlink`,
    `Chmod`, `Chown` — enough to do real work inside the sandbox.
  - `slog.GroupAttrs`, `runtime/trace.FlightRecorder`, `net/http.CrossOriginProtection`,
    `hash.Cloner`, `io/fs.ReadLinkFS`, `T.Attr` and `T.Output` on `testing.TB`.

### Runtime

  - **Container-aware `GOMAXPROCS`**: the runtime now reads the cgroup CPU limit, so a pod limited
    to 2 CPUs on a 64-core node no longer sets `GOMAXPROCS` to 64. It also updates it when the limit
    changes. This fixed a great deal of accidental over-parallelism in Kubernetes.
  - The **Green Tea garbage collector**: better scanning locality, 10-40% less GC overhead on
    workloads with many small objects. No API change.
  - `go doc -http` serves documentation locally in a browser.

**What to stop writing**: `time.Sleep` in tests to let a goroutine catch up, the
`wg.Add(1); go func(){ defer wg.Done() ... }()` triple, and manual `GOMAXPROCS` tuning in
containers.
*/

func m016Go125() {
	fmt.Println("\n--- Section 5: Go 1.25 ---")

	// WaitGroup.Go.
	var wg sync.WaitGroup
	results := make([]int, 4)
	for i := range 4 {
		wg.Go(func() { results[i] = i * i }) // Add(1) + go + defer Done(), in one call
	}
	wg.Wait()
	fmt.Printf("  sync.WaitGroup.Go: %v\n", results)
	fmt.Println("    it makes the `Add inside the goroutine` race impossible to write")

	// reflect.TypeAssert.
	var v any = "typed assertion"
	if s, ok := reflect.TypeAssert[string](reflect.ValueOf(v)); ok {
		fmt.Printf("  reflect.TypeAssert[string]: %q (typed, no allocation)\n", s)
	}
	if _, ok := reflect.TypeAssert[int](reflect.ValueOf(v)); !ok {
		fmt.Println("  reflect.TypeAssert[int] correctly reports no match")
	}

	// slog.GroupAttrs.
	fmt.Println("  slog.GroupAttrs builds a group from a []slog.Attr (module 013)")

	// Container-aware GOMAXPROCS.
	fmt.Printf("  GOMAXPROCS=%d on a %d-CPU machine\n", runtime.GOMAXPROCS(0), runtime.NumCPU())
	fmt.Println("    since 1.25 this reads the cgroup CPU limit, and updates when it changes")
	fmt.Println("  the Green Tea GC: 10-40% less GC overhead on many-small-object workloads")

	// synctest.
	fmt.Println("  testing/synctest: a fake clock and deterministic scheduling for tests")
	fmt.Println("    see TestSynctestFakeClock in mod_014_testing_test.go - it sleeps an hour,")
	fmt.Println("    instantly, and asserts the elapsed time is EXACTLY one hour")
	fmt.Println("  STOP WRITING: time.Sleep in a test to let a goroutine catch up")
}

// =================================================================================================
// Section 6: Go 1.26 — errors.AsType, slog.MultiHandler, reflect iterators
// =================================================================================================

/*
## Go 1.26

### Library

  - **`errors.AsType[T](err) (T, bool)`**: the generic `errors.As`. No pre-declared variable, no
    `&target`, and the type parameter states the intent at the call site. It is the form to prefer
    in new code (module 009, Section 4).
  - **`slog.NewMultiHandler(handlers...)`**: fan one log record out to several handlers — readable
    text to the console and JSON to a file, from one logger, with no third-party package.
  - **`reflect` iterators**: `Type.Fields()`, `Type.Methods()`, `Type.Ins()`, `Type.Outs()`,
    `Value.Fields()`, `Value.Methods()`. Reflective code that used to be an index loop over
    `NumField()` is now an ordinary `range`.
  - **`bytes.Buffer.Peek`**: look at the next bytes without consuming them.
  - **`testing.TB.ArtifactDir()`**: a directory for files a test produces — screenshots, logs,
    profiles — that a CI system can collect.
  - `net.Dialer.DialTCP`/`DialUDP`/`DialIP`/`DialUnix` taking a `context` and `netip` types;
    `net/http.ClientConn` for explicit connection management; `netip.Prefix.Compare`.
  - `crypto/hpke` (Hybrid Public Key Encryption) and further post-quantum work.

### Runtime

  - Further garbage-collector improvements, and `GODEBUG=tracebacklabels` to include pprof labels in
    tracebacks (its default flipped in 1.27).

**What to stop writing**: `var target *MyErr; errors.As(err, &target)`, a hand-rolled multi-handler,
and `for i := 0; i < t.NumField(); i++` in reflective code.
*/

func m016Go126() {
	fmt.Println("\n--- Section 6: Go 1.26 ---")

	// errors.AsType.
	wrapped := fmt.Errorf("outer: %w", &m009ParseError{Line: 7, Column: 2, Token: "]"})
	if pe, ok := errors.AsType[*m009ParseError](wrapped); ok {
		fmt.Printf("  errors.AsType[*m009ParseError]: line=%d column=%d\n", pe.Line, pe.Column)
	}
	fmt.Println("    no &target and no pre-declared variable - the type parameter says it all")

	// reflect iterators.
	fmt.Print("  reflect Type.Fields() as an iterator: ")
	for f := range reflect.TypeOf(m013Point{}).Fields() {
		fmt.Printf("%s ", f.Name)
	}
	fmt.Println()
	fmt.Print("  reflect Type.Methods(): ")
	for meth := range reflect.TypeOf(&m007Account{}).Methods() {
		fmt.Printf("%s ", meth.Name)
	}
	fmt.Println()
	fmt.Println("    this replaces `for i := 0; i < t.NumField(); i++`")

	// slog.MultiHandler.
	var buf strings.Builder
	multi := slog.New(slog.NewMultiHandler(
		slog.NewTextHandler(&buf, &slog.HandlerOptions{
			ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					return slog.Attr{}
				}
				return a
			},
		}),
	))
	multi.Info("one record", slog.String("to", "many handlers"))
	fmt.Printf("  slog.NewMultiHandler: %s", buf.String())

	// bytes.Buffer.Peek is shown in module 013, Section 2.
	fmt.Println("  bytes.Buffer.Peek: look ahead without consuming (module 013)")
	fmt.Println("  testing.TB.ArtifactDir(): a directory for test output that CI can collect")
}

// =================================================================================================
// Section 7: Go 1.27 — generic methods, json/v2, uuid
// =================================================================================================

/*
## Go 1.27

The current release, and the one with the largest language change since generics themselves.

### Language

  - **Generic methods.** A method may now declare its **own** type parameters in addition to the
    receiver's:

	type List[E any] []E
	func (l List[E]) Apply[F any](f func(E) F) List[F]

    Before 1.27 this was `method must have no type parameters`, and the workaround was a
    package-level function taking the receiver as its first argument — which is exactly why the
    standard library has `slices.SortFunc(s, f)` rather than `s.SortFunc(f)`. The restriction that
    remains: **a generic method cannot satisfy an interface**, because an interface method has one
    signature and a generic method has infinitely many (module 010, Section 8).
  - **Function type inference in all assignment contexts.** A generic function can now be passed as
    an argument, returned, or stored in a struct field or map value without explicit instantiation.
    Go 1.21 allowed it only when assigning to a variable of function type.
  - **A struct-literal key may be any valid field selector**, including a **promoted** field from an
    embedded struct: `Line{name: "diagonal"}` where `name` comes from an embedded `Object`
    (module 002b, Section 12).

### Library

  - **`encoding/json/v2` and `encoding/json/jsontext`** are stable. The split is syntax
    (`jsontext`) from semantics (`json/v2`); options are call arguments rather than decoder state;
    field matching is case-**sensitive**; duplicate names are rejected; it streams properly and is
    substantially faster. `encoding/json` is unchanged and is not deprecated (module 013, Section 6).
  - **`uuid`**: RFC 9562 UUIDs in the standard library. `New`, `NewV4`, `NewV7` (time-ordered — use
    this for database keys), `Parse`, `MustParse`, `Compare`, and text marshalling.
  - **`strings.CutLast` and `bytes.CutLast`**: the mirror of `Cut`, splitting at the **last**
    separator.
  - **`math/rand/v2` `(*Rand).N[Int]`**: the first generic method in the standard library.
  - **`hash/maphash.Hasher[T]` and `ComparableHasher[T]`**: hashing any comparable type.
  - **`testing/synctest.Sleep`**: advance the fake clock and wait, in one call.
  - **`net/http/httptest.NewTestServer(t, h)`**: registers its own cleanup, so there is no
    `defer srv.Close()` to forget.
  - `net/url.URL.Clone` and `Values.Clone`; `math/big.Int.Divide` with an explicit rounding mode;
    `database/sql` column-level scanning; Unicode 17.

### Toolchain

  - A batch of long-deprecated `GODEBUG` settings was **removed** (`gotypesalias`, `tlsrsakex`,
    `asynctimerchan` and others) — a reminder that the compatibility escape hatches are temporary.
  - `crypto/mldsa`: ML-DSA post-quantum signatures.

**What to start writing**: a generic method where a transformation genuinely belongs on the type,
`strings.CutLast` instead of `LastIndex` plus two slices, `uuid.NewV7()` for database keys, and
`json/v2` in new code that cares about speed or strictness.
*/

func m016Go127() {
	fmt.Println("\n--- Section 7: Go 1.27 ---")

	// --- Generic methods ---
	list := m010List[int]{1, 2, 3}
	labels := list.Apply(func(n int) string { return fmt.Sprintf("<%d>", n) })
	fmt.Printf("  generic method: m010List[int].Apply -> %v (%T)\n", labels, labels)
	fmt.Println("    before 1.27 this was `method must have no type parameters`")
	fmt.Println("    the standard library adopted it at once: math/rand/v2 (*Rand).N[Int]")
	r := rand.New(rand.NewPCG(7, 7))
	fmt.Printf("    r.N(100)=%d  r.N(int64(100))=%d\n", r.N(100), r.N(int64(100)))
	fmt.Println("    still not allowed: a generic method cannot satisfy an interface")

	// --- Inference in all assignment contexts ---
	table := map[string]func(int, int) int{"min": m010Min[int]}
	inferred := map[string]func(int) int{"identity": m010Identity} // inferred, no [int]
	fmt.Printf("  inference into a map value with no instantiation: %d / %d\n",
		table["min"](4, 9), inferred["identity"](5))
	type holder struct{ Pick func(string, string) string }
	h := holder{Pick: m010Min} // inferred into a struct field
	fmt.Printf("  inferred into a struct field: %q\n", h.Pick("b", "a"))

	// --- Promoted-field struct literal keys ---
	line := m002bLine{Name: "diagonal", Length: 5}
	fmt.Printf("  promoted-field literal key: %+v\n", line)
	fmt.Println("    `Name` comes from the embedded m002bObject; before 1.27 this needed")
	fmt.Println("    m002bLine{m002bObject: m002bObject{Name: \"diagonal\"}, ...}")

	// --- strings.CutLast ---
	dir, base, _ := strings.CutLast("/usr/local/bin/go", "/")
	stem, ext, _ := strings.CutLast("archive.tar.gz", ".")
	fmt.Printf("  strings.CutLast: (%q, %q) and (%q, %q)\n", dir, base, stem, ext)

	// --- uuid ---
	fmt.Printf("  uuid.NewV7(): %v  <- time-ordered; use it for database keys\n", uuid.NewV7())

	// --- json/v2 ---
	fmt.Println("  encoding/json/v2 + jsontext are stable (module 013, Section 6):")
	fmt.Println("    options are arguments, matching is case-sensitive, duplicates rejected,")
	fmt.Println("    it streams properly, and it refuses to guess - a raw time.Duration is an error")

	// --- synctest.Sleep, httptest.NewTestServer ---
	fmt.Println("  testing/synctest.Sleep: advance the fake clock and wait, in one call")
	fmt.Println("  httptest.NewTestServer(t, h): registers its own cleanup (module 014)")

	fmt.Println()
	fmt.Println("  removed in 1.27: the long-deprecated GODEBUG settings gotypesalias,")
	fmt.Println("  tlsrsakex, asynctimerchan and others - the escape hatches are temporary")
}

// =================================================================================================
// Section 8: The Cumulative Picture
// =================================================================================================

/*
## The Cumulative Picture

If you last wrote Go against 1.20, here is the whole delta as a set of habits.

### Stop writing

	your own Min/Max helper           -> min, max (builtins, 1.21)
	sort.Slice(s, func(i, j int)...)  -> slices.Sort / slices.SortFunc (1.21)
	golang.org/x/exp/slices|maps      -> slices, maps (1.21)
	x/exp/constraints.Ordered         -> cmp.Ordered (1.21)
	sync.Once + a package variable    -> sync.OnceValue (1.21)
	i := i at the top of a loop       -> nothing; fixed in 1.22
	for i := 0; i < n; i++            -> for i := range n (1.22)
	math/rand                         -> math/rand/v2 (1.22)
	a callback-based ForEach          -> an iter.Seq (1.23)
	filepath.Clean + prefix checks    -> os.Root (1.24)
	for range b.N                     -> for b.Loop() (1.24)
	a tools.go file                   -> the `tool` directive (1.24)
	time.Sleep in a test              -> testing/synctest (1.25)
	wg.Add(1); go func(){defer...}()  -> wg.Go(f) (1.25)
	var t *E; errors.As(err, &t)      -> errors.AsType[*E](err) (1.26)
	for i := 0; i < t.NumField()      -> range t.Fields() (1.26)
	LastIndex + two slice expressions -> strings.CutLast (1.27)
	github.com/google/uuid            -> uuid (1.27)

### Start considering

  - **generic methods** where a transformation belongs on the type (1.27)
  - **`json/v2`** in new code that cares about speed, strictness or streaming (1.27)
  - **`uuid.NewV7`** rather than v4 for anything used as a database key (1.27)
  - **`slog`** instead of `log` for anything whose output is collected (1.21)
  - **iterators** for sequences that are not already slices (1.23)
  - **`unique`** when you hold many equal copies of the same value (1.23)
  - **PGO** — a `default.pgo` file is 2-7% for free (1.21)

### The pattern

Almost every one of these replaces something you had to hand-write, or removes a footgun that used
to need a workaround. Go got larger, but idiomatic Go got **shorter** — and the two changes that
cost nothing at all to adopt, the loop-variable fix and Swiss-table maps, simply made existing code
correct and faster.
*/

func m016CumulativePicture() {
	fmt.Println("\n--- Section 8: The Cumulative Picture ---")

	type change struct{ old, new, release string }
	changes := []change{
		{"your own Min/Max helper", "min, max (builtins)", "1.21"},
		{"sort.Slice", "slices.Sort / slices.SortFunc", "1.21"},
		{"x/exp/slices, x/exp/maps", "slices, maps", "1.21"},
		{"x/exp/constraints.Ordered", "cmp.Ordered", "1.21"},
		{"sync.Once + a package var", "sync.OnceValue", "1.21"},
		{"i := i at the top of a loop", "nothing - it was fixed", "1.22"},
		{"for i := 0; i < n; i++", "for i := range n", "1.22"},
		{"math/rand", "math/rand/v2", "1.22"},
		{"a callback-based ForEach", "an iter.Seq", "1.23"},
		{"filepath.Clean + prefix check", "os.Root", "1.24"},
		{"for range b.N", "for b.Loop()", "1.24"},
		{"a tools.go file", "the `tool` directive", "1.24"},
		{"time.Sleep in a test", "testing/synctest", "1.25"},
		{"wg.Add(1); go func(){...}()", "wg.Go(f)", "1.25"},
		{"var t *E; errors.As(err, &t)", "errors.AsType[*E](err)", "1.26"},
		{"for i := 0; i < t.NumField()", "range t.Fields()", "1.26"},
		{"LastIndex + two slices", "strings.CutLast", "1.27"},
		{"github.com/google/uuid", "uuid", "1.27"},
	}

	fmt.Printf("  %-31s -> %-30s %s\n", "STOP WRITING", "USE INSTEAD", "SINCE")
	for _, c := range changes {
		fmt.Printf("  %-31s -> %-30s %s\n", c.old, c.new, c.release)
	}

	fmt.Println()
	fmt.Println("  the two that cost nothing to adopt, because they needed no code change at all:")
	fmt.Println("    the loop-variable fix (1.22) made existing code CORRECT")
	fmt.Println("    Swiss-table maps (1.24) and the Green Tea GC (1.25) made it FASTER")
	fmt.Println()
	fmt.Printf("  this module was compiled and run by %s\n", runtime.Version())
	fmt.Printf("  at %s\n", time.Now().Format(time.RFC3339))
}

// Run016 runs every section of module 016 in order.
func Run016() {
	m016Go121()
	m016Go122()
	m016Go123()
	m016Go124()
	m016Go125()
	m016Go126()
	m016Go127()
	m016CumulativePicture()
}

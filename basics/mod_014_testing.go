package basics

import (
	"errors"
	"fmt"
	"strings"
)

/*
# Module 014 — Testing

Testing is part of the Go toolchain, not a library you choose. `go test` finds every `_test.go`
file, compiles it into the package, and runs every `TestXxx`. There is no test runner to install,
no annotations, and — deliberately — **no assertion library in the standard library**.

That last point is a real design decision, argued in the FAQ: assertions encourage terse failure
messages that do not say what went wrong. Go's convention is an `if` and a `t.Errorf` that prints
what you got and what you wanted. Many teams add `github.com/google/go-cmp` for comparing large
values, and that is the one dependency worth taking.

**The runnable tests for this module are in `mod_014_testing_test.go`.** This file is the prose;
run the tests with:

	go test ./basics/                       # run them
	go test -v ./basics/                    # with each subtest named
	go test -run 'TestTableDriven/negative' ./basics/
	go test -bench . -benchmem ./basics/    # benchmarks with allocation counts
	go test -cover ./basics/                # coverage
	go test -race ./basics/                 # the race detector
	go test -fuzz FuzzReverse ./basics/     # fuzzing (runs until you stop it)
*/

// =================================================================================================
// Section 1: The Basics of go test
// =================================================================================================

/*
## The Basics of go test

- A test file's name **must end in `_test.go`**. It is compiled only during `go test` and is never
  part of the shipped binary.
- Four function shapes are recognised, all needing the `testing` import:

	func TestXxx(t *testing.T)          a test
	func BenchmarkXxx(b *testing.B)     a benchmark
	func FuzzXxx(f *testing.F)          a fuzz target
	func ExampleXxx()                   a documented, verified example

  The name after `Test` must start with an **upper-case letter** — `Testfoo` is not a test.
- A test file may be in the **same package** (`package basics`) and see unexported identifiers, or
  in the **external test package** (`package basics_test`), which sees only the exported API. The
  second is a useful discipline: it tests what your users can actually reach, and it breaks import
  cycles when testing a package that your test's dependencies import.
- **Failure reporting**:
    - `t.Error` / `t.Errorf` — mark failed, **keep going**
    - `t.Fatal` / `t.Fatalf` — mark failed, **stop this test immediately** (it calls
      `runtime.Goexit`, so it must not be called from a helper goroutine)
    - `t.Log` / `t.Logf` — printed only with `-v`, or on failure
    - `t.Skip` / `t.Skipf` — skip, usually guarded by `testing.Short()`
- The conventional failure message names both sides, in a fixed order:

	t.Errorf("Reverse(%q) = %q, want %q", in, got, want)

- **`t.Helper()`** marks a function as a helper, so failures are reported at the *caller's* line
  rather than inside the helper. Always call it first in any assertion helper you write.
- **`t.Cleanup(f)`** registers cleanup that runs when the test and all its subtests finish, in LIFO
  order. Prefer it to `defer`: it works from helpers, and it runs even after `t.Fatal`.
- **`TestMain(m *testing.M)`** takes over the whole package's test run for one-time setup and
  teardown. It must call `m.Run()` and then `os.Exit` with its result.
*/

// m014Reverse is the unit under test for most of this module.
func m014Reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// m014IsEven mirrors examples/condition.go, this repo's original testing example.
func m014IsEven(n int) bool { return n%2 == 0 }

var m014ErrEmpty = errors.New("empty input")

// m014ParseKV parses "k=v" pairs, and is the unit for the table-driven and fuzz tests.
func m014ParseKV(s string) (map[string]string, error) {
	if strings.TrimSpace(s) == "" {
		return nil, m014ErrEmpty
	}
	out := make(map[string]string)
	for pair := range strings.SplitSeq(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		k, v, found := strings.Cut(pair, "=")
		if !found {
			return nil, fmt.Errorf("m014ParseKV: %q is not a key=value pair", pair)
		}
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out, nil
}

func m014Basics() {
	fmt.Println("--- Section 1: The Basics of go test ---")
	fmt.Println("  a test file ends in _test.go and is never part of the shipped binary")
	fmt.Println("  TestXxx / BenchmarkXxx / FuzzXxx / ExampleXxx - the name must be Upper-cased")
	fmt.Println("  package basics      -> the internal test, sees unexported identifiers")
	fmt.Println("  package basics_test -> the external test, sees only the exported API")
	fmt.Println()
	fmt.Println("  Error/Errorf   mark failed, keep going")
	fmt.Println("  Fatal/Fatalf   mark failed, stop this test now (never from a goroutine)")
	fmt.Println("  Log/Logf       shown with -v, or on failure")
	fmt.Println("  Skip/Skipf     skip, usually behind testing.Short()")
	fmt.Println("  Helper()       report failures at the CALLER's line")
	fmt.Println("  Cleanup(f)     LIFO teardown that survives t.Fatal - prefer it to defer")
	fmt.Println()
	fmt.Printf("  the units under test here: m014Reverse(%q)=%q, m014IsEven(4)=%t\n",
		"Gęś", m014Reverse("Gęś"), m014IsEven(4))
	kv, _ := m014ParseKV("a=1, b=2")
	fmt.Printf("  m014ParseKV(\"a=1, b=2\") = %v\n", kv)
}

// =================================================================================================
// Section 2: Table-Driven Tests and Subtests
// =================================================================================================

/*
## Table-Driven Tests and Subtests

The **table-driven test** is the dominant Go testing idiom. A slice of anonymous structs holds the
cases; one loop runs them all:

	tests := []struct {
	    name string
	    in   string
	    want string
	}{
	    {"empty", "", ""},
	    {"ascii", "abc", "cba"},
	    {"utf8", "Gęś", "śęG"},
	}
	for _, tt := range tests {
	    t.Run(tt.name, func(t *testing.T) {
	        if got := Reverse(tt.in); got != tt.want {
	            t.Errorf("Reverse(%q) = %q, want %q", tt.in, got, tt.want)
	        }
	    })
	}

- **`t.Run(name, f)`** creates a **subtest** with its own `*testing.T`. Subtests give you: a name
  in the output, independent pass/fail, `-run` filtering by path
  (`-run 'TestReverse/utf8'`), and a scope for `t.Parallel` and `t.Cleanup`.
- Subtest names have spaces replaced by underscores and are made unique by appending `#01`, so keep
  them short and identifier-like.
- **`t.Parallel()`** signals that a subtest may run concurrently with other parallel subtests of the
  same parent. The parent's own body finishes first, *then* the parallel children run — which is why
  a `defer` in the parent runs too early and `t.Cleanup` is the right tool.
    - Since **Go 1.22** the loop variable is per-iteration, so the old `tt := tt` line before
      `t.Parallel()` is no longer needed. In pre-1.22 code its absence is a real bug: every parallel
      subtest saw the last case.
- **`-race` is close to mandatory** for anything concurrent, and the combination
  `go test -race -count=2` also catches state leaking between runs.
- Keep the table's cases **independent**. If one case's setup affects another, the `-run` filter and
  `t.Parallel` will both produce mystifying results.
*/

func m014TableDriven() {
	fmt.Println("\n--- Section 2: Table-Driven Tests and Subtests ---")
	fmt.Println("  the dominant Go idiom: a slice of anonymous structs plus one loop")
	fmt.Println("  t.Run gives each case a name, independent failure, and -run filtering:")
	fmt.Println("    go test -run 'TestTableDriven/utf8' ./basics/")
	fmt.Println("  t.Parallel runs subtests concurrently; the parent body finishes first,")
	fmt.Println("  so use t.Cleanup rather than defer for anything the children need")
	fmt.Println("  since Go 1.22 the `tt := tt` line before t.Parallel is no longer needed")
	fmt.Println()
	fmt.Println("  see TestTableDriven and TestParallelSubtests in mod_014_testing_test.go")
}

// =================================================================================================
// Section 3: Benchmarks
// =================================================================================================

/*
## Benchmarks

	func BenchmarkReverse(b *testing.B) {
	    for b.Loop() {          // Go 1.24; before that: for range b.N
	        Reverse("hello")
	    }
	}

- The framework calls the benchmark repeatedly with a growing iteration count until the timing is
  statistically stable, then reports **nanoseconds per operation**.
- **`b.Loop()` (Go 1.24) is the form to use.** Compared with the old `for i := 0; i < b.N; i++`:
    - it runs setup **outside** the timed region automatically, so `b.ResetTimer()` is usually
      unnecessary
    - it **prevents the compiler from optimising the loop body away**, which the old form did not —
      a benchmark measuring nothing at all was a classic and silent mistake
    - the body executes exactly once per iteration, so per-iteration state is easier to reason about
- **`-benchmem`** adds allocations per operation and bytes per operation. These numbers are usually
  more actionable than the timing, because allocation count is what you can actually reduce.
- `b.ReportAllocs()` turns that on per benchmark; `b.ReportMetric` adds your own units.
- `b.Run` creates **sub-benchmarks**, which is how you sweep an input size.
- **`benchstat`** (`golang.org/x/perf/cmd/benchstat`) compares two runs and reports whether the
  difference is significant. Run with `-count=10` and compare — a single run is noise.
- The classic mistakes: benchmarking with the timer running during setup, letting the compiler
  eliminate the work (fixed by `b.Loop`), and measuring on a laptop with a thermal governor.
*/

func m014Benchmarks() {
	fmt.Println("\n--- Section 3: Benchmarks ---")
	fmt.Println("  for b.Loop() { ... }   <- Go 1.24, the form to use")
	fmt.Println("    - setup outside the loop is excluded from the timing automatically")
	fmt.Println("    - the compiler cannot optimise the body away, which it could with b.N")
	fmt.Println("  go test -bench . -benchmem ./basics/")
	fmt.Println("  go test -bench . -count=10 ./basics/ | benchstat -   <- compare properly")
	fmt.Println("  allocations per op are usually more actionable than nanoseconds per op")
	fmt.Println()
	fmt.Println("  see BenchmarkReverse and BenchmarkParseKV in mod_014_testing_test.go")
}

// =================================================================================================
// Section 4: Fuzzing
// =================================================================================================

/*
## Fuzzing

	func FuzzReverse(f *testing.F) {
	    f.Add("abc")                       // seed corpus
	    f.Fuzz(func(t *testing.T, s string) {
	        doubled := Reverse(Reverse(s))
	        if utf8.ValidString(s) && doubled != s {
	            t.Errorf("round trip failed: %q -> %q", s, doubled)
	        }
	    })
	}

- **Fuzzing is built in since Go 1.18.** `f.Add` supplies seeds; `f.Fuzz` takes a function whose
  parameters after `*testing.T` are the generated inputs. Only a fixed set of types is supported:
  the numeric types, `bool`, `string`, `[]byte` and `rune`.
- `go test` alone runs the **seed corpus only**, as ordinary deterministic test cases. Actual
  fuzzing needs `-fuzz FuzzXxx`, and it **runs until you stop it** or it finds a failure.
- A failing input is written to `testdata/fuzz/FuzzXxx/` and is then replayed as a normal test case
  on every future `go test`. **Commit that file** — it is a regression test the fuzzer wrote for you.
- Fuzzing needs a **property** to check, not an expected value. The productive shapes are:
    - a **round trip**: `Decode(Encode(x)) == x`
    - a **differential**: your fast implementation agrees with an obviously-correct slow one
    - an **invariant**: the output is always sorted, or is always valid UTF-8
    - **it does not panic**: the weakest property, and still finds plenty of bugs
- Only one fuzz target can run at a time, and `-fuzz` cannot be combined with a multi-package
  pattern.
*/

func m014Fuzzing() {
	fmt.Println("\n--- Section 4: Fuzzing ---")
	fmt.Println("  go test -fuzz FuzzReverse ./basics/    <- runs until stopped or a failure")
	fmt.Println("  go test ./basics/                      <- runs only the seed corpus")
	fmt.Println("  a failing input is saved to testdata/fuzz/... and replayed forever after")
	fmt.Println("  COMMIT that file: it is a regression test the fuzzer wrote for you")
	fmt.Println()
	fmt.Println("  fuzz for a PROPERTY, not an expected value:")
	fmt.Println("    round trip     Decode(Encode(x)) == x")
	fmt.Println("    differential   the fast version agrees with the obviously-correct slow one")
	fmt.Println("    invariant      the output is always sorted / always valid UTF-8")
	fmt.Println("    no panic       the weakest property, and still finds plenty")

	// The round-trip property, checked here on a few values.
	for _, s := range []string{"", "abc", "Gęś", "🙂🙃"} {
		fmt.Printf("    reverse twice %-8q -> %-8q holds=%t\n",
			s, m014Reverse(m014Reverse(s)), m014Reverse(m014Reverse(s)) == s)
	}
}

// =================================================================================================
// Section 5: Examples as Documentation
// =================================================================================================

/*
## Examples as Documentation

	func ExampleReverse() {
	    fmt.Println(Reverse("hello"))
	    // Output: olleh
	}

- An `Example` function is **both documentation and a test**. `go doc` and pkg.go.dev show it
  alongside the identifier it names, and `go test` **runs it and compares stdout to the `// Output:`
  comment**. Documentation that cannot go stale is the whole point.
- Without an `// Output:` comment the example is **compiled but not run** — useful when it needs
  network access, but easy to do by accident and then wonder why it never fails.
- **`// Unordered output:`** compares the lines as a set, which is what you need for anything
  involving map iteration.
- Naming determines placement: `Example`, `ExampleFoo`, `ExampleT`, `ExampleT_M`, and a suffix after
  an underscore starting lower-case for a variant, `ExampleFoo_withOptions`. `go vet` checks that
  the name refers to a real identifier, so an example for an **unexported** unit must use the
  package-level form `Example_lowerCaseName` — which is what this module's examples do.
- Examples live in `_test.go` files, and an example in the **external test package**
  (`package basics_test`) is the honest one, because it can only use the exported API — exactly what
  a reader would write.
*/

func m014Examples() {
	fmt.Println("\n--- Section 5: Examples as Documentation ---")
	fmt.Println("  an Example is documentation AND a test: go test compares stdout to")
	fmt.Println("  the `// Output:` comment, so the docs cannot go stale")
	fmt.Println("  `// Unordered output:` compares lines as a set - use it for map iteration")
	fmt.Println("  no Output comment at all means it is COMPILED BUT NOT RUN")
	fmt.Println("  naming: ExampleFoo, ExampleT_M, ExampleFoo_withOptions - and go vet checks")
	fmt.Println("  that the name is a real identifier, so an example for an UNEXPORTED unit")
	fmt.Println("  must use the package-level form Example_lowerCaseName")
	fmt.Println()
	fmt.Println("  see Example_reverse and Example_parseKV in mod_014_testing_test.go, and run `go test -v ./basics/`")
}

// =================================================================================================
// Section 6: testing/synctest (Go 1.25)
// =================================================================================================

/*
## testing/synctest (Go 1.25)

Testing concurrent code has always had the same two problems: a test that sleeps is **slow**, and a
test that depends on scheduling is **flaky**. `testing/synctest`, stable in Go 1.25, fixes both.

	synctest.Test(t, func(t *testing.T) {
	    start := time.Now()
	    done := make(chan struct{})
	    go func() { time.Sleep(time.Hour); close(done) }()
	    <-done
	    // elapsed is exactly one hour, and the test took microseconds
	    if elapsed := time.Since(start); elapsed != time.Hour {
	        t.Errorf("elapsed = %v, want 1h", elapsed)
	    }
	})

Inside the *bubble* created by `synctest.Test`:

  - **Time is fake.** `time.Now`, `time.Sleep`, `time.After`, timers and tickers all use a virtual
    clock that starts at a fixed instant and advances **instantly** whenever every goroutine in the
    bubble is blocked. A one-hour timeout test runs in microseconds and measures *exactly* an hour.
  - **`synctest.Wait()`** blocks until every other goroutine in the bubble is durably blocked. That
    replaces the `time.Sleep(10 * time.Millisecond)` that everyone writes to "let the goroutine
    catch up", and which is the single biggest source of flaky Go tests.
  - **`synctest.Sleep(d)`** (**Go 1.27**) advances the fake clock by `d` and then waits, combining
    the two operations that were almost always written together.
  - The bubble **must be fully drained**: if a goroutine started inside it is still running when the
    function returns, `synctest.Test` panics. That is a feature — it turns a goroutine leak into a
    test failure.

Caveats: only goroutines started *inside* the bubble are bubbled, and real I/O (a network call, a
file read) blocks for real and will deadlock the fake clock. Use it for logic that is concurrent,
not for integration tests.

This is the most useful testing addition in years — if you have ever written
`time.Sleep(100 * time.Millisecond) // give it time to finish`, this replaces it.
*/

func m014Synctest() {
	fmt.Println("\n--- Section 6: testing/synctest (Go 1.25) ---")
	fmt.Println("  synctest.Test(t, func(t *testing.T) { ... }) creates a BUBBLE where:")
	fmt.Println("    time is fake and advances instantly when every goroutine is blocked")
	fmt.Println("    a one-hour timeout test runs in microseconds and measures exactly an hour")
	fmt.Println("    synctest.Wait() waits until every other goroutine is durably blocked")
	fmt.Println("    synctest.Sleep(d) (Go 1.27) advances the clock and then waits")
	fmt.Println("    a goroutine still running at the end makes the test PANIC - leaks become failures")
	fmt.Println()
	fmt.Println("  it replaces every `time.Sleep(100*time.Millisecond) // let it catch up`,")
	fmt.Println("  which is the biggest single source of flaky Go tests")
	fmt.Println()
	fmt.Println("  see TestSynctestFakeClock and TestSynctestWait in mod_014_testing_test.go")
}

// =================================================================================================
// Section 7: Coverage, Test Doubles and the Rest of the Toolchain
// =================================================================================================

/*
## Coverage, Test Doubles and the Rest of the Toolchain

### Coverage

	go test -cover ./...
	go test -coverprofile=cover.out ./... && go tool cover -html=cover.out
	go test -covermode=atomic ./...        # required when combined with -race

Coverage is a **diagnostic, not a target**. It tells you what is definitely untested; it says
nothing about whether the covered lines are tested *well*. Chasing 100% produces tests that assert
nothing.

### Test doubles

Go needs no mocking framework, because interfaces are satisfied implicitly (module 008):

  - take an **`io.Reader`** instead of an `*os.File`, and `strings.NewReader` is your fake
  - take a small **interface** you defined at the point of use, and write a struct with the two
    methods you need
  - **embed the interface** in your fake and override only the methods the test exercises
    (module 008, Section 4)
  - `net/http/httptest` gives you a real HTTP server and a `ResponseRecorder`. **Go 1.27** added
    `httptest.NewTestServer(t, handler)`, which registers its own cleanup so you cannot forget
    `defer srv.Close()`
  - `testing/fstest.MapFS` is an in-memory filesystem satisfying `fs.FS`

### The rest

  - **`go-cmp`** (`github.com/google/go-cmp/cmp`) for comparing large values: it reports a readable
    diff rather than `false`, and `cmpopts` controls how. This is the one testing dependency worth
    taking, and it is what `reflect.DeepEqual` should have been.
  - **`t.TempDir()`** creates a directory removed automatically at the end of the test.
  - **`t.Setenv(k, v)`** sets an environment variable and restores it afterwards. It makes the test
    non-parallel, deliberately.
  - **`t.Attr(key, value)`** (Go 1.25) attaches structured metadata to a test result, and
    **`t.ArtifactDir()`** (Go 1.26) gives a directory for files a test produces — screenshots, logs,
    profiles — that the CI system can collect.
  - **`go test -json`** produces machine-readable output; `-shuffle=on` randomises test order to
    catch order dependencies; `-count=1` defeats the test cache.
*/

func m014CoverageAndDoubles() {
	fmt.Println("\n--- Section 7: Coverage, Test Doubles and the Rest of the Toolchain ---")
	fmt.Println("  go test -coverprofile=cover.out ./... && go tool cover -html=cover.out")
	fmt.Println("  coverage is a diagnostic, not a target - it shows what is definitely untested")
	fmt.Println()
	fmt.Println("  test doubles need no framework, because interfaces are implicit:")
	fmt.Println("    take io.Reader, and strings.NewReader is your fake")
	fmt.Println("    embed the interface in a struct and override only what you exercise")
	fmt.Println("    httptest.NewTestServer(t, h)  <- Go 1.27, registers its own cleanup")
	fmt.Println("    fstest.MapFS                  <- an in-memory fs.FS")
	fmt.Println()
	fmt.Println("  t.TempDir()      removed automatically")
	fmt.Println("  t.Setenv(k, v)   restored automatically (and forces the test non-parallel)")
	fmt.Println("  t.Attr(k, v)     Go 1.25: structured metadata on the result")
	fmt.Println("  t.ArtifactDir()  Go 1.26: a directory for files CI should collect")
	fmt.Println("  -shuffle=on      randomise order, to catch order dependencies")
	fmt.Println("  -count=1         defeat the test cache")
	fmt.Println()
	fmt.Println("  github.com/google/go-cmp is the one testing dependency worth taking:")
	fmt.Println("  it reports a readable diff instead of reflect.DeepEqual's bare false")
}

// Run014 runs every section of module 014 in order.
func Run014() {
	m014Basics()
	m014TableDriven()
	m014Benchmarks()
	m014Fuzzing()
	m014Examples()
	m014Synctest()
	m014CoverageAndDoubles()
}

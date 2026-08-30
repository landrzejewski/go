package basics

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

/*
# Module 005 — Functions

Functions are first-class values in Go: they have types, can be stored in variables, passed as
arguments, returned from other functions, and can close over their environment. What Go leaves out
is as telling as what it includes — there is **no overloading**, **no default parameter values**,
and **no named arguments at the call site**. One name means one function, and reading a call tells
you exactly which one runs.
*/

// =================================================================================================
// Section 1: Declarations and Parameters
// =================================================================================================

/*
## Declarations and Parameters

- `func name(params) results { body }`. Parameters are `name Type`, and **consecutive parameters of
  the same type may share it**: `func add(a, b int)` is `func add(a int, b int)`.
- **All arguments are passed by value** — always, with no exceptions. Passing a large struct copies
  it; passing a pointer copies the pointer. Passing a slice copies the three-word header, which is
  why the *elements* appear to be shared while `append` does not propagate (module 006).
- There is **no function overloading**. Two functions in a package cannot share a name, whatever
  their signatures. The Go answer is distinct names (`Parse`, `ParseInt`, `ParseFloat`) or a variadic
  options parameter.
- There are **no default parameter values** and **no named arguments**. The idiomatic substitutes are
  a config struct or the *functional options* pattern (Section 5).
- A function with no results simply omits the result list. Parameters may be unnamed if the body
  does not use them — useful when satisfying an interface.
- Functions are **not** required to be declared before use: package-level order does not matter.
- Recursion works with no special syntax. Go has **no tail-call optimisation**, so deep recursion
  grows the stack — but goroutine stacks start at ~8 KB and grow dynamically up to 1 GB, so
  recursion depths that would overflow in C are usually fine.
*/

// Consecutive parameters of the same type share it.
func m005Add(a, b int) int { return a + b }

// Unnamed parameters are legal when the body ignores them.
func m005Ignore(int, string) string { return "arguments deliberately unused" }

// No overloading: this must have a different name from m005Add.
func m005AddFloats(a, b float64) float64 { return a + b }

func m005Factorial(n int) int {
	if n <= 1 {
		return 1
	}
	return n * m005Factorial(n-1)
}

type m005BigStruct struct{ Data [1024]int }

func m005ByValue(s m005BigStruct) { s.Data[0] = 99 }

func m005ByPointer(s *m005BigStruct) { s.Data[0] = 99 }

func m005Declarations() {
	fmt.Println("--- Section 1: Declarations and Parameters ---")

	fmt.Printf("m005Add(2, 3) = %d\n", m005Add(2, 3))
	fmt.Printf("m005AddFloats(2, 3) = %v  (a second name, because there is no overloading)\n",
		m005AddFloats(2, 3))
	fmt.Println(m005Ignore(1, "x"))
	fmt.Printf("m005Factorial(10) = %d\n", m005Factorial(10))

	// --- Everything is passed by value ---
	big := m005BigStruct{}
	m005ByValue(big)
	fmt.Printf("after passing a 1024-int struct by value: Data[0]=%d (the copy was modified)\n", big.Data[0])
	m005ByPointer(&big)
	fmt.Printf("after passing a pointer:                   Data[0]=%d\n", big.Data[0])

	// A slice header is copied too - but the header points at shared elements.
	s := []int{1, 2, 3}
	m005ModifyElement(s)
	fmt.Printf("a function CAN modify slice elements: %v\n", s)
	m005TryAppend(s)
	fmt.Printf("but its append is invisible to us:    %v (module 006 explains why)\n", s)

	// No defaults, no named arguments:
	//	func greet(name string, greeting string = "Hello") // ERROR: syntax error: unexpected = in parameter list; possibly missing comma or )
	//	m005Add(a: 1, b: 2)                                 // ERROR: syntax error: unexpected : in argument list; possibly missing comma or )
	fmt.Println("no default values and no named arguments - use a config struct or options")
}

func m005ModifyElement(s []int) { s[0] = 99 }

func m005TryAppend(s []int) { s = append(s, 4); _ = s }

// =================================================================================================
// Section 2: Multiple Return Values and Named Results
// =================================================================================================

/*
## Multiple Return Values and Named Results

- A function may return **any number of values**: `func f() (int, string, error)`. This is what lets
  Go handle errors as ordinary values rather than exceptions, and it is used everywhere.
- The dominant convention is `(result, error)` with the error **last**. A close second is the
  **comma-ok** form `(value, bool)` used by map reads, type assertions and channel receives.
- Every returned value must be **consumed or discarded**. `v, _ := f()` explicitly throws one away;
  silently ignoring one is a compile error (`assignment mismatch`).
- **Named results** — `func f() (sum int, err error)` — declare the results as variables,
  initialised to their zero values at entry. Their two real uses are:
    1. letting a **deferred closure inspect or modify** what is about to be returned. This is the
       only way to turn a panic into an error return (Section 6), and the only way to attach a
       `Close` error to the result (module 004, Section 6).
    2. **documenting** what the results mean when the types alone do not:
       `func Split(path string) (dir, file string)`.
- A **naked return** — `return` with no operands in a function with named results — returns their
  current values. It is legal and occasionally neat in a very short function, but in anything longer
  it hides what is actually returned. Most style guides, including Google's, discourage it.
- The **result of a multi-value call can be spread** into another call, but only as the *sole*
  argument: `fmt.Println(f())` works if `f`'s results match; `fmt.Println(f(), x)` does not.
*/

// The dominant convention: the error comes last.
func m005Divide(dividend, divisor float64) (float64, error) {
	if divisor == 0 {
		return 0, errors.New("division by zero")
	}
	return dividend / divisor, nil
}

// Named results used for documentation: the types alone would not say which is which.
func m005SplitPath(path string) (dir, file string) {
	i := strings.LastIndex(path, "/")
	if i < 0 {
		return ".", path
	}
	return path[:i], path[i+1:]
}

// Named results used properly: the defer inspects what is about to be returned.
func m005TracedDivide(a, b float64) (result float64, err error) {
	defer func() {
		if err != nil {
			fmt.Printf("    [trace] %v / %v failed: %v\n", a, b, err)
		} else {
			fmt.Printf("    [trace] %v / %v = %v\n", a, b, result)
		}
	}()
	return m005Divide(a, b)
}

// A naked return. Legal, but the reader has to scroll up to see what comes back.
func m005NakedReturn(n int) (doubled int, label string) {
	doubled = n * 2
	label = "doubled"
	return // returns (doubled, label)
}

func m005MultipleReturns() {
	fmt.Println("\n--- Section 2: Multiple Return Values and Named Results ---")

	if q, err := m005Divide(10, 4); err == nil {
		fmt.Printf("m005Divide(10, 4) = %v\n", q)
	}
	if _, err := m005Divide(1, 0); err != nil {
		fmt.Printf("m005Divide(1, 0) -> %v\n", err)
	}

	// Every result must be consumed or explicitly discarded.
	//	q := m005Divide(10, 4) // ERROR: assignment mismatch: 1 variable but m005Divide returns 2 values
	q, _ := m005Divide(10, 4)
	fmt.Printf("discarding the error with _: %v\n", q)

	// Named results as documentation.
	dir, file := m005SplitPath("/usr/local/bin/go")
	fmt.Printf("m005SplitPath: dir=%q file=%q\n", dir, file)

	// Named results plus a defer that inspects them.
	fmt.Println("named results inspected by a defer:")
	_, _ = m005TracedDivide(9, 3)
	_, _ = m005TracedDivide(9, 0)

	// Naked return.
	d, l := m005NakedReturn(21)
	fmt.Printf("naked return gave (%d, %q) - legal, but avoid it in longer functions\n", d, l)

	// Spreading a multi-value call into another call - only as the sole argument.
	fmt.Print("spreading a multi-value result into Println: ")
	fmt.Println(m005SplitPath("/a/b"))
	//	fmt.Println(m005SplitPath("/a/b"), "extra") // ERROR: multiple-value m005SplitPath(...) in single-value context
}

// =================================================================================================
// Section 3: Variadic Functions
// =================================================================================================

/*
## Variadic Functions

- The **last** parameter may be variadic, written `...T`. Inside the function it is an ordinary
  `[]T`, so `len`, `range` and indexing all work.
- Called with no variadic arguments, the parameter is a **nil slice** — not an empty one. `len` is 0
  either way, so this rarely matters, but `== nil` will be true.
- **Spreading**: `f(s...)` passes an existing `[]T` directly. Go **does not copy** it — the callee
  receives a slice header over *your* array, so a callee that writes to it modifies your data. This
  surprises people who assume variadics are always fresh.
- You cannot mix: `f(1, 2, s...)` is a compile error. Build one slice first, or use
  `append([]T{1, 2}, s...)`.
- A `...any` parameter is what `fmt.Println` uses. Note that spreading a `[]string` into `...any`
  does **not** compile — the element types differ — so you must convert element by element. This is
  a frequent annoyance and a good reason to reach for generics (module 010).
- `go vet`'s `printf` check understands variadic format functions and will flag argument/verb
  mismatches — one of the highest-value checks in the toolchain.
*/

func m005SumAll(values ...int) (sum int) {
	for _, v := range values {
		sum += v
	}
	return sum
}

func m005Describe(prefix string, values ...any) string {
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, fmt.Sprintf("%v(%T)", v, v))
	}
	return prefix + ": " + strings.Join(parts, ", ")
}

func m005Variadic() {
	fmt.Println("\n--- Section 3: Variadic Functions ---")

	fmt.Printf("m005SumAll()          = %d\n", m005SumAll())
	fmt.Printf("m005SumAll(1,2,3)     = %d\n", m005SumAll(1, 2, 3))

	// Spreading an existing slice.
	nums := []int{4, 5, 6}
	fmt.Printf("m005SumAll(nums...)   = %d\n", m005SumAll(nums...))

	// Inside the function the parameter is a plain slice - and with no arguments it is NIL.
	m005ReportVariadic()
	m005ReportVariadic(1)

	// Spreading does NOT copy: the callee shares your backing array.
	shared := []int{1, 2, 3}
	m005ClobberFirst(shared...)
	fmt.Printf("after spreading into a callee that writes to it: %v (no copy was made)\n", shared)

	// Mixing fixed and spread arguments is not allowed:
	//	m005SumAll(1, 2, nums...) // ERROR: have (number, number, []int...) want (...int)
	combined := append([]int{1, 2}, nums...)
	fmt.Printf("build one slice instead: m005SumAll(%v...) = %d\n", combined, m005SumAll(combined...))

	// ...any
	fmt.Println(m005Describe("mixed", 1, "two", 3.0, true))

	// A []string cannot be spread into ...any - the element types differ.
	words := []string{"a", "b"}
	//	fmt.Println(m005Describe("x", words...)) // ERROR: cannot use words (variable of type []string) as []any value in argument
	asAny := make([]any, len(words))
	for i, w := range words {
		asAny[i] = w
	}
	fmt.Println(m005Describe("converted element by element", asAny...))
}

func m005ReportVariadic(values ...int) {
	fmt.Printf("inside the function: %v len=%d isNil=%t\n", values, len(values), values == nil)
}

func m005ClobberFirst(values ...int) {
	if len(values) > 0 {
		values[0] = 999
	}
}

// =================================================================================================
// Section 4: Functions as Values
// =================================================================================================

/*
## Functions as Values

- A function is a **first-class value** with a type: `func(int, int) int`. Two function types are
  identical if their parameter and result types match — parameter *names* are irrelevant.
- Function values can be stored in variables, slices, maps and struct fields, passed as arguments
  and returned from functions. A map of `func()` is the idiomatic dispatch table, and this
  package's own `Module.Run` field is exactly that.
- The zero value of a function type is `nil`, and **calling a nil function panics**. Function values
  are comparable **only against `nil`** — you cannot ask whether two functions are the same.
- A **method value** — `x.Method` — is a function value that has captured `x` as its receiver.
  A **method expression** — `T.Method` — is a function whose first parameter is the receiver.
  Both are ordinary function values thereafter.
- A **function type declaration** (`type Handler func(string) error`) names a signature. It makes
  APIs readable and — because it is a defined type — can carry methods, which is exactly how
  `http.HandlerFunc` turns a plain function into an interface implementation (module 008).
- Higher-order functions are ordinary Go, but note that the standard library reaches for them less
  than other languages do: `slices.SortFunc` and `slices.IndexFunc` exist, but there is no
  `Map`/`Filter`/`Reduce` in the standard library, because a `for` loop is usually clearer. Module
  012 shows the generic iterator versions.
*/

// A named function type documents the signature and can carry methods.
type m005Transform func(int) int

// A method on a function type - the trick behind http.HandlerFunc.
func (t m005Transform) Twice(n int) int { return t(t(n)) }

type m005Counter struct{ n int }

func (c *m005Counter) Inc()    { c.n++ }
func (c m005Counter) Get() int { return c.n }

func m005FunctionsAsValues() {
	fmt.Println("\n--- Section 4: Functions as Values ---")

	// A function in a variable.
	var double m005Transform = func(n int) int { return n * 2 }
	fmt.Printf("double(21) = %d, and via its method: double.Twice(3) = %d\n", double(21), double.Twice(3))

	// The zero value is nil, and calling it panics.
	var nilFunc func()
	fmt.Printf("a nil function value: isNil=%t\n", nilFunc == nil)
	fmt.Println("calling it would panic: runtime error: invalid memory address or nil pointer dereference")

	// Function values compare only against nil.
	//	fmt.Println(double == double) // ERROR: invalid operation: double == double (func can only be compared to nil)

	// A dispatch table - the same shape as this package's Modules registry.
	ops := map[string]func(int, int) int{
		"add": func(a, b int) int { return a + b },
		"mul": func(a, b int) int { return a * b },
		"sub": func(a, b int) int { return a - b },
	}
	for _, name := range slices.Sorted(m005MapKeys(ops)) {
		fmt.Printf("  ops[%q](6, 3) = %d\n", name, ops[name](6, 3))
	}

	// --- Method values and method expressions ---
	c := &m005Counter{}
	inc := c.Inc // a METHOD VALUE: the receiver c is already bound
	inc()
	inc()
	fmt.Printf("method value c.Inc bound the receiver: n=%d\n", c.Get())

	get := m005Counter.Get // a METHOD EXPRESSION: the receiver becomes the first parameter
	fmt.Printf("method expression m005Counter.Get(*c) = %d\n", get(*c))

	// --- Higher-order functions ---
	numbers := []int{5, 2, 8, 1}
	sorted := slices.Clone(numbers)
	slices.SortFunc(sorted, func(a, b int) int { return b - a }) // descending
	fmt.Printf("slices.SortFunc descending: %v\n", sorted)
	idx := slices.IndexFunc(numbers, func(n int) bool { return n > 4 })
	fmt.Printf("slices.IndexFunc(n > 4) = %d\n", idx)

	// A function that takes a function.
	fmt.Print("m005ForEach: ")
	m005ForEach(numbers, func(i, v int) { fmt.Printf("%d:%d ", i, v) })
	fmt.Println()
}

func m005ForEach(values []int, task func(index, value int)) {
	for i, v := range values {
		task(i, v)
	}
}

// m005MapKeys is a tiny local helper; module 012 uses maps.Keys from the standard library instead.
func m005MapKeys[K comparable, V any](m map[K]V) func(func(K) bool) {
	return func(yield func(K) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// =================================================================================================
// Section 5: Closures
// =================================================================================================

/*
## Closures

- A **function literal** (`func(x int) int { ... }` with no name) is a closure: it captures the
  variables of the enclosing scope **by reference**, not by value. It sees their later changes, and
  writes through them are visible outside.
- A captured variable **escapes to the heap** and stays alive as long as the closure does. That is
  how a factory function can return a counter whose state survives the factory's return — something
  that would be a dangling pointer in C.
- Because capture is by reference, **several closures created in the same scope share the same
  variable**. To give each its own copy, declare a fresh variable inside the loop or scope. Since
  Go 1.22 the loop variable itself is already fresh per iteration, which removed the most common
  instance of this problem (module 004, Section 3).
- Closures are the mechanism behind: the **functional options** pattern, `defer func(){}()`,
  middleware, memoisation, and every `sync.Once`-guarded lazy initialiser.
- Watch the lifetime: a closure capturing a big object keeps it alive. A long-lived callback holding
  a reference to a request-scoped buffer is a real memory leak.
*/

// m005IDGeneratorFactory returns a closure over its own private counter.
func m005IDGeneratorFactory(prefix string) func() string {
	id := 0 // captured; escapes to the heap and outlives this function
	return func() string {
		id++
		return fmt.Sprintf("%s-%03d", prefix, id)
	}
}

// The functional options pattern: Go's substitute for default and named parameters.
type m005Server struct {
	host    string
	port    int
	timeout int
	tls     bool
}

type m005Option func(*m005Server)

func m005WithPort(p int) m005Option    { return func(s *m005Server) { s.port = p } }
func m005WithTimeout(t int) m005Option { return func(s *m005Server) { s.timeout = t } }
func m005WithTLS() m005Option          { return func(s *m005Server) { s.tls = true } }

func m005NewServer(host string, opts ...m005Option) *m005Server {
	s := &m005Server{host: host, port: 80, timeout: 30} // the defaults live here
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func m005Closures() {
	fmt.Println("\n--- Section 5: Closures ---")

	// --- State that survives the factory ---
	nextUser := m005IDGeneratorFactory("user")
	nextOrder := m005IDGeneratorFactory("order")
	fmt.Printf("%s %s %s | %s %s  <- two independent counters\n",
		nextUser(), nextUser(), nextUser(), nextOrder(), nextOrder())

	// --- Capture is by reference ---
	x := 1
	show := func() { fmt.Printf("  the closure sees x = %d\n", x) }
	show()
	x = 42
	show()
	fmt.Println("  the closure observed the later assignment - capture is by reference")

	// And it can write back.
	total := 0
	addToTotal := func(n int) { total += n }
	for _, n := range []int{1, 2, 3} {
		addToTotal(n)
	}
	fmt.Printf("  a closure writing to an outer variable: total=%d\n", total)

	// --- Sharing versus copying ---
	shared := 0
	var sharers []func() int
	for range 3 {
		sharers = append(sharers, func() int { shared++; return shared })
	}
	fmt.Printf("  three closures over ONE variable: %d %d %d\n", sharers[0](), sharers[1](), sharers[2]())

	var owners []func() int
	for i := range 3 {
		own := i * 10 // a fresh variable per iteration
		owners = append(owners, func() int { return own })
	}
	fmt.Printf("  three closures over their OWN variable: %d %d %d\n", owners[0](), owners[1](), owners[2]())

	// --- Functional options ---
	def := m005NewServer("localhost")
	custom := m005NewServer("example.com", m005WithPort(8443), m005WithTLS(), m005WithTimeout(5))
	fmt.Printf("  defaults: %+v\n", *def)
	fmt.Printf("  options:  %+v\n", *custom)
	fmt.Println("  this is how Go does default and named parameters")

	// --- Memoisation ---
	fib := m005MemoFib()
	fmt.Printf("  memoised fib(50) = %d (instant; the naive recursion would take minutes)\n", fib(50))
}

func m005MemoFib() func(int) int {
	cache := map[int]int{}
	var fib func(int) int
	fib = func(n int) int { // declared first so the literal can refer to itself
		if n < 2 {
			return n
		}
		if v, ok := cache[n]; ok {
			return v
		}
		v := fib(n-1) + fib(n-2)
		cache[n] = v
		return v
	}
	return fib
}

// =================================================================================================
// Section 6: panic and recover
// =================================================================================================

/*
## panic and recover

- `panic(v)` stops normal execution, runs every **deferred** function up the stack, and — if nothing
  recovers — prints the value and a stack trace and exits with status 2.
- `recover()` stops that unwinding. It is **only meaningful when called directly from a deferred
  function** of the panicking frame. Called anywhere else it returns `nil` and does nothing. It
  returns the value passed to `panic`.
- **`panic` is not an exception mechanism.** Go's rule is that expected failures are `error` return
  values; `panic` is for programmer bugs and truly unrecoverable states. Do not use it for control
  flow, and do not let it cross a package boundary — a library that panics on bad input is
  considered broken.
- The runtime panics on its own for: nil pointer dereference, index out of range, slice bounds out
  of range, integer divide by zero, a failed single-value type assertion, closing a closed channel,
  sending on a closed channel. These arrive as `runtime.Error` values.
- **Some failures cannot be recovered.** `fatal error: concurrent map writes`, `fatal error: all
  goroutines are asleep - deadlock!` and stack exhaustion bypass `recover` entirely and kill the
  process. Recover is not a safety net for data races.
- **A panic in one goroutine kills the whole program**, even if another goroutine has a `recover`.
  Each goroutine must protect itself. This is the number-one source of surprise server crashes.
- The legitimate patterns are:
    1. **recover at a boundary** — an HTTP handler or worker loop turning a panic into a 500 and a
       log line, so one bad request does not take down the process. `net/http` does this for you.
    2. **panic across a package's own internals**, recovered at every exported entry point — used by
       `encoding/json` and by parsers to avoid threading an error through deep recursion.
    3. **`MustXxx` helpers** — `regexp.MustCompile`, `template.Must` — that panic on inputs which are
       compile-time constants in practice, so a failure means the program is simply wrong.
- Re-panicking (`panic(r)` inside the recover) after logging preserves the crash while adding
  context. Note it *replaces* the original stack trace; wrap the value if you need the original.
*/

func m005PanicAndRecover() {
	fmt.Println("\n--- Section 6: panic and recover ---")

	// --- recover turns a panic into an error ---
	fmt.Println("  ", m005SafeDivide(10, 2))
	fmt.Println("  ", m005SafeDivide(10, 0))

	// --- Runtime panics ---
	for _, f := range []func(){
		func() { var p *m005Counter; p.Inc() },
		func() { s := []int{1}; _ = s[5] },
		func() { a, b := 1, 0; _ = a / b },
		func() { var i any = "text"; _ = i.(int) },
		func() { ch := make(chan int); close(ch); close(ch) },
	} {
		fmt.Printf("   runtime panic: %v\n", m005CatchPanic(f))
	}

	// --- recover only works in a DEFERRED function of the panicking frame ---
	fmt.Println("  ", m005RecoverInWrongPlace())

	// --- The MustXxx pattern ---
	fmt.Printf("   MustPositive(5) = %d\n", m005MustPositive(5))
	fmt.Printf("   MustPositive(-1) -> %v\n", m005CatchPanic(func() { m005MustPositive(-1) }))

	// --- A panic in another goroutine cannot be recovered from here ---
	fmt.Println("   a panic in a goroutine kills the whole program - each goroutine needs its own")
	fmt.Println("   recover; see module 011")
}

// m005SafeDivide is the boundary pattern: a panic becomes an ordinary error.
func m005SafeDivide(a, b int) (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprintf("%d/%d recovered: %v", a, b, r)
		}
	}()
	return fmt.Sprintf("%d/%d = %d", a, b, a/b)
}

// m005CatchPanic runs f and returns whatever it panicked with.
func m005CatchPanic(f func()) (recovered any) {
	defer func() { recovered = recover() }()
	f()
	return "did not panic"
}

// m005RecoverInWrongPlace shows that a non-deferred recover does nothing.
func m005RecoverInWrongPlace() string {
	if r := recover(); r != nil { // not deferred, and nothing is panicking: always nil
		return fmt.Sprintf("recovered %v", r)
	}
	return "recover() outside a deferred function returns nil and does nothing"
}

// m005MustPositive is the Must pattern: a bad argument means the program itself is wrong.
func m005MustPositive(n int) int {
	if n <= 0 {
		panic(fmt.Sprintf("m005MustPositive: %d is not positive", n))
	}
	return n
}

// Run005 runs every section of module 005 in order.
func Run005() {
	m005Declarations()
	m005MultipleReturns()
	m005Variadic()
	m005FunctionsAsValues()
	m005Closures()
	m005PanicAndRecover()
}

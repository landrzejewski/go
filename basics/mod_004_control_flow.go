package basics

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
)

/*
# Module 004 — Control Flow

Go has exactly **three** control-flow keywords for branching and looping: `if`, `switch` and `for`.
There is no `while`, no `do-while`, no `for-each` as a separate construct, and no ternary. `for`
absorbs every loop shape, and `switch` is far more capable than its C ancestor.

Two rules apply everywhere in this module:

  - the condition must be a **`bool`** — no truthiness, no assignment-as-expression
  - the opening brace must be on the **same line**, because of semicolon insertion (module 001a)
*/

// =================================================================================================
// Section 1: if Statements
// =================================================================================================

/*
## if Statements

- `if cond { }` — the condition is **not parenthesised**, and the braces are **mandatory** even for
  a single statement. That combination makes the `goto fail` class of bug impossible to write.
- The condition must be a `bool`. `if 1 {}`, `if s {}` and `if p {}` are all compile errors.
- `if` may carry an **init statement**: `if v, err := f(); err != nil { }`. The variables it
  declares are scoped to the whole if/else chain and vanish afterwards. This is the single most
  common idiom in Go, because it keeps `err` from leaking into the enclosing scope where it could be
  accidentally reused or shadowed.
- `else` and `else if` must start on the **same line as the closing brace** of the previous block —
  `} else {`. Putting `else` on its own line inserts a semicolon after `}` and breaks the chain.
- `if` is a **statement, not an expression**. It has no value, so there is no `x := if c {1} else {2}`.
- Style: prefer the **early return**. Handle the error case in the `if` and return, leaving the happy
  path unindented at the bottom of the function. Go code reads as a series of guards followed by the
  real work; deeply nested `else` blocks are considered a smell.
*/

func m004IfStatements() {
	fmt.Println("--- Section 1: if Statements ---")

	n := 42
	if n > 0 {
		fmt.Println("positive")
	} else if n < 0 {
		fmt.Println("negative")
	} else {
		fmt.Println("zero")
	}

	// Braces are mandatory; the condition is unparenthesised.
	//	if n > 0 fmt.Println("x")   // ERROR: syntax error: unexpected name fmt, expected {
	//	if (n > 0) { }              // legal but non-idiomatic; gofmt leaves it, vet says nothing

	// The condition must be a bool.
	//	if n { }  // ERROR: non-boolean condition in if statement

	// --- The init statement ---
	if value, err := m004Lookup("go"); err != nil {
		fmt.Println("lookup failed:", err)
	} else {
		fmt.Printf("init statement: value=%q, and both value and err are scoped to this chain\n", value)
	}
	//	fmt.Println(err) // ERROR: undefined: err

	if _, err := m004Lookup("rust"); err != nil {
		fmt.Println("lookup failed:", err)
	}

	// --- else must share the closing brace's line ---
	// Written like this:
	//	if n > 0 {
	//	}
	//	else {
	//	}
	// a semicolon is inserted after the `}`, and the compiler reports:
	// ERROR: syntax error: unexpected else, expected }
	fmt.Println("`} else {` must be on one line - semicolon insertion again")

	// --- Early return over nested else ---
	fmt.Println("early-return style:", m004Classify(15))
	fmt.Println("early-return style:", m004Classify(-1))
}

func m004Lookup(key string) (string, error) {
	data := map[string]string{"go": "a programming language"}
	if v, ok := data[key]; ok {
		return v, nil
	}
	return "", fmt.Errorf("key %q not found", key)
}

// m004Classify shows the early-return style: guards first, happy path last and unindented.
func m004Classify(n int) string {
	if n < 0 {
		return "negative"
	}
	if n == 0 {
		return "zero"
	}
	if n%15 == 0 {
		return "fizzbuzz"
	}
	return "positive"
}

// =================================================================================================
// Section 2: switch Statements
// =================================================================================================

/*
## switch Statements

Go's `switch` is much more than C's. There are three distinct forms.

### Expression switch

`switch x { case a, b: ... }`. Cases may list **several values**, and — unlike C — cases do **not
fall through**: each `case` body ends the switch, so no `break` is needed. The values need not be
constants, and the type need only be comparable.

### Tagless switch

`switch { case cond1: ... }` with no expression at all. This is `switch true`, and it replaces a
long `if/else if` chain. It is idiomatic Go and often clearer than the equivalent chain.

### Type switch

`switch v := x.(type) { case int: ... }` on an **interface** value, dispatching on the dynamic type.
Covered properly in module 008; mentioned here for completeness.

### Details that matter

- `switch` may carry an **init statement** exactly like `if`: `switch x := f(); x { ... }`.
- `fallthrough` transfers control to the **next case body unconditionally**, without evaluating its
  condition. It must be the last statement in a case, and it cannot appear in the final case or in a
  type switch. It is rare and usually a mistake.
- `default` may appear **anywhere**, not only last, though putting it last is conventional. If no
  case matches and there is no default, nothing happens.
- `break` inside a switch breaks the **switch**, not an enclosing loop. To break the loop you need a
  **label** (Section 5) — a genuine gotcha when converting from other languages.
- An empty case body does nothing and does *not* fall through — the opposite of C, where an empty
  case is the standard way to group values. In Go you list the values on one `case` instead.
- Cases are evaluated **in source order**, top to bottom, and the first match wins.
*/

func m004SwitchStatements() {
	fmt.Println("\n--- Section 2: switch Statements ---")

	// --- Expression switch: no break needed, several values per case ---
	for _, day := range []string{"Saturday", "Monday", "Caturday"} {
		switch day {
		case "Saturday", "Sunday":
			fmt.Printf("  %s: weekend\n", day)
		case "Monday", "Tuesday", "Wednesday", "Thursday", "Friday":
			fmt.Printf("  %s: weekday\n", day)
		default:
			fmt.Printf("  %s: not a day\n", day)
		}
	}

	// With an init statement.
	switch n := len("gopher"); n {
	case 6:
		fmt.Println("init statement in a switch: length is", n)
	default:
		fmt.Println("unexpected length", n)
	}

	// --- Tagless switch: replaces an if/else if chain ---
	for _, score := range []int{95, 72, 41} {
		var grade string
		switch {
		case score >= 90:
			grade = "A"
		case score >= 80:
			grade = "B"
		case score >= 60:
			grade = "C"
		default:
			grade = "F"
		}
		fmt.Printf("  score %d -> %s\n", score, grade)
	}

	// Cases need not be constant.
	limit := 10
	value := 15
	switch {
	case value > limit*2:
		fmt.Println("tagless switch with computed cases: far above the limit")
	case value > limit:
		fmt.Println("tagless switch with computed cases: above the limit")
	}

	// --- fallthrough ---
	fmt.Println("fallthrough runs the NEXT body without testing its condition:")
	switch 2 {
	case 1:
		fmt.Println("  one")
	case 2:
		fmt.Println("  two")
		fallthrough
	case 3:
		fmt.Println("  three (reached by fallthrough, even though 2 != 3)")
	case 4:
		fmt.Println("  four (NOT reached - fallthrough only goes one case deep)")
	}

	// An empty case does NOT fall through - the opposite of C.
	switch 1 {
	case 1:
		// deliberately empty
	case 2:
		fmt.Println("  unreachable: an empty case body ends the switch")
	}
	fmt.Println("an empty case body ends the switch; list values on one case instead")

	// --- break inside a switch breaks the SWITCH, not the loop ---
	fmt.Println("plain `break` in a switch does not leave the loop:")
	for i := range 3 {
		switch i {
		case 1:
			break // leaves the switch only
		}
		fmt.Printf("  iteration %d still runs\n", i)
	}
	fmt.Println("use a labelled break to leave the loop - see Section 5")
}

// =================================================================================================
// Section 3: for Loops — all five forms
// =================================================================================================

/*
## for Loops — all five forms

`for` is Go's only loop keyword. It covers every shape other languages spread across several:

	for i := 0; i < n; i++ { }   // 1. the C-style three-clause loop
	for cond { }                 // 2. the "while" loop
	for { }                      // 3. the infinite loop
	for i, v := range coll { }   // 4. range over a collection
	for range n { }              // 5. range over an integer (Go 1.22)

- The three clauses are all **optional**. Omitting the condition gives an infinite loop; omitting
  init and post gives a while loop. The semicolons disappear along with the clauses.
- There are **no parentheses**, and the braces are **mandatory**.
- `break` leaves the innermost loop; `continue` skips to the post statement. Both accept a label.
- **Go 1.22 changed loop variable scoping**: `i` and `v` are now **fresh variables in every
  iteration**, not one variable reused. Before 1.22, capturing the loop variable in a closure or a
  goroutine captured the *same* variable and every closure saw the final value — the single most
  common Go bug ever. The old behaviour applies only to modules declaring `go 1.21` or earlier in
  `go.mod`, so this is one of the rare language changes gated on the language version.
- **Go 1.22 added `range` over an integer**: `for i := range 10` iterates `i = 0..9`, and
  `for range 10` repeats ten times. It removes most three-clause loops from real code.
- **Go 1.23 added `range` over a function** — iterators. Covered in Section 4 and module 012.
*/

func m004ForLoops() {
	fmt.Println("\n--- Section 3: for Loops — all five forms ---")

	// 1. Three-clause.
	fmt.Print("  three-clause: ")
	for i := 0; i < 5; i++ {
		fmt.Print(i, " ")
	}
	fmt.Println()

	// 2. Condition only - Go's "while".
	fmt.Print("  while-style:  ")
	n := 1
	for n < 20 {
		fmt.Print(n, " ")
		n *= 2
	}
	fmt.Println()

	// 3. Infinite, with an explicit break.
	fmt.Print("  infinite+break: ")
	count := 0
	for {
		count++
		if count > 3 {
			break
		}
		fmt.Print(count, " ")
	}
	fmt.Println()

	// 4. Range over a collection (Section 4 covers every rangeable type).
	fmt.Print("  range slice:  ")
	for i, v := range []string{"a", "b", "c"} {
		fmt.Printf("%d:%s ", i, v)
	}
	fmt.Println()

	// 5. Go 1.22: range over an integer.
	fmt.Print("  range int:    ")
	for i := range 5 {
		fmt.Print(i, " ")
	}
	fmt.Println()
	fmt.Print("  repeat n times (index discarded): ")
	for range 3 {
		fmt.Print("* ")
	}
	fmt.Println()

	// --- continue skips to the post statement ---
	fmt.Print("  odd numbers via continue: ")
	for i := range 10 {
		if i%2 == 0 {
			continue
		}
		fmt.Print(i, " ")
	}
	fmt.Println()

	// --- Go 1.22: the loop variable is fresh in every iteration ---
	// Deliberately the three-clause form: `for i := range 3` is itself new in Go 1.22,
	// so it has no "before" to compare against. This loop does.
	var closures []func() int
	for i := 0; i < 3; i++ {
		closures = append(closures, func() int { return i }) // captures THIS iteration's i
	}
	fmt.Print("  closures capturing the loop variable: ")
	for _, c := range closures {
		fmt.Print(c(), " ")
	}
	fmt.Println("<- 0 1 2 since Go 1.22; it was 3 3 3 before")
	fmt.Println("  (the old behaviour still applies to modules declaring go 1.21 or earlier)")

	// The pre-1.22 workaround, still seen everywhere in older code:
	var oldStyle []func() int
	for i := range 3 {
		i := i // shadow the loop variable to give each closure its own copy
		oldStyle = append(oldStyle, func() int { return i })
	}
	fmt.Print("  the old `i := i` workaround is now redundant: ")
	for _, c := range oldStyle {
		fmt.Print(c(), " ")
	}
	fmt.Println()
}

// =================================================================================================
// Section 4: range — what can be ranged over
// =================================================================================================

/*
## range — what can be ranged over

`range` adapts to its operand, and the number and meaning of the loop variables changes with it:

	array / slice   i, v      index, element (element is a COPY)
	string          i, r      byte offset, rune (decodes UTF-8)
	map             k, v      key, value (order randomised)
	channel         v         each value received, until the channel is closed
	integer         i         0 .. n-1                              (Go 1.22)
	func(yield)     …         whatever the iterator yields          (Go 1.23)

- You may take **fewer** variables than are on offer: `for v := range ch`, `for i := range slice`
  (index only), `for range n`. You cannot take more.
- **The element is a copy.** `for _, v := range items { v.Field = x }` modifies the copy and is a
  no-op on the slice — a very common bug. Use `items[i].Field = x` instead.
- **The range expression is evaluated once**, before the loop starts. For a slice that means the
  length is fixed at entry: appending inside the loop does not extend it. For an array — but not a
  slice — the *whole array is copied* first.
- Ranging over a **nil slice or nil map** is legal and simply runs zero times. Ranging over a **nil
  channel blocks forever**.
- Ranging over a **channel** receives until it is closed. If nobody closes it, the loop deadlocks.
- **Go 1.23: ranging over a function.** `range f` where `f` is `func(yield func(V) bool)` calls `f`,
  which calls `yield` per element; `yield` returning `false` means the loop body broke out. The
  standard signatures are named `iter.Seq[V]` and `iter.Seq2[K, V]`. Module 012 covers them fully.
*/

func m004RangeForms() {
	fmt.Println("\n--- Section 4: range — what can be ranged over ---")

	// Slice: index and a COPY of the element.
	type item struct{ Name string }
	items := []item{{Name: "a"}, {Name: "b"}}
	for _, v := range items {
		v.Name = "modified" // writes to the copy
	}
	fmt.Printf("  modifying the range variable does nothing: %v\n", items)
	for i := range items {
		items[i].Name = "modified" // writes through the index
	}
	fmt.Printf("  indexing works: %v\n", items)

	// Taking fewer variables.
	fmt.Print("  index only: ")
	for i := range items {
		fmt.Print(i, " ")
	}
	fmt.Println()

	// String: byte offset and decoded rune.
	fmt.Print("  string: ")
	for i, r := range "Gęś" {
		fmt.Printf("(%d,%c) ", i, r)
	}
	fmt.Println(" <- offsets jump, because ę and ś are two bytes each")

	// Map: order is randomised.
	m := map[string]int{"a": 1, "b": 2}
	fmt.Print("  map: ")
	for k, v := range m {
		fmt.Printf("%s=%d ", k, v)
	}
	fmt.Println()

	// Channel: receives until closed.
	ch := make(chan int, 3)
	for i := range 3 {
		ch <- i * 10
	}
	close(ch) // without this, the range below would deadlock
	fmt.Print("  channel: ")
	for v := range ch {
		fmt.Print(v, " ")
	}
	fmt.Println()

	// nil slice and nil map range zero times; a nil channel blocks forever.
	var nilSlice []int
	var nilMap map[string]int
	rounds := 0
	for range nilSlice {
		rounds++
	}
	for range nilMap {
		rounds++
	}
	fmt.Printf("  ranging over a nil slice and a nil map ran %d times (no panic)\n", rounds)

	// --- The range expression is evaluated once ---
	grow := []int{1, 2, 3}
	appended := 0
	for range grow {
		grow = append(grow, 99) // does NOT extend this loop
		appended++
	}
	fmt.Printf("  appending inside the loop ran %d times; len is now %d\n", appended, len(grow))

	// For an ARRAY the whole thing is copied before the loop starts.
	arr := [3]int{1, 2, 3}
	for i, v := range arr {
		arr[2] = 99 // modifies the array, but not the copy being ranged
		if i == 2 {
			fmt.Printf("  array range sees the ORIGINAL value %d, not the modified %d\n", v, arr[2])
		}
	}

	// --- Go 1.23: range over a function ---
	fmt.Print("  range over an iterator (Go 1.23): ")
	for v := range m004Countdown(3) {
		fmt.Print(v, " ")
	}
	fmt.Println()

	fmt.Print("  a two-value iterator: ")
	for i, s := range m004Enumerate([]string{"x", "y"}) {
		fmt.Printf("(%d,%s) ", i, s)
	}
	fmt.Println()

	// Breaking out of an iterator: yield returns false and the iterator stops.
	fmt.Print("  breaking out early: ")
	for v := range m004Countdown(10) {
		if v < 8 {
			break
		}
		fmt.Print(v, " ")
	}
	fmt.Println("<- the iterator was told to stop")
}

// m004Countdown returns an iterator yielding n, n-1, ... 1.
func m004Countdown(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := n; i > 0; i-- {
			if !yield(i) { // yield reports false when the loop body breaks
				return
			}
		}
	}
}

// m004Enumerate returns a two-value iterator over a slice.
func m004Enumerate[T any](s []T) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		for i, v := range s {
			if !yield(i, v) {
				return
			}
		}
	}
}

// =================================================================================================
// Section 5: Labels, break, continue and goto
// =================================================================================================

/*
## Labels, break, continue and goto

- A **label** is an identifier followed by a colon, placed before a statement. Labels have their own
  namespace and are scoped to the enclosing function. An **unused label is a compile error**, like
  an unused variable.
- `break Label` leaves the labelled `for`, `switch` or `select` — the standard way to escape a
  nested loop, and the only way to break a loop from inside a `switch`.
- `continue Label` starts the next iteration of the labelled **loop** (labels on `switch` are not
  valid targets for `continue`).
- `goto Label` jumps within the current function. Go keeps `goto` but fences it in: it **cannot jump
  into a block**, and it **cannot jump over a variable declaration** that is in scope at the target.
  Both are compile errors, which rules out the uninitialised-variable hazards that make `goto`
  dangerous in C.
- Legitimate uses of `goto` in real Go are almost entirely limited to generated code and to
  hand-written state machines and parsers in the standard library. In application code, a labelled
  `break` or an extracted function is nearly always better.
- Style note: `gofmt` outdents labels one level relative to the statement they label.
*/

func m004LabelsAndGoto() {
	fmt.Println("\n--- Section 5: Labels, break, continue and goto ---")

	// --- Labelled break escapes nested loops ---
	fmt.Println("  labelled break, searching a grid for 5:")
search:
	for i := range 3 {
		for j := range 3 {
			if i*3+j == 5 {
				fmt.Printf("    found at (%d,%d)\n", i, j)
				break search // leaves BOTH loops
			}
		}
	}

	// A plain break here would only leave the inner loop.
	found := 0
	for i := range 3 {
		for j := range 3 {
			if i*3+j == 5 {
				break // inner loop only
			}
			found++
		}
	}
	fmt.Printf("    with a plain break, the outer loop kept going: %d iterations\n", found)

	// --- Labelled break from inside a switch ---
	fmt.Print("  labelled break out of a switch: ")
loop:
	for i := range 5 {
		switch {
		case i == 3:
			break loop // a plain `break` would only leave the switch
		default:
			fmt.Print(i, " ")
		}
	}
	fmt.Println()

	// --- Labelled continue ---
	fmt.Println("  labelled continue, skipping rows containing a zero:")
rows:
	for _, row := range [][]int{{1, 2}, {3, 0}, {4, 5}} {
		for _, v := range row {
			if v == 0 {
				continue rows // next ROW, not next element
			}
		}
		fmt.Printf("    row %v has no zeros\n", row)
	}

	// --- goto ---
	i := 0
retry:
	i++
	if i < 3 {
		goto retry
	}
	fmt.Printf("  goto used as a retry loop: i=%d\n", i)

	// goto cannot jump over a declaration that is in scope at the label:
	//	goto skip
	//	v := 1     // ERROR: goto skip jumps over declaration of v at ...
	//	skip:
	//	fmt.Println(v)
	// nor into a block:
	//	goto inside
	//	{ inside: fmt.Println("x") } // ERROR: goto inside jumps into block starting at ...
	fmt.Println("  goto cannot jump into a block or over a declaration - both are compile errors")

	// An unused label is an error, just like an unused variable:
	//	unused: // ERROR: label unused defined and not used
}

// =================================================================================================
// Section 6: defer
// =================================================================================================

/*
## defer

- `defer f(args)` schedules `f` to run when the **surrounding function returns** — by any route:
  a `return`, running off the end, or a **panic** unwinding the stack. This is Go's answer to
  `finally` and to RAII, and it is what makes cleanup reliable.
- Deferred calls run in **LIFO order**: the last one deferred is the first to run. That matches the
  usual acquire-in-order, release-in-reverse pattern.
- **The arguments are evaluated immediately**, at the moment of the `defer` statement — not when the
  call finally runs. `defer fmt.Println(i)` captures the value of `i` *now*. To capture it later,
  defer a closure: `defer func() { fmt.Println(i) }()`.
- The **receiver is evaluated immediately** too, which is why `defer mu.Unlock()` is safe but
  `defer p.Close()` on a `p` that is reassigned later will close the *original*.
- A deferred closure can **read and modify named result parameters**, because it runs after the
  return value has been assigned but before the caller sees it. This is how `recover` turns a panic
  into an error return (module 005).
- `defer` is **function-scoped, not block-scoped**. A `defer` inside a `for` body does *not* run at
  the end of the iteration — it piles up until the function returns. Opening files in a loop with
  `defer f.Close()` therefore leaks descriptors. Extract the body into its own function, or call
  `Close` explicitly.
- `defer` has a real but small cost. Since Go 1.14 most defers are "open-coded" and are nearly free;
  do not contort code to avoid one.
- Deferring a call whose error you ignore hides failures. For a writable file, `defer f.Close()`
  can silently discard a write error — close it explicitly and check.
*/

func m004Defer() {
	fmt.Println("\n--- Section 6: defer ---")

	// --- LIFO order ---
	fmt.Println("  LIFO order:")
	func() {
		for i := range 3 {
			defer fmt.Printf("    deferred %d\n", i)
		}
		fmt.Println("    function body finished")
	}()

	// --- Arguments are evaluated at the defer statement ---
	func() {
		i := 1
		defer fmt.Printf("  argument evaluated at defer time: i was %d\n", i)
		i = 99
		_ = i
	}()

	func() {
		i := 1
		defer func() { fmt.Printf("  a closure reads i at run time: i is %d\n", i) }()
		i = 99
	}()

	// --- Modifying a named result ---
	fmt.Println("  named result modified by a defer:", m004DoubleResult(5))

	// --- Cleanup runs even when the function panics ---
	fmt.Println("  cleanup during a panic:", m004PanicWithCleanup())

	// --- defer is function-scoped, not block-scoped ---
	fmt.Println("  a defer inside a loop piles up until the FUNCTION returns:")
	m004DeferInLoopWrong()
	fmt.Println("  extracting the body fixes it:")
	m004DeferInLoopRight()
}

// m004DoubleResult shows a deferred closure mutating the named result after `return` has set it.
func m004DoubleResult(n int) (result int) {
	defer func() { result *= 2 }() // runs after `return n` assigns result = n
	return n
}

func m004PanicWithCleanup() (msg string) {
	defer func() {
		if r := recover(); r != nil {
			msg = fmt.Sprintf("recovered from %v, and the defer still ran", r)
		}
	}()
	defer fmt.Println("    this defer runs even though the function panics")
	panic("something went wrong")
}

func m004DeferInLoopWrong() {
	for i := range 3 {
		// In real code this would be `defer file.Close()` - and all three files would stay open
		// until the function returned.
		defer fmt.Printf("    wrong: cleanup for %d, running only now at function exit\n", i)
	}
	fmt.Println("    loop finished, but nothing has been cleaned up yet")
}

func m004DeferInLoopRight() {
	for i := range 3 {
		func() {
			defer fmt.Printf("    right: cleanup for %d, at the end of ITS iteration\n", i)
		}()
	}
}

// m004ReadConfig is a realistic sketch: acquire, defer the release, then work.
func m004ReadConfig(path string) (err error) {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open config: %w", err)
	}
	// Deferred immediately after the error check - the canonical placement.
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close config: %w", cerr)
		}
	}()
	// The actual work goes here; a real version would decode the bytes.
	if _, err = io.ReadAll(f); err != nil {
		return fmt.Errorf("read config: %w", err)
	}
	return nil
}

// =================================================================================================
// Section 7: Putting it together
// =================================================================================================

/*
## Putting it together

A short worked example using the whole module: a tagless `switch` for classification, an `if` with
an init statement for the error path, a labelled `break` to leave a nested scan, `range` over a
string, and a `defer` for cleanup.

The point is not the algorithm — it is that Go's control flow is small enough that a realistic
function uses nearly all of it, and still reads top to bottom with no nesting deeper than two.
*/

var m004ErrEmpty = errors.New("empty input")

// m004Analyse reports the first digit in s and a description of s's content.
func m004Analyse(s string) (first rune, kind string, err error) {
	defer func() {
		if err != nil {
			kind = "unknown"
		}
	}()

	if len(s) == 0 {
		return 0, "", m004ErrEmpty
	}

	letters, digits, others := 0, 0, 0
scan:
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits++
			if first == 0 {
				first = r
			}
		case r == '!':
			others++
			break scan // stop the whole scan at the first '!'
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			letters++
		default:
			others++
		}
	}

	switch {
	case digits > 0 && letters > 0:
		kind = "alphanumeric"
	case digits > 0:
		kind = "numeric"
	case letters > 0:
		kind = "alphabetic"
	default:
		kind = "other"
	}
	return first, kind, nil
}

func m004PuttingItTogether() {
	fmt.Println("\n--- Section 7: Putting it together ---")

	for _, in := range []string{"go2rust", "12345", "hello", "ab!cd", ""} {
		first, kind, err := m004Analyse(in)
		if err != nil {
			fmt.Printf("  %-8q -> error: %v (kind reset to %q by the defer)\n", in, err, kind)
			continue
		}
		firstStr := "none"
		if first != 0 {
			firstStr = string(first)
		}
		fmt.Printf("  %-8q -> kind=%-12s firstDigit=%s\n", in, kind, firstStr)
	}

	// The defer sketch from Section 6, exercised against a path that does not exist.
	if err := m004ReadConfig("/nonexistent"); err != nil {
		fmt.Printf("  m004ReadConfig on a missing file: %v\n", err)
	}
}

// Run004 runs every section of module 004 in order.
func Run004() {
	m004IfStatements()
	m004SwitchStatements()
	m004ForLoops()
	m004RangeForms()
	m004LabelsAndGoto()
	m004Defer()
	m004PuttingItTogether()
}

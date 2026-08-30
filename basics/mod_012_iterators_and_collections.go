package basics

import (
	"cmp"
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
	"unique"
)

/*
# Module 012 — Iterators and the Collection Packages

Between Go 1.21 and 1.23 the standard library gained the pieces it had been missing for a decade:
`slices`, `maps` and `cmp` in 1.21, and **range-over-function iterators** plus the `iter` package in
1.23. Together they replaced most hand-written loops and the whole of the old `sort` API.

If you learned Go before 1.21, this module is the one with the most that is new to you.
*/

// =================================================================================================
// Section 1: Range Over a Function
// =================================================================================================

/*
## Range Over a Function

- Since **Go 1.23**, `for x := range f` is legal when `f` is a function of one of exactly three
  shapes:

	func(yield func() bool)         // for range f
	func(yield func(V) bool)        // for v := range f
	func(yield func(K, V) bool)     // for k, v := range f

- The `iter` package names the last two: **`iter.Seq[V]`** and **`iter.Seq2[K, V]`**. They are
  ordinary generic type *definitions* for those function types (`type Seq[V any] func(...)`, no
  `=`) — there is no magic and no interface. Note they are definitions, not aliases: generic
  aliases only became legal in Go 1.24, while `iter` shipped in 1.23.
- **How it works**: `range` calls your function, passing it a `yield` closure that contains the loop
  body. You call `yield` once per element. `yield` returns `false` when the loop body executed a
  `break`, `return`, `goto` or a labelled continue out of the loop — at which point **you must stop
  and return**.
- Ignoring `yield`'s result is the one real hazard. If you keep yielding after `false`, the runtime
  panics with `range function continued iteration after function for loop body returned false`.
  (The similarly worded `...after loop body panic` is the separate case of continuing once the loop
  body has panicked.) Always write `if !yield(v) { return }`.
- Iterators are **lazy** and compose without building intermediate slices, and they work over things
  that are not collections at all — a file's lines, a database cursor, a tree walk, an infinite
  sequence.
- **Cleanup**: if your iterator holds a resource, `defer` its release inside the iterator function.
  It runs whether iteration completes or the consumer breaks out.
- Naming convention in the standard library: `All` returns index/key-value pairs, `Values` returns
  just values, `Backward` reverses, `Sorted` collects and sorts, `Collect` materialises.
*/

// The simplest possible iterator.
func m012Count(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := range n {
			if !yield(i) {
				return // the consumer broke out; stop immediately
			}
		}
	}
}

// A two-value iterator.
func m012Pairs[K comparable, V any](keys []K, values []V) iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for i := range min(len(keys), len(values)) {
			if !yield(keys[i], values[i]) {
				return
			}
		}
	}
}

// An iterator over something that is not a collection: the words of a string, lazily.
func m012Words(s string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for len(s) > 0 {
			s = strings.TrimLeft(s, " ")
			if s == "" {
				return
			}
			end := strings.IndexByte(s, ' ')
			if end < 0 {
				yield(s)
				return
			}
			if !yield(s[:end]) {
				return
			}
			s = s[end:]
		}
	}
}

// An iterator holding a resource, released by defer whether or not the consumer breaks.
func m012WithCleanup(n int) iter.Seq[int] {
	return func(yield func(int) bool) {
		fmt.Println("    (resource acquired)")
		defer fmt.Println("    (resource released - runs even on an early break)")
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

// A tree walk, where an iterator is far nicer than a callback.
type m012Tree struct {
	Value       int
	Left, Right *m012Tree
}

func (t *m012Tree) InOrder() iter.Seq[int] {
	return func(yield func(int) bool) {
		var walk func(*m012Tree) bool
		walk = func(n *m012Tree) bool {
			if n == nil {
				return true
			}
			return walk(n.Left) && yield(n.Value) && walk(n.Right)
		}
		walk(t)
	}
}

func m012RangeOverFunc() {
	fmt.Println("--- Section 1: Range Over a Function ---")

	fmt.Print("  iter.Seq[int]: ")
	for v := range m012Count(5) {
		fmt.Print(v, " ")
	}
	fmt.Println()

	fmt.Print("  iter.Seq2[string,int]: ")
	for k, v := range m012Pairs([]string{"a", "b", "c"}, []int{1, 2, 3}) {
		fmt.Printf("%s=%d ", k, v)
	}
	fmt.Println()

	// Breaking out: yield returns false and the iterator stops.
	fmt.Print("  breaking out at 3: ")
	for v := range m012Count(100) {
		if v == 3 {
			break
		}
		fmt.Print(v, " ")
	}
	fmt.Println("<- the iterator returned; it did not run 100 times")

	// An iterator over a non-collection.
	fmt.Print("  words, lazily: ")
	for w := range m012Words("  range over a function  ") {
		fmt.Printf("%q ", w)
	}
	fmt.Println()

	// Cleanup on an early break.
	fmt.Println("  an iterator holding a resource:")
	for v := range m012WithCleanup(10) {
		if v == 2 {
			break
		}
	}

	// A tree walk.
	tree := &m012Tree{Value: 5,
		Left:  &m012Tree{Value: 3, Left: &m012Tree{Value: 1}},
		Right: &m012Tree{Value: 8, Left: &m012Tree{Value: 7}}}
	fmt.Print("  in-order tree walk as an iterator: ")
	for v := range tree.InOrder() {
		fmt.Print(v, " ")
	}
	fmt.Println()
	fmt.Println("  before 1.23 this needed a callback, an explicit stack, or a goroutine + channel")
}

// =================================================================================================
// Section 2: iter.Pull — turning a push iterator into a pull one
// =================================================================================================

/*
## iter.Pull — turning a push iterator into a pull one

- A `Seq` is a **push** iterator: it drives the loop and calls you back. Sometimes you need a
  **pull** iterator instead — one you ask for the next value — for example to merge two sorted
  sequences, to look ahead, or to interleave.
- **`iter.Pull(seq)`** returns `(next func() (V, bool), stop func())`. `iter.Pull2` is the
  two-value version.
- **You must call `stop`**, and `defer stop()` is the way. Without it the underlying iterator's
  goroutine-like coroutine state is never released. Calling `stop` twice is safe; calling `next`
  after `stop` returns the zero value and `false`.
- `Pull` is implemented with **coroutines** inside the runtime, not goroutines, so it is much
  cheaper than the old channel-based trick — but it is still far more expensive than a plain
  `range`. Reach for it only when you genuinely need to control the pace.
*/

func m012IterPull() {
	fmt.Println("\n--- Section 2: iter.Pull ---")

	next, stop := iter.Pull(m012Count(5))
	defer stop() // mandatory

	a, _ := next()
	b, _ := next()
	fmt.Printf("  pulled two values on demand: %d %d\n", a, b)
	c, ok := next()
	fmt.Printf("  and a third: %d ok=%t\n", c, ok)

	// Merging two sorted sequences - the classic reason to need a pull iterator.
	left := slices.Values([]int{1, 4, 9})
	right := slices.Values([]int{2, 3, 10})
	fmt.Printf("  merging two sorted sequences: %v\n", slices.Collect(m012MergeSorted(left, right)))

	// After stop, next reports false forever.
	next2, stop2 := iter.Pull(m012Count(10))
	stop2()
	_, ok2 := next2()
	fmt.Printf("  after stop(), next() reports ok=%t\n", ok2)
}

// m012MergeSorted merges two sorted sequences, which needs look-ahead on both - so it needs Pull.
func m012MergeSorted[T cmp.Ordered](a, b iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		nextA, stopA := iter.Pull(a)
		defer stopA()
		nextB, stopB := iter.Pull(b)
		defer stopB()

		va, okA := nextA()
		vb, okB := nextB()
		for okA && okB {
			if va <= vb {
				if !yield(va) {
					return
				}
				va, okA = nextA()
			} else {
				if !yield(vb) {
					return
				}
				vb, okB = nextB()
			}
		}
		for okA {
			if !yield(va) {
				return
			}
			va, okA = nextA()
		}
		for okB {
			if !yield(vb) {
				return
			}
			vb, okB = nextB()
		}
	}
}

// =================================================================================================
// Section 3: The slices Package
// =================================================================================================

/*
## The slices Package

Added in **Go 1.21**, extended with iterator bridges in **1.23**. It replaces almost every
hand-written slice loop, and the whole of `sort` for slices.

	searching     Contains, ContainsFunc, Index, IndexFunc, BinarySearch, BinarySearchFunc
	ordering      Sort, SortFunc, SortStableFunc, IsSorted, IsSortedFunc, Min, Max, MinFunc, MaxFunc
	comparing     Equal, EqualFunc, Compare, CompareFunc
	modifying     Insert, Delete, DeleteFunc, Replace, Reverse, Compact, CompactFunc, Clip, Grow
	copying       Clone, Concat, Repeat (1.23), Sorted, SortedFunc
	iterators     All, Values, Backward, Collect, Sorted, AppendSeq (1.23)
	chunking      Chunk (1.23)

Points that matter:

- **`slices.Sort` uses `pdqsort`** and is both faster and clearer than `sort.Slice`, which needed a
  closure and used reflection internally. Migrate; there is no reason to keep `sort.Slice`.
- **`SortFunc` takes a three-way comparison** returning negative/zero/positive, not a `less` bool.
  Pair it with `cmp.Compare`. `SortFunc` is **not stable**; use `SortStableFunc` when equal elements
  must keep their order.
- **`Delete`, `Insert`, `Replace` and `Compact` modify in place and return a new slice** — you must
  use the result. Since Go 1.22 `Delete` also **zeroes the vacated tail**, which fixes the pointer
  leak that the manual `append(s[:i], s[i+1:]...)` idiom has.
- **`Clip`** trims capacity down to length, releasing the tail of a large backing array.
- `Equal` compares element by element and is far faster than `reflect.DeepEqual`, and — unlike it —
  treats a nil and an empty slice as **equal**.
*/

func m012SlicesPackage() {
	fmt.Println("\n--- Section 3: The slices Package ---")

	s := []int{5, 2, 8, 2, 1}

	// Searching.
	fmt.Printf("  Contains(8)=%t Index(2)=%d IndexFunc(>4)=%d\n",
		slices.Contains(s, 8), slices.Index(s, 2),
		slices.IndexFunc(s, func(n int) bool { return n > 4 }))

	// Ordering.
	sorted := slices.Clone(s)
	slices.Sort(sorted)
	fmt.Printf("  Sort: %v   Min=%d Max=%d IsSorted=%t\n",
		sorted, slices.Min(s), slices.Max(s), slices.IsSorted(sorted))

	// SortFunc takes a three-way comparison, not a less function.
	// "pear" and "kiwi" share a length, so the single-key sort keeps them in input order
	// (pear, kiwi) while the multi-key sort below flips them (kiwi, pear).
	words := []string{"banana", "fig", "apple", "pear", "kiwi"}
	byLength := slices.Clone(words)
	slices.SortFunc(byLength, func(a, b string) int { return cmp.Compare(len(a), len(b)) })
	fmt.Printf("  SortFunc by length: %v\n", byLength)

	// Multi-key ordering with cmp.Or: length first, then alphabetically.
	multi := slices.Clone(words)
	slices.SortFunc(multi, func(a, b string) int {
		return cmp.Or(cmp.Compare(len(a), len(b)), cmp.Compare(a, b))
	})
	fmt.Printf("  SortFunc length then alpha (cmp.Or): %v\n", multi)

	// Modifying: all of these RETURN the new slice.
	dedup := slices.Compact(slices.Sorted(slices.Values(s)))
	fmt.Printf("  Sorted+Compact (dedup): %v\n", dedup)
	fmt.Printf("  Delete(1,3):  %v\n", slices.Delete(slices.Clone(s), 1, 3))
	fmt.Printf("  Insert(2,99): %v\n", slices.Insert(slices.Clone(s), 2, 99))
	fmt.Printf("  DeleteFunc(even): %v\n", slices.DeleteFunc(slices.Clone(s), func(n int) bool {
		return n%2 == 0
	}))
	reversed := slices.Clone(s)
	slices.Reverse(reversed)
	fmt.Printf("  Reverse: %v   Concat: %v\n", reversed, slices.Concat([]int{0}, s))
	fmt.Printf("  Repeat([1,2], 3): %v\n", slices.Repeat([]int{1, 2}, 3))

	// Clip releases a large backing array.
	big := make([]int, 3, 1000)
	fmt.Printf("  Clip: cap %d -> %d\n", cap(big), cap(slices.Clip(big)))

	// Comparing.
	fmt.Printf("  Equal(nil, []int{}) = %t  <- unlike reflect.DeepEqual, which says false\n",
		slices.Equal([]int(nil), []int{}))
	fmt.Printf("  Compare([1,2],[1,3]) = %d\n", slices.Compare([]int{1, 2}, []int{1, 3}))

	// Chunk (Go 1.23) yields sub-slices.
	fmt.Print("  Chunk(s, 2): ")
	for chunk := range slices.Chunk(s, 2) {
		fmt.Printf("%v ", chunk)
	}
	fmt.Println()

	// The iterator bridges.
	fmt.Print("  All (index, value): ")
	for i, v := range slices.All(s) {
		fmt.Printf("%d:%d ", i, v)
	}
	fmt.Println()
	fmt.Print("  Backward: ")
	for _, v := range slices.Backward(s) {
		fmt.Print(v, " ")
	}
	fmt.Println()
}

// =================================================================================================
// Section 4: The maps Package
// =================================================================================================

/*
## The maps Package

Also **Go 1.21**, with the iterator functions promoted in **1.23**.

	Keys(m)      iter.Seq[K]      the keys, in random order
	Values(m)    iter.Seq[V]      the values, in random order
	All(m)       iter.Seq2[K, V]  every pair
	Collect(seq) map[K]V          build a map from an iter.Seq2
	Clone(m)     map[K]V          a shallow copy
	Copy(dst, src)                copy every pair into dst
	Equal(a, b) / EqualFunc       compare
	DeleteFunc(m, pred)           delete every matching pair, in place
	Insert(m, seq)                insert every pair from an iter.Seq2

- **`Keys` and `Values` return iterators, not slices.** This tripped up everyone who had used the
  `golang.org/x/exp/maps` versions, which returned slices. To get a slice, wrap:
  `slices.Collect(maps.Keys(m))`, or better `slices.Sorted(maps.Keys(m))` since map order is random
  anyway.
- **`Clone` is shallow**: the values are copied, but if a value is itself a slice or map, the copy
  shares it (module 006).
- Deleting during a `range` over a map **is** allowed and defined: an entry deleted before it is
  reached will not be produced. Adding during a range is allowed but the new entry may or may not
  appear — so `DeleteFunc` is safe and adding is not.
*/

func m012MapsPackage() {
	fmt.Println("\n--- Section 4: The maps Package ---")

	m := map[string]int{"banana": 3, "apple": 5, "cherry": 1}

	// Keys and Values are ITERATORS, so collect or sort them.
	fmt.Printf("  slices.Sorted(maps.Keys):   %v\n", slices.Sorted(maps.Keys(m)))
	fmt.Printf("  slices.Sorted(maps.Values): %v\n", slices.Sorted(maps.Values(m)))

	// SortedFunc takes an iter.Seq, not a Seq2 - so sort the KEYS with a custom comparison.
	byLen := slices.SortedFunc(maps.Keys(m), func(a, b string) int {
		return cmp.Or(cmp.Compare(len(a), len(b)), cmp.Compare(a, b))
	})
	fmt.Printf("  SortedFunc over maps.Keys, by length: %v\n", byLen)

	// maps.All is an iter.Seq2, so range it directly - note the order is random, which
	// is why the sorted walk below exists at all.
	fmt.Print("  maps.All as an iter.Seq2 (random order): ")
	for k, v := range maps.All(m) {
		fmt.Printf("%s=%d ", k, v)
	}
	fmt.Println()

	fmt.Print("  the same pairs, walked in sorted key order: ")
	for _, k := range slices.Sorted(maps.Keys(m)) {
		fmt.Printf("%s=%d ", k, m[k])
	}
	fmt.Println()

	// Clone and Equal.
	clone := maps.Clone(m)
	fmt.Printf("  Clone then Equal: %t\n", maps.Equal(m, clone))
	clone["durian"] = 9
	fmt.Printf("  after adding to the clone: Equal=%t (len %d vs %d)\n",
		maps.Equal(m, clone), len(m), len(clone))

	// Clone is SHALLOW.
	nested := map[string][]int{"a": {1, 2}}
	shallow := maps.Clone(nested)
	shallow["a"][0] = 99
	fmt.Printf("  Clone is shallow: original now %v\n", nested)

	// DeleteFunc modifies in place.
	scores := maps.Clone(m)
	maps.DeleteFunc(scores, func(_ string, v int) bool { return v < 3 })
	fmt.Printf("  DeleteFunc(v < 3): %v\n", slices.Sorted(maps.Keys(scores)))

	// Copy and Collect.
	dst := map[string]int{"extra": 0}
	maps.Copy(dst, m)
	fmt.Printf("  Copy into an existing map: %d entries\n", len(dst))

	built := maps.Collect(m012Pairs([]string{"x", "y"}, []int{10, 20}))
	fmt.Printf("  maps.Collect from an iter.Seq2: %v\n", built)

	// Deleting during a range is defined; adding is not.
	fmt.Println("  deleting during a range is safe and defined; adding is not")
}

// =================================================================================================
// Section 5: The cmp Package and Ordering
// =================================================================================================

/*
## The cmp Package and Ordering

- **`cmp.Ordered`** is the constraint for `<`: every integer and float type, plus `string`. It is
  what `golang.org/x/exp/constraints.Ordered` used to be, now in the standard library and covered by
  the compatibility promise. **Use `cmp.Ordered`, not the `x/exp` version.**
- **`cmp.Compare(a, b)`** returns -1, 0 or +1 and defines a **total order** — crucially, it places
  `NaN` **before** everything else, so sorting a `[]float64` is well defined even with NaNs, which
  a naive `a < b` comparison is not.
- **`cmp.Less(a, b)`** is the boolean form, with the same NaN handling.
- **`cmp.Or(vals...)`** (Go 1.22) returns the first non-zero argument, or the zero value if all are
  zero. Two uses: defaulting (`cmp.Or(cfg.Host, "localhost")`) and — the neat one — chaining
  comparisons for a multi-key sort.
- Since Go 1.21 **`min` and `max` are builtins**, variadic and usable on any ordered type including
  strings. Do not write your own.
*/

type m012Employee struct {
	Name   string
	Dept   string
	Salary int
}

func m012CmpPackage() {
	fmt.Println("\n--- Section 5: The cmp Package and Ordering ---")

	fmt.Printf("  cmp.Compare: (1,2)=%d (2,2)=%d (3,2)=%d\n",
		cmp.Compare(1, 2), cmp.Compare(2, 2), cmp.Compare(3, 2))
	fmt.Printf("  cmp.Less(\"a\",\"b\")=%t\n", cmp.Less("a", "b"))

	// NaN handling: cmp.Compare gives a total order where `<` does not.
	nan := 0.0 / m012Zero()
	floats := []float64{3, nan, 1, 2}
	slices.SortFunc(floats, cmp.Compare)
	fmt.Printf("  sorting with NaN using cmp.Compare: %v (NaN sorts first)\n", floats)
	fmt.Printf("  a naive `a < b` would be inconsistent: NaN < 1 = %t and NaN > 1 = %t\n",
		nan < 1, nan > 1)

	// cmp.Or for defaults.
	fmt.Printf("  cmp.Or(\"\", \"\", \"localhost\") = %q\n", cmp.Or("", "", "localhost"))
	fmt.Printf("  cmp.Or(0, 8080)              = %d\n", cmp.Or(0, 8080))

	// cmp.Or for a multi-key sort - the idiomatic modern form.
	staff := []m012Employee{
		{"Ada", "eng", 120}, {"Alan", "eng", 150}, {"Grace", "ops", 150}, {"Barbara", "eng", 150},
	}
	slices.SortFunc(staff, func(a, b m012Employee) int {
		return cmp.Or(
			cmp.Compare(a.Dept, b.Dept),      // department ascending
			-cmp.Compare(a.Salary, b.Salary), // then salary DESCENDING
			cmp.Compare(a.Name, b.Name),      // then name ascending
		)
	})
	for _, e := range staff {
		fmt.Printf("    %-8s %-4s %d\n", e.Name, e.Dept, e.Salary)
	}

	// min and max are builtins since Go 1.21, and are variadic.
	fmt.Printf("  builtins: min(3,1,2)=%d max(3,1,2)=%d min(\"b\",\"a\")=%q\n",
		min(3, 1, 2), max(3, 1, 2), min("b", "a"))
}

func m012Zero() float64 { return 0 }

// =================================================================================================
// Section 6: Building Iterator Pipelines, and unique
// =================================================================================================

/*
## Building Iterator Pipelines, and unique

- Because a `Seq` is just a function, an **adapter** is a function taking a `Seq` and returning a
  `Seq`. Adapters compose, and the whole chain stays lazy: nothing is computed until the consumer's
  loop asks for it, and everything stops the moment the consumer breaks.
- This is where `Map`/`Filter`/`Take` finally earn their place in Go. Over a slice they are no
  better than a loop; over an iterator they avoid every intermediate allocation and work over
  infinite or expensive sources.
- **`unique.Make[T](v)`** (Go 1.23) *interns* a comparable value: equal values share one allocation,
  and the returned `unique.Handle[T]` is comparable by pointer, so comparison is O(1) regardless of
  the value's size. Use it for large numbers of repeated strings or structs — a symbol table, parsed
  identifiers, labels on metrics. `handle.Value()` gets the value back.
- **`weak.Pointer[T]`** (Go 1.24) is the companion: a reference the garbage collector may clear,
  for caches that must not keep their contents alive. It is what `unique` uses internally.
- A caution: an iterator is **not** free. Each element costs a closure call, and a deep pipeline
  costs one per stage. For a hot loop over a slice already in memory, a plain `for` is faster. Use
  iterators for composition and for sources that are not slices.
*/

func m012Take[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		if n <= 0 {
			return
		}
		count := 0
		for v := range seq {
			if !yield(v) {
				return
			}
			if count++; count >= n {
				return
			}
		}
	}
}

func m012MapSeq2[K, V, R any](seq iter.Seq2[K, V], f func(K, V) R) iter.Seq[R] {
	return func(yield func(R) bool) {
		for k, v := range seq {
			if !yield(f(k, v)) {
				return
			}
		}
	}
}

// An infinite source, to prove the pipeline is lazy.
func m012Fibonacci() iter.Seq[int] {
	return func(yield func(int) bool) {
		a, b := 0, 1
		for {
			if !yield(a) {
				return
			}
			a, b = b, a+b
		}
	}
}

func m012Pipelines() {
	fmt.Println("\n--- Section 6: Building Iterator Pipelines, and unique ---")

	// A lazy pipeline over an infinite sequence, reusing module 010's adapters.
	evens := m012Take(m010FilterSeq(m012Fibonacci(), func(n int) bool { return n%2 == 0 }), 6)
	fmt.Printf("  first 6 even Fibonacci numbers, from an INFINITE source: %v\n",
		slices.Collect(evens))

	// Composing over a map.
	inventory := map[string]int{"apple": 5, "banana": 0, "cherry": 12}
	inStock := m012MapSeq2(maps.All(inventory), func(k string, v int) string {
		return fmt.Sprintf("%s(%d)", k, v)
	})
	labels := slices.Collect(inStock)
	slices.Sort(labels)
	fmt.Printf("  iter.Seq2 -> iter.Seq -> collect: %v\n", labels)

	// Nothing is materialised until Collect or a range.
	fmt.Print("  a chain that stops early does no extra work: ")
	calls := 0
	counted := func(yield func(int) bool) {
		for i := range 1_000_000 {
			calls++
			if !yield(i) {
				return
			}
		}
	}
	for v := range m012Take(iter.Seq[int](counted), 3) {
		fmt.Print(v, " ")
	}
	fmt.Printf("<- the source produced only %d values, not a million\n", calls)

	// --- unique (Go 1.23) ---
	a := unique.Make("a very long repeated identifier string")
	b := unique.Make("a very long repeated identifier string")
	c := unique.Make("a different one")
	fmt.Printf("  unique.Make: a == b is %t (one allocation, pointer comparison)\n", a == b)
	fmt.Printf("               a == c is %t\n", a == c)
	fmt.Printf("               Value() gets it back: %q\n", a.Value()[:14]+"...")
	fmt.Println("  use it for many repeated strings or structs: symbol tables, metric labels")

	fmt.Println("  a caution: each element costs a closure call, so for a hot loop over a")
	fmt.Println("  slice already in memory, a plain `for` is still faster")
}

// Run012 runs every section of module 012 in order.
func Run012() {
	m012RangeOverFunc()
	m012IterPull()
	m012SlicesPackage()
	m012MapsPackage()
	m012CmpPackage()
	m012Pipelines()
}

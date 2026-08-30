package basics

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"unsafe"
)

/*
# Module 002b — Composite Types

Section numbering continues from module 002a, which ended at Section 7.

Arrays, slices, maps and structs are the four composite types built into the language. Two of them
are pure values (arrays, structs) and two contain a pointer to shared data (slices, maps) — and
almost every surprise in this module comes from that distinction.
*/

// =================================================================================================
// Section 8: Arrays
// =================================================================================================

/*
## Arrays

- An array has a **fixed length that is part of its type**. `[3]int` and `[4]int` are two different
  types, and a function taking a `[3]int` will not accept a `[4]int`. The length must be a constant
  expression, known at compile time.
- An array is a **value**. Assigning it, passing it to a function or returning it **copies every
  element**. This is the opposite of C, where an array decays to a pointer, and it is why arrays are
  rare in Go APIs: pass a slice instead.
- `len(a)` is a compile-time constant for an array. `cap(a)` equals `len(a)`.
- Arrays are **comparable** with `==` whenever their element type is, comparing element by element.
  Slices are not — that asymmetry matters when choosing a map key.
- Literal forms:
    - `[3]int{1, 2, 3}` — all elements
    - `[...]int{1, 2, 3}` — length inferred from the literal
    - `[100]int{1, 10: 3, 99: 100}` — **indexed** elements; everything else is the zero value. The
      length can even be implied by the largest index: `[...]int{99: 1}` has length 100.
- Multidimensional arrays are arrays of arrays: `[2][3]int`, laid out contiguously in memory.
- Use an array when the size is genuinely fixed and part of the meaning — a `[16]byte` UUID, a
  `[8]float64` matrix row, a lookup table indexed by an enum — or when you want a value type with
  no allocation. Otherwise use a slice.
*/

func m002bArrays() {
	fmt.Println("--- Section 8: Arrays ---")

	// The length is part of the type.
	var fixed [3]int
	fmt.Printf("var [3]int zero value: %v, len=%d cap=%d, type=%T\n", fixed, len(fixed), cap(fixed), fixed)

	// Literal forms.
	explicit := [3]string{"a", "b", "c"}
	inferred := [...]string{"a", "b", "c", "d"} // length inferred: 4
	fmt.Printf("explicit %T=%v  inferred %T=%v\n", explicit, explicit, inferred, inferred)

	// Indexed literal: only the named positions are set, the rest are zero.
	sparse := [100]int{1, 10: 3, 99: 100}
	fmt.Printf("sparse len=%d, [0]=%d [1]=%d [10]=%d [99]=%d\n",
		len(sparse), sparse[0], sparse[1], sparse[10], sparse[99])

	// The largest index can even imply the length.
	implied := [...]int{99: 1}
	fmt.Printf("[...]int{99: 1} has length %d\n", len(implied))

	// --- Arrays are values: assignment copies ---
	original := [3]int{1, 2, 3}
	copied := original
	copied[0] = 999
	fmt.Printf("original=%v copied=%v - the whole array was copied\n", original, copied)

	// So does passing one to a function.
	m002bMutateArray(original)
	fmt.Printf("after passing to a function: %v - unchanged\n", original)

	// --- Arrays are comparable ---
	fmt.Printf("[3]int{1,2,3} == [3]int{1,2,3} is %t\n", original == [3]int{1, 2, 3})
	// Different lengths are different TYPES, so this is not even a valid comparison:
	//	fmt.Println(original == [4]int{1, 2, 3, 0}) // ERROR: invalid operation: original == [4]int{…} (mismatched types [3]int and [4]int)

	// Because they are comparable, arrays can be map keys - slices cannot.
	seen := map[[3]int]string{{1, 2, 3}: "the first triple"}
	fmt.Println("an array as a map key:", seen[[3]int{1, 2, 3}])

	// --- Multidimensional ---
	grid := [2][3]int{{1, 2, 3}, {4, 5, 6}}
	for _, row := range grid {
		fmt.Printf("  %v\n", row)
	}

	// Out-of-range indexing with a CONSTANT index is caught at compile time:
	//	fmt.Println(original[5]) // ERROR: invalid argument: index 5 out of bounds [0:3]
	// With a variable index it is a runtime panic instead.
	fmt.Println("a constant out-of-range index is a compile error; a variable one panics at runtime")
}

func m002bMutateArray(a [3]int) { a[0] = -1 } // operates on a copy

// =================================================================================================
// Section 9: Slices — the header, len and cap
// =================================================================================================

/*
## Slices — the header, len and cap

- A slice is a **three-word header**: a pointer to a backing array, a **length** and a **capacity**.
  The slice itself is a value and is copied on assignment — but the *pointer inside it* is copied
  too, so both copies see the same elements. This is the single most important thing to understand
  about Go's data model, and module 006 returns to it.
- `len(s)` is how many elements you may index. `cap(s)` is how many the backing array holds counting
  **from the start of the slice**, so it tells you how far the slice can grow before `append` must
  reallocate.
- Slicing `s[low:high]` produces a new header with `len = high-low` and — critically —
  **`cap = cap(s) - low`**. The capacity extends to the end of the *backing array*, not to `high`.
  So slicing a `[300]int` with `[2:5]` gives `len 3` but `cap 298`.
- That is why a slice can be **re-extended beyond its length**: `s = s[:cap(s)]` is legal and
  reveals elements that were sliced away. It is also why a small slice of a huge array keeps the
  whole array alive — a genuine memory leak. `slices.Clone` breaks the link.
- The **three-index form** `s[low:high:max]` sets the capacity explicitly, to `max-low`. Use it to
  hand out a slice that cannot grow into its neighbour's memory: `s[0:2:2]` forces the next `append`
  to reallocate.
- Indices must satisfy `0 <= low <= high <= cap(s)`. Note `cap`, not `len`: you may slice *past* the
  length, up to the capacity.
- `nil` slice vs **empty** slice: `var s []int` is nil, `s := []int{}` is not. Both have `len 0`,
  both support `range` and `append`, and `len(s) == 0` is the right emptiness test for either. They
  differ only where code looks at nil-ness explicitly: `s == nil`, `reflect.DeepEqual` (which treats
  them as unequal), and `encoding/json`, which marshals a nil slice as `null` and an empty one as
  `[]`. **Prefer the nil slice** as your zero value.
*/

func m002bSliceHeader() {
	fmt.Println("\n--- Section 9: Slices — the header, len and cap ---")

	// A slice literal creates its own backing array.
	s := []int{1, 2, 3, 4, 5}
	fmt.Printf("s=%v len=%d cap=%d\n", s, len(s), cap(s))

	// make(T, len) and make(T, len, cap).
	made := make([]int, 3, 10)
	fmt.Printf("make([]int, 3, 10) = %v len=%d cap=%d\n", made, len(made), cap(made))

	// --- The capacity rule: cap counts to the end of the BACKING ARRAY ---
	backing := make([]int, 300)
	window := backing[2:5]
	fmt.Printf("backing[2:5]: len=%d but cap=%d (300 - 2), not 3\n", len(window), cap(window))
	fmt.Println("  cap extends to the end of the backing array, not to `high`")

	// Which is why a slice can be re-extended past its own length.
	extended := window[:cap(window)]
	fmt.Printf("window[:cap(window)] gives len=%d - the elements were never gone\n", len(extended))

	// --- Sharing: slices of the same array alias each other ---
	base := []int{0, 1, 2, 3, 4}
	left, right := base[:3], base[2:]
	left[2] = 99 // index 2 of `base`, which is index 0 of `right`
	fmt.Printf("base=%v left=%v right=%v - one write, three views\n", base, left, right)

	// slices.Clone breaks the link (and releases the big backing array).
	independent := slices.Clone(left)
	independent[0] = -1
	fmt.Printf("after slices.Clone and a write: base=%v clone=%v\n", base, independent)

	// --- The three-index form caps growth ---
	full := []int{1, 2, 3, 4, 5}
	loose := full[0:2]   // cap 5: appending here overwrites full[2]
	tight := full[0:2:2] // cap 2: appending must reallocate
	fmt.Printf("full[0:2] cap=%d   full[0:2:2] cap=%d\n", cap(loose), cap(tight))
	loose = append(loose, 99)
	fmt.Printf("append to the loose slice clobbered the original: full=%v\n", full)
	tight = append(tight, 77)
	fmt.Printf("append to the capped slice reallocated: full=%v tight=%v\n", full, tight)

	// --- nil vs empty ---
	var nilSlice []int
	emptySlice := []int{}
	fmt.Printf("nil:   %v len=%d isNil=%t\n", nilSlice, len(nilSlice), nilSlice == nil)
	fmt.Printf("empty: %v len=%d isNil=%t\n", emptySlice, len(emptySlice), emptySlice == nil)
	nilJSON, _ := json.Marshal(nilSlice)
	emptyJSON, _ := json.Marshal(emptySlice)
	fmt.Printf("the one place it shows: JSON gives %s vs %s\n", nilJSON, emptyJSON)
	fmt.Println("test emptiness with len(s) == 0, which is correct for both")

	// Slices are NOT comparable, so they cannot be map keys either:
	//	fmt.Println(nilSlice == emptySlice) // ERROR: invalid operation: nilSlice == emptySlice (slice can only be compared to nil)
	fmt.Println("slices compare only against nil; use slices.Equal for element-wise comparison:",
		slices.Equal(nilSlice, emptySlice))
}

// =================================================================================================
// Section 10: append, copy, clear and growth
// =================================================================================================

/*
## append, copy, clear and growth

- `append(s, elems...)` returns a **new slice header** and you must use the result. If the capacity
  suffices it writes in place and returns a header over the same array; if not it allocates a bigger
  array, copies, and returns a header over the *new* one. `go vet` flags a discarded result.
- The consequence is that **`append` sometimes shares and sometimes does not**, depending on
  capacity. That is the trap in the previous section: the same code can mutate a caller's data or
  not, depending on how much room happened to be left.
- Growth is amortised: roughly doubling for small slices, then tapering to about 1.25× for large
  ones. The exact factor is an implementation detail and has changed between releases — never rely
  on it. If you know the final size, `make([]T, 0, n)` and skip the regrowth entirely.
- `append(dst, src...)` concatenates; `append(s[:i], s[i+1:]...)` deletes element `i`. Since Go 1.21
  prefer `slices.Delete`, `slices.Insert` and `slices.Concat`, which are clearer and handle the
  edge cases.
- `copy(dst, src)` copies `min(len(dst), len(src))` elements and returns that count. It handles
  overlapping slices correctly. Note it is bounded by the **length**, not the capacity — copying
  into `make([]int, 0, 10)` copies nothing, a common mistake.
- `clear(s)` (Go 1.21) sets every element of a slice to its zero value, keeping the length. On a map
  it deletes every key instead. Use it to drop references so the garbage collector can work.
- Removing an element from a slice of pointers leaves the tail element still pointing at the object.
  Zero the vacated slot, or use `slices.Delete`, which does it for you.
*/

func m002bAppendCopyClear() {
	fmt.Println("\n--- Section 10: append, copy, clear and growth ---")

	// append returns a new header; you must use it.
	s := []int{1, 2, 3}
	s = append(s, 4, 5)
	fmt.Printf("after append: %v len=%d cap=%d\n", s, len(s), cap(s))
	//	append(s, 6) // ERROR: append(s, 6) (value of type []int) is not used

	// Spreading another slice.
	s = append(s, []int{6, 7}...)
	fmt.Printf("append(s, other...) = %v\n", s)

	// A string can be spread into a []byte.
	bytes := append([]byte("Go"), "lang"...)
	fmt.Printf("append([]byte, string...) = %q\n", bytes)

	// --- Growth ---
	fmt.Println("capacity growth while appending 1..10 to an empty slice:")
	var grow []int
	prev := -1
	for i := range 10 {
		grow = append(grow, i)
		if cap(grow) != prev {
			fmt.Printf("  len=%2d cap=%2d  <- reallocated\n", len(grow), cap(grow))
			prev = cap(grow)
		}
	}
	fmt.Println("  the exact factor is an implementation detail - never depend on it")

	// Preallocating avoids every reallocation.
	sized := make([]int, 0, 10)
	for i := range 10 {
		sized = append(sized, i)
	}
	fmt.Printf("make([]int, 0, 10) then 10 appends: cap stayed %d\n", cap(sized))

	// --- copy is bounded by LENGTH, not capacity ---
	dst := make([]int, 3)
	n := copy(dst, []int{1, 2, 3, 4, 5})
	fmt.Printf("copy into len-3 dst: copied %d elements -> %v\n", n, dst)

	empty := make([]int, 0, 10) // len 0, cap 10
	n = copy(empty, []int{1, 2, 3})
	fmt.Printf("copy into len-0 cap-10 dst: copied %d elements -> %v (a classic mistake)\n", n, empty)

	// --- Insert and delete ---
	letters := []string{"a", "b", "c", "d"}
	letters = slices.Delete(letters, 1, 2) // remove index 1
	fmt.Printf("slices.Delete(s, 1, 2) = %v\n", letters)
	letters = slices.Insert(letters, 1, "B")
	fmt.Printf("slices.Insert(s, 1, \"B\") = %v\n", letters)
	fmt.Printf("slices.Concat = %v\n", slices.Concat(letters, []string{"e", "f"}))

	// The manual idiom, for comparison - and its pointer-leak hazard.
	manual := []string{"a", "b", "c", "d"}
	manual = append(manual[:1], manual[2:]...)
	fmt.Printf("append(s[:1], s[2:]...) = %v (with a slice of pointers this leaks the tail)\n", manual)

	// --- clear ---
	nums := []int{1, 2, 3}
	clear(nums)
	fmt.Printf("clear on a slice zeroes it, keeping the length: %v len=%d\n", nums, len(nums))
}

// =================================================================================================
// Section 11: Maps
// =================================================================================================

/*
## Maps

- A map is an unordered collection of key/value pairs, written `map[K]V`. Under the hood it is a
  **pointer to a runtime hash-table structure**, so copying a map value shares the table — a map is
  never deep-copied by assignment.
- The **key type must be comparable**: it must support `==`. Booleans, numbers, strings, pointers,
  channels, interfaces, and arrays and structs built only from those, all qualify. **Slices, maps
  and functions do not**, so they cannot be keys.
    - A float key is legal but a bad idea: `NaN != NaN`, so a NaN key can never be found again.
    - An interface key is legal but panics at runtime if the dynamic value turns out to be
      uncomparable.
- The zero value is a **nil map**: readable (yielding zero values) but **writing to it panics**.
  Always create one with `make(map[K]V)` or a literal before assigning.
- Reading a missing key returns the value type's **zero value**, not an error. The two-value
  **comma-ok** form `v, ok := m[k]` is how you distinguish "missing" from "present but zero".
- `delete(m, k)` is a no-op if the key is absent. `clear(m)` (Go 1.21) removes every key.
- **Iteration order is deliberately randomised**, and it is re-randomised on every `range`. This is
  not an implementation accident — the runtime does it on purpose so that nobody can depend on the
  order. To iterate in order, collect the keys and sort them, or use `slices.Sorted(maps.Keys(m))`.
- A map entry is **not addressable**: `&m[k]` does not compile, and neither does `m[k].Field = v`
  when the value is a struct. Store a *pointer* to the struct, or read-modify-write the whole value.
- Maps are **not safe for concurrent use**. Concurrent access is detected by the runtime and
  aborts the program with `fatal error: concurrent map read and map write` (or `concurrent map
  writes` for two writers) — which is not a recoverable panic. Use a mutex, or `sync.Map` for its
  specific workloads: entries written once and read many times, or goroutines that each touch a
  disjoint set of keys (module 011).
- Since Go 1.24 maps use a **Swiss-table** implementation: faster lookups and less memory. No API
  changed; only the constant factors did.
*/

func m002bMaps() {
	fmt.Println("\n--- Section 11: Maps ---")

	// Literal and make.
	ages := map[string]int{"Ada": 36, "Alan": 41, "Grace": 45}
	counts := make(map[string]int)
	counts["seen"]++ // reading a missing key gives 0, so ++ works on a fresh key
	fmt.Printf("ages=%v counts=%v\n", ages, counts)

	// --- The nil map ---
	var nilMap map[string]int
	fmt.Printf("nil map: %v len=%d, reading gives %d\n", nilMap, len(nilMap), nilMap["anything"])
	//	nilMap["k"] = 1 // panic: assignment to entry in nil map
	fmt.Println("reading a nil map is fine; writing to it panics")

	// --- comma-ok tells missing from zero ---
	ages["Zero"] = 0
	for _, key := range []string{"Ada", "Zero", "Nobody"} {
		v, ok := ages[key]
		fmt.Printf("  ages[%q] = %d, present=%t\n", key, v, ok)
	}
	fmt.Println("without comma-ok, a missing key and a zero value look identical")

	// delete and clear.
	delete(ages, "Zero")
	delete(ages, "Not there") // a no-op, not an error
	fmt.Printf("after delete: %v\n", ages)

	// --- Iteration order is randomised on purpose ---
	fmt.Println("three separate ranges over the same map:")
	for range 3 {
		var order []string
		for k := range ages {
			order = append(order, k)
		}
		fmt.Printf("  %v\n", order)
	}
	fmt.Println("to iterate deterministically, sort the keys:")
	for _, k := range slices.Sorted(maps.Keys(ages)) {
		fmt.Printf("  %s=%d\n", k, ages[k])
	}

	// maps package helpers (Go 1.21+).
	cloned := maps.Clone(ages)
	fmt.Printf("maps.Clone equal to the original? %t\n", maps.Equal(ages, cloned))

	// --- Map entries are not addressable ---
	type counter struct{ N int }
	structs := map[string]counter{"a": {N: 1}}
	//	structs["a"].N++ // ERROR: cannot assign to struct field structs["a"].N in map
	entry := structs["a"] // read
	entry.N++             // modify
	structs["a"] = entry  // write back
	fmt.Printf("read-modify-write a struct value: %v\n", structs)

	pointers := map[string]*counter{"a": {N: 1}}
	pointers["a"].N++ // fine: the map value is a pointer, and we are not assigning to the entry
	fmt.Printf("or store pointers instead: N=%d\n", pointers["a"].N)

	// --- Uncomparable key types ---
	//	bad := map[[]int]string{} // ERROR: invalid map key type []int
	fmt.Println("slices, maps and funcs cannot be map keys; arrays and comparable structs can")
	byPoint := map[m002aPoint]string{{X: 1, Y: 2}: "a comparable struct works as a key"}
	fmt.Println(" ", byPoint[m002aPoint{X: 1, Y: 2}])
}

// =================================================================================================
// Section 12: Structs
// =================================================================================================

/*
## Structs

- A struct is a fixed sequence of named, typed fields. It is a **value**: assignment and argument
  passing copy every field. There is no inheritance — composition and embedding take its place
  (module 007).
- Field visibility follows the usual rule: an **uppercase** field name is exported, a lowercase one
  is package-private. This matters enormously for `encoding/json` and friends, which use reflection
  and therefore **cannot see unexported fields at all**.
- Literal forms:
    - **keyed** `Point{X: 1, Y: 2}` — omitted fields take their zero value. Always prefer this: it
      survives fields being added or reordered.
    - **positional** `Point{1, 2}` — must list *every* field, in declaration order. It breaks
      silently when the struct changes, and `go vet` warns about it for structs from other packages.
- **Go 1.27**: a key in a struct literal may now be **any valid field selector**, including a
  promoted field from an embedded struct — `Line{name: "diagonal"}` where `name` comes from an
  embedded `Object`. Previously only top-level field names were allowed. The selector may not
  traverse a **pointer**-typed embedded field, though: there is no struct to initialise behind a
  nil pointer.
- Structs are **comparable** with `==` if and only if every field is. Comparing structs containing
  a slice or map is a compile error; comparing an interface field that holds an uncomparable value
  panics at runtime.
- An **empty struct** `struct{}` occupies **zero bytes**. `map[string]struct{}` is the idiomatic
  set, and `chan struct{}` the idiomatic signal-only channel.
- **Anonymous structs** — `struct{ Name string }{Name: "x"}` — are useful for table-driven test
  cases and one-off JSON payloads, where naming the type would add nothing.
- **Struct tags** are raw string literals attached to a field and read at runtime by reflection.
  `json:"name,omitempty"` is the common case. They are just strings to the compiler: a typo is
  silent, which is why `go vet`'s `structtag` check exists.
- Field **order affects size** because of alignment padding. Grouping fields largest-first can
  shrink a struct measurably; `fieldalignment` in `go vet -vettool` reports it. Do this only where
  it matters — millions of instances, or a hot cache line.
*/

type m002bObject struct {
	Name  string
	Color string
}

// m002bLine embeds m002bObject, so Name and Color are promoted into it.
type m002bLine struct {
	m002bObject
	Length float64
}

type m002bUser struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Nickname string `json:"nickname,omitzero"` // Go 1.24: omit when the field is the zero value
	password string // unexported: invisible to encoding/json
}

func m002bStructs() {
	fmt.Println("\n--- Section 12: Structs ---")

	// Keyed literal - the form to prefer.
	u := m002bUser{ID: 1, Name: "Ada", password: "secret"}
	fmt.Printf("%+v\n", u)

	// Positional literal must list every field, in order.
	full := m002bUser{2, "Alan", "alan@example.com", "", "hunter2"}
	fmt.Printf("positional: %+v\n", full)
	//	partial := m002bUser{3, "Grace"} // ERROR: too few values in struct literal of type m002bUser

	// Unexported fields are invisible to encoding/json - and so is an empty omitempty/omitzero field.
	encoded, _ := json.Marshal(u)
	fmt.Printf("json.Marshal: %s\n", encoded)
	fmt.Println("  `password` is absent because it is unexported, not because of a tag")

	// --- Go 1.27: a struct-literal key may be a promoted field ---
	line := m002bLine{Name: "diagonal", Length: 5}
	fmt.Printf("Go 1.27 promoted-field key: %+v (Name reached through the embedded struct)\n", line)
	// Before 1.27 this had to be written out in full:
	classic := m002bLine{m002bObject: m002bObject{Name: "diagonal"}, Length: 5}
	fmt.Printf("the pre-1.27 spelling: %+v\n", classic)

	// --- Value semantics ---
	a := m002bObject{Name: "original"}
	b := a
	b.Name = "copy"
	fmt.Printf("a=%v b=%v - assignment copied every field\n", a, b)

	// --- Comparability ---
	fmt.Printf("structs compare field by field: %t\n", a == m002bObject{Name: "original"})
	type withSlice struct{ S []int }
	//	fmt.Println(withSlice{} == withSlice{}) // ERROR: invalid operation: withSlice{} == withSlice{} (struct containing []int cannot be compared)
	_ = withSlice{}
	fmt.Println("a struct containing a slice or map is not comparable at all")

	// --- The empty struct costs nothing ---
	set := map[string]struct{}{"a": {}, "b": {}}
	_, inSet := set["a"]
	fmt.Printf("map[string]struct{} as a set: \"a\" present=%t, and struct{} occupies %d bytes\n",
		inSet, unsafe.Sizeof(struct{}{}))

	// --- Anonymous structs ---
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"abc", 3},
	}
	for _, c := range cases {
		fmt.Printf("  anonymous struct case: len(%q) == %d is %t\n", c.in, c.want, len(c.in) == c.want)
	}
}

// =================================================================================================
// Section 13: Defined Types, Aliases and Generic Aliases
// =================================================================================================

/*
## Defined Types, Aliases and Generic Aliases

Go has **two** things that look similar and behave completely differently.

- **Type definition** — `type Celsius float64`. This creates a **new, distinct type** with the same
  underlying representation. It is not assignable to `float64` without a conversion, and — the whole
  point — it **can carry methods**. This is how you get a domain type out of a primitive: a
  `Celsius` cannot be accidentally added to a `Fahrenheit`.
- **Type alias** — `type Temperature = float64`, with an `=`. This creates **another name for the
  same type**. It is completely interchangeable with the original, it cannot have its own methods,
  and `%T` prints the original name. Aliases exist for gradual code repair — moving a type between
  packages while keeping the old name working — not for making code read nicer.
- A defined type does **not** inherit the original's methods, only its underlying type and therefore
  its operators. `type MyString string` still supports `+` and indexing, but loses nothing else
  because `string` has no methods to lose. Define a type from a *struct* type and its methods are
  indeed left behind.
- Conversion between a defined type and its underlying type is always allowed and free at runtime:
  `float64(c)`, `Celsius(f)`.
- **Go 1.24: aliases may have type parameters** — `type Set[T comparable] = map[T]struct{}`. This
  completes a long-standing gap: before 1.24 a generic type could be defined but not aliased.
- Practical guidance: reach for a **defined type** whenever a bare `string` or `int` is carrying
  meaning that the compiler could be checking for you — user IDs, currency codes, file paths,
  units. Reach for an **alias** essentially only when refactoring across packages.
*/

// Defined types: distinct, and able to carry methods.
type m002bCelsius float64
type m002bFahrenheit float64
type m002bPath string

// Aliases: other names for existing types. No methods, no distinction.
type m002bText = string
type m002bTemperature = float64

// Go 1.24: a generic alias.
type m002bSet[T comparable] = map[T]struct{}

func (c m002bCelsius) String() string { return fmt.Sprintf("%.1f°C", float64(c)) }

func (c m002bCelsius) ToFahrenheit() m002bFahrenheit { return m002bFahrenheit(c*9/5 + 32) }

func (p m002bPath) Base() string {
	if i := strings.LastIndex(string(p), "/"); i >= 0 {
		return string(p)[i+1:]
	}
	return string(p)
}

func m002bDefinedTypesAndAliases() {
	fmt.Println("\n--- Section 13: Defined Types, Aliases and Generic Aliases ---")

	// --- A defined type is distinct ---
	body := m002bCelsius(36.6)
	fmt.Printf("body=%v (%T) -> %.2f (%T)\n", body, body, float64(body.ToFahrenheit()), body.ToFahrenheit())

	// The compiler now catches a unit mix-up that a bare float64 would have allowed:
	//	var f m002bFahrenheit = body // ERROR: cannot use body (variable of type m002bCelsius) as m002bFahrenheit value in variable declaration
	//	sum := body + 5.0            // fine: 5.0 is an untyped constant and adopts m002bCelsius
	//	bad := body + f              // ERROR: invalid operation: body + f (mismatched types m002bCelsius and m002bFahrenheit)
	fmt.Println("mixing Celsius and Fahrenheit is now a compile error, not a Mars-lander bug")

	// Conversion is explicit and free.
	asFloat := float64(body)
	fmt.Printf("float64(body) = %v (%T) - conversion required, costs nothing at runtime\n", asFloat, asFloat)

	// Methods work because m002bPath is DEFINED, not aliased.
	p := m002bPath("/usr/local/bin/go")
	fmt.Printf("m002bPath(%q).Base() = %q\n", string(p), p.Base())

	// --- An alias is the same type ---
	var t m002bText = "an alias is interchangeable with its target"
	var s string = t // no conversion needed: they ARE the same type
	fmt.Printf("%s (%%T prints the original: %T)\n", s, t)

	var temp m002bTemperature = 21.5
	var plain float64 = temp
	fmt.Printf("m002bTemperature and float64 are one type: %v %v\n", temp, plain)

	// An alias cannot carry methods:
	//	func (t m002bText) Shout() string { return t } // ERROR: cannot define new methods on non-local type string

	// --- Go 1.24: generic aliases ---
	seen := m002bSet[string]{"a": {}, "b": {}}
	_, ok := seen["a"]
	fmt.Printf("m002bSet[string] (a Go 1.24 generic alias) = %v, contains \"a\"=%t\n", seen, ok)
	// It is genuinely the same type as the map it aliases:
	var asMap map[string]struct{} = seen
	fmt.Printf("assignable to map[string]struct{} with no conversion: len=%d\n", len(asMap))
}

// Run002b runs every section of module 002b in order.
func Run002b() {
	m002bArrays()
	m002bSliceHeader()
	m002bAppendCopyClear()
	m002bMaps()
	m002bStructs()
	m002bDefinedTypesAndAliases()
}

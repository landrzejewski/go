package basics

import (
	"fmt"
	"runtime"
	"structs"
	"unsafe"
)

/*
# Module 006 — Pointers and Memory

This module is the one that decides whether the rest of Go makes sense. Almost every "why did my
change not stick?" question in Go traces back to a single sentence:

	Go has NO reference types. Every assignment and every argument pass copies a value.

What varies is *what the value is*. Copying an `int` copies the number. Copying a slice copies a
three-word header that still points at the same elements. Copying a map copies a pointer to a
runtime hash table. Nothing is special-cased by the language — the difference is entirely in what
the type happens to contain.
*/

// =================================================================================================
// Section 1: Pointers
// =================================================================================================

/*
## Pointers

- `&x` takes the address of `x`; `*p` dereferences `p`. `*T` is the type "pointer to T".
- The zero value of a pointer is `nil`. Dereferencing nil panics with
  `runtime error: invalid memory address or nil pointer dereference`.
- **There is no pointer arithmetic.** You cannot write `p++` or `p + 1`. This single restriction is
  what lets Go have a precise garbage collector and stay memory-safe without a borrow checker.
  The `unsafe` package can do it, at the cost of leaving the safe language (Section 6).
- Pointers are compared with `==` / `!=` (same address or not) and cannot be ordered.
- **Taking the address of a local is safe.** Go performs *escape analysis*: if a local outlives its
  function, the compiler allocates it on the heap automatically. `return &localVar` is idiomatic Go
  and would be a dangling pointer in C.
- **Not everything is addressable.** You can take the address of a variable, a slice element, a
  pointer dereference, and a field of an addressable struct. You **cannot** take the address of a
  map entry, a string index, a constant, or a function's return value directly.
- `new(T)` allocates a zeroed `T` and returns a `*T`. It is rarely used: `&T{}` is the same thing
  with the option of setting fields, and is what almost all Go code writes.
- Go **automatically dereferences** for field access and method calls: `p.Field` means `(*p).Field`,
  and `p.Method()` works whether `p` is a `T` or a `*T` as long as the method set allows it
  (module 007). This is why explicit `*` is rare in idiomatic Go.
*/

type m006Node struct {
	Value int
	Next  *m006Node
}

func m006Pointers() {
	fmt.Println("--- Section 1: Pointers ---")

	x := 42
	p := &x
	fmt.Printf("x=%d  p=%p  *p=%d\n", x, p, *p)

	*p = 99 // write through the pointer
	fmt.Printf("after *p = 99: x=%d\n", x)

	// Two pointers to the same variable are equal.
	q := &x
	fmt.Printf("p == q: %t (same address)   p == &x: %t\n", p == q, p == &x)

	// The zero value is nil.
	var nilPtr *int
	fmt.Printf("var p *int -> %v, isNil=%t\n", nilPtr, nilPtr == nil)
	//	fmt.Println(*nilPtr) // panic: runtime error: invalid memory address or nil pointer dereference

	// --- No pointer arithmetic ---
	//	p++       // ERROR: invalid operation: p++ (non-numeric type *int)
	//	p = p + 1 // ERROR: invalid operation: p + 1 (mismatched types *int and untyped int)
	//	fmt.Println(p < q) // ERROR: invalid operation: p < q (operator < not defined on pointer)
	fmt.Println("no pointer arithmetic and no pointer ordering - only == and !=")

	// --- Returning the address of a local is safe: escape analysis moves it to the heap ---
	escaped := m006MakeLocal()
	fmt.Printf("returned &local: %d (the compiler heap-allocated it; this would dangle in C)\n", *escaped)

	// --- new vs &T{} ---
	viaNew := new(m006Node)
	viaLiteral := &m006Node{Value: 7}
	fmt.Printf("new(T) gives a zeroed *T: %+v\n", *viaNew)
	fmt.Printf("&T{...} is the same but can set fields: %+v  <- prefer this\n", *viaLiteral)

	// --- Automatic dereference ---
	viaLiteral.Value = 8 // means (*viaLiteral).Value
	fmt.Printf("p.Field is shorthand for (*p).Field: %d == %d\n", viaLiteral.Value, (*viaLiteral).Value)

	// --- Addressability ---
	s := []int{1, 2, 3}
	fmt.Printf("a slice element IS addressable: &s[0] = %p\n", &s[0])
	m := map[string]int{"a": 1}
	//	fmt.Println(&m["a"]) // ERROR: invalid operation: cannot take address of m["a"] (map index expression of type int)
	//	fmt.Println(&"text") // ERROR: invalid operation: cannot take address of "text" (untyped string constant)
	//	fmt.Println(&m006MakeLocal()) // ERROR: invalid operation: cannot take address of m006MakeLocal() (value of type *int)
	_ = m
	fmt.Println("map entries, constants and call results are not addressable")

	// A linked list - the classic reason to want pointers at all.
	list := &m006Node{Value: 1, Next: &m006Node{Value: 2, Next: &m006Node{Value: 3}}}
	fmt.Print("linked list: ")
	for n := list; n != nil; n = n.Next {
		fmt.Print(n.Value, " ")
	}
	fmt.Println()
}

func m006MakeLocal() *int {
	local := 7
	return &local // safe in Go: escape analysis promotes `local` to the heap
}

// =================================================================================================
// Section 2: Value Semantics — what copying actually copies
// =================================================================================================

/*
## Value Semantics — what copying actually copies

Every type in Go is copied on assignment. The question is only **how big the copy is** and **what
the copy points at**. This table is the whole of Go's data model:

	type          size of the copy            does the copy share data?
	int, float    the number                  no
	array [N]T    all N elements              no
	struct        all fields, recursively     no (unless a field is one of the below)
	string        pointer + length (2 words)  yes, but the bytes are immutable, so it is invisible
	slice         pointer + len + cap         YES — elements are shared
	map           one pointer                 YES — the whole table is shared
	channel       one pointer                 YES
	func          one pointer (+ closure)     YES
	pointer       one address                 YES — by definition
	interface     type word + data word       depends on what it holds

- So a **struct containing a slice** is "half shared": copying it copies the header, and both copies
  then index the same elements. This is a genuine source of bugs, and the reason `slices.Clone`
  exists.
- **`append` is the discontinuity.** Writing `s[0] = x` always reaches the shared array. Calling
  `append` may write into the shared array *or* allocate a new one, depending on spare capacity, so
  the caller may or may not see the result. Never rely on it: return the appended slice.
- The practical rules that fall out of this:
    - to let a function modify a struct, pass `*T`
    - to let a function modify slice **elements**, pass the slice — that already works
    - to let a function **grow** a slice, return the new slice (or pass `*[]T`, which is rare)
    - a map argument is always "shared"; there is no way to pass one by value
*/

type m006Config struct {
	Name  string
	Tags  []string          // a header: shared on copy
	Meta  map[string]string // a pointer: shared on copy
	Count int               // a number: copied
}

func m006ValueSemantics() {
	fmt.Println("\n--- Section 2: Value Semantics — what copying actually copies ---")

	// --- A plain struct copies completely ---
	type point struct{ X, Y int }
	a := point{1, 2}
	b := a
	b.X = 99
	fmt.Printf("plain struct: a=%v b=%v (fully independent)\n", a, b)

	// --- An array copies completely; a slice does not ---
	arr1 := [3]int{1, 2, 3}
	arr2 := arr1
	arr2[0] = 99
	fmt.Printf("array:  arr1=%v arr2=%v (independent)\n", arr1, arr2)

	sl1 := []int{1, 2, 3}
	sl2 := sl1
	sl2[0] = 99
	fmt.Printf("slice:  sl1=%v sl2=%v (SHARED elements)\n", sl1, sl2)

	// --- A struct containing a slice and a map is half shared ---
	orig := m006Config{
		Name:  "original",
		Tags:  []string{"a", "b"},
		Meta:  map[string]string{"k": "v"},
		Count: 1,
	}
	copied := orig
	copied.Name = "copy"        // independent: a string field
	copied.Count = 2            // independent: an int field
	copied.Tags[0] = "MODIFIED" // SHARED: reaches orig.Tags too
	copied.Meta["k"] = "MODIFIED"
	fmt.Printf("orig:   %+v\n", orig)
	fmt.Printf("copied: %+v\n", copied)
	fmt.Println("  Name and Count diverged; Tags and Meta did not - the copy shares them")

	// --- The append discontinuity ---
	fmt.Println("the same function, two different outcomes, depending only on spare capacity:")
	noRoom := []int{1, 2, 3} // len 3, cap 3
	m006AppendInPlace(noRoom)
	fmt.Printf("  cap == len, so append reallocated: caller still sees %v\n", noRoom)

	room := make([]int, 3, 10) // len 3, cap 10
	copy(room, []int{1, 2, 3})
	m006AppendInPlace(room)
	fmt.Printf("  spare capacity, so append wrote in place: caller sees %v (len is still %d!)\n",
		room[:cap(room)][:4], len(room))
	fmt.Println("  never rely on this - return the appended slice instead")

	// The correct signatures.
	grown := m006AppendCorrectly([]int{1, 2, 3})
	fmt.Printf("  returning the slice always works: %v\n", grown)

	var viaPointer = []int{1, 2, 3}
	m006AppendViaPointer(&viaPointer)
	fmt.Printf("  or take a *[]T (rare, but unambiguous): %v\n", viaPointer)
}

func m006AppendInPlace(s []int)         { s = append(s, 99); _ = s }
func m006AppendCorrectly(s []int) []int { return append(s, 99) }
func m006AppendViaPointer(s *[]int)     { *s = append(*s, 99) }

// =================================================================================================
// Section 3: Pointer or Value? — choosing a parameter type
// =================================================================================================

/*
## Pointer or Value? — choosing a parameter type

The decision is about **semantics first, performance second**.

Pass (or return) a **pointer** when:

  - the function must **modify** the caller's value
  - the type is **large** and copying would be measurable — but note "large" means hundreds of bytes;
    copying a 3-field struct is cheaper than the indirection a pointer costs
  - the type **must not be copied**: anything embedding `sync.Mutex`, `sync.WaitGroup`, `strings.Builder`
    or `atomic.Int64`. `go vet`'s `copylocks` check catches these, and it is one of the most valuable
    checks in the toolchain
  - you need a **nil** to mean "absent", distinct from the zero value
  - the type already has **pointer-receiver methods**, so its method set requires a pointer (module 007)

Pass a **value** when:

  - the type is small (a few words) — this covers most structs in practice
  - you want **immutability at the call site**: a value parameter cannot surprise the caller
  - the type is naturally a value: `time.Time`, `net/netip.Addr`, a coordinate, an ID

Do **not** pass a pointer to a slice, map, channel or function to "avoid copying" — the copy is
already just a header or a word, and `*[]T` makes every use site worse. The only reason to take a
`*[]T` is to reassign the caller's slice variable itself.

**Consistency matters more than either rule**: if any method of a type takes a pointer receiver,
give them all pointer receivers, and pass the type as a pointer throughout.
*/

type m006Counter struct{ n int }

func (c m006Counter) IncByValue()    { c.n++ }
func (c *m006Counter) IncByPointer() { c.n++ }

type m006Small struct{ X, Y, Z int }
type m006Large struct{ Data [4096]byte }

func m006PointerOrValue() {
	fmt.Println("\n--- Section 3: Pointer or Value? — choosing a parameter type ---")

	// --- Modification requires a pointer ---
	c := m006Counter{}
	c.IncByValue()
	c.IncByValue()
	fmt.Printf("after two IncByValue():   n=%d (each modified a copy)\n", c.n)
	c.IncByPointer()
	c.IncByPointer()
	fmt.Printf("after two IncByPointer(): n=%d\n", c.n)

	// Go auto-takes the address for you: c.IncByPointer() is (&c).IncByPointer(),
	// but only because `c` is addressable.
	fmt.Println("`c.IncByPointer()` is shorthand for `(&c).IncByPointer()` - c must be addressable")

	// --- Size ---
	fmt.Printf("sizeof(m006Small)=%d bytes  sizeof(*m006Small)=%d bytes\n",
		unsafe.Sizeof(m006Small{}), unsafe.Sizeof((*m006Small)(nil)))
	fmt.Printf("sizeof(m006Large)=%d bytes - here a pointer is clearly worth it\n",
		unsafe.Sizeof(m006Large{}))
	fmt.Println("  a 3-field struct is cheaper to copy than to chase through a pointer")

	// --- Types that must not be copied ---
	// A struct embedding a sync.Mutex must always be passed by pointer:
	//	func lockAndUse(m m006Guarded) {} // go vet: passes lock by value: m006Guarded contains sync.Mutex
	fmt.Println("`go vet`'s copylocks check catches a sync.Mutex copied by value - always heed it")

	// --- nil as "absent" ---
	fmt.Println("optional field via a pointer:", m006Describe(nil), "|", m006Describe(m006IntPtr(0)))

	// --- Do not do this ---
	//	func process(s *[]int) {} // pointless: a slice header is already only 3 words
	fmt.Println("never take a *[]T, *map or *chan just to 'avoid a copy' - the copy is a header")
}

func m006IntPtr(n int) *int { return &n }

func m006Describe(p *int) string {
	if p == nil {
		return "absent"
	}
	return fmt.Sprintf("present, value %d (distinct from absent even when it is zero)", *p)
}

// =================================================================================================
// Section 4: Stack, Heap and Escape Analysis
// =================================================================================================

/*
## Stack, Heap and Escape Analysis

- Go gives you **no control** over where a value lives. There is no `new` that means "heap" and no
  `alloca`. The compiler decides, and the decision is called **escape analysis**.
- The rule the compiler applies: if it cannot prove a value stops being referenced when the function
  returns, the value **escapes** and is heap-allocated. Otherwise it stays on the stack, where
  allocation costs nothing and deallocation is a stack-pointer adjustment.
- Common reasons to escape: returning `&local`; storing a pointer into a global, a slice, a map or a
  channel; passing to a function the compiler cannot see through; assigning to an `interface{}`
  (which is why `fmt.Println(x)` allocates); capturing in a closure that outlives the frame; a size
  not known at compile time (`make([]byte, n)`).
- **You can see the decisions**: `go build -gcflags='-m'` prints them, and `-m -m` explains why.
  This is the single most useful optimisation tool in Go, and it needs no profiler.
- **Goroutine stacks start small (2 KB) and grow** by copying to a bigger stack, up to 1 GB by
  default on 64-bit platforms (250 MB on 32-bit). That is why deep recursion usually works, and why
  passing large structs by value is not
  the disaster it would be with a fixed 1 MB stack.
- Because stacks move, **a pointer's numeric address is not stable**. Never store one as an integer
  and expect it to remain valid; that is exactly what `uintptr` cannot promise.
- The garbage collector is **concurrent, tri-colour, mark-and-sweep**, and is not generational or
  compacting. It is tuned for low pause times, not throughput. `GOGC` sets the growth target and
  `GOMEMLIMIT` (Go 1.19) sets a soft memory ceiling — the pair is what makes Go usable in containers.
  Go 1.25 introduced the **Green Tea** garbage collector, which improves scanning locality; it
  was opt-in via `GOEXPERIMENT=greenteagc` there and became the default in Go 1.26.
- The optimisation that actually matters is **reducing allocation count**, not size: preallocate
  with `make([]T, 0, n)`, reuse buffers with `sync.Pool`, and avoid boxing into `any` in hot paths.
  Measure with `go test -bench . -benchmem` before changing anything.
*/

func m006EscapeAnalysis() {
	fmt.Println("\n--- Section 4: Stack, Heap and Escape Analysis ---")

	fmt.Println("run this to see the compiler's decisions:")
	fmt.Println("  go build -gcflags='-m' ./basics/   (add a second -m for the reasoning)")
	fmt.Println("expected output includes lines such as:")
	fmt.Println("  ./mod_006_pointers_and_memory.go: moved to heap: local")
	fmt.Println("  ./mod_006_pointers_and_memory.go: ... escapes to heap")

	// This one escapes: its address is returned.
	escapes := m006MakeLocal()
	// This one does not: it never leaves the frame.
	stays := m006StaysOnStack()
	fmt.Printf("escaping local -> %d, non-escaping local -> %d\n", *escapes, stays)

	// --- Allocation statistics ---
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)

	// One allocation of the right size...
	preallocated := make([]int, 0, 1000)
	for i := range 1000 {
		preallocated = append(preallocated, i)
	}

	runtime.ReadMemStats(&after)
	preallocCount := after.Mallocs - before.Mallocs

	runtime.ReadMemStats(&before)
	// ...versus regrowing from nil.
	var regrown []int
	for i := range 1000 {
		regrown = append(regrown, i)
	}
	runtime.ReadMemStats(&after)
	regrowCount := after.Mallocs - before.Mallocs

	fmt.Printf("appending 1000 ints, preallocated: ~%d allocations\n", preallocCount)
	fmt.Printf("appending 1000 ints, from nil:     ~%d allocations\n", regrowCount)
	fmt.Println("  reducing the allocation COUNT is what matters; measure with -benchmem")

	// Stack and GC facts.
	fmt.Printf("goroutine stacks start at 2 KB and grow to a %d-byte limit by default on 64-bit platforms\n",
		int64(1)<<30)
	fmt.Printf("GOMAXPROCS=%d  NumGoroutine=%d  NumCPU=%d\n",
		runtime.GOMAXPROCS(0), runtime.NumGoroutine(), runtime.NumCPU())
	fmt.Println("GOGC tunes the GC growth target; GOMEMLIMIT (Go 1.19) sets a soft ceiling")
	fmt.Println("Green Tea GC: better scanning locality, no API change - experimental in 1.25,")
	fmt.Println("the default since 1.26")
}

func m006StaysOnStack() int {
	local := 7
	p := &local // the address never leaves this frame, so `local` stays on the stack
	return *p
}

// =================================================================================================
// Section 5: Aliasing Hazards and How to Break Them
// =================================================================================================

/*
## Aliasing Hazards and How to Break Them

Four aliasing bugs account for most of the surprises in real Go code.

 1. **A sub-slice keeps the whole backing array alive.** `small := huge[:10]` retains all of `huge`.
    Fix with `slices.Clone(huge[:10])`.
 2. **Two slices of one array overlap.** Writing through one is visible through the other, and
    `append` on the shorter one silently overwrites the longer one's elements. Fix with the
    three-index form `s[a:b:b]`, which caps the capacity so `append` must reallocate.
 3. **A struct copy shares its slice and map fields.** Fix by writing an explicit deep copy —
    Go has no built-in one, and `reflect.DeepEqual` compares but does not clone.
 4. **A `[]*T` retains every pointed-to object.** Deleting an element by re-slicing leaves the tail
    element still pointing at the object, so it never gets collected. `slices.Delete` zeroes the
    vacated slot for you.

The general tool for the first three is `slices.Clone` / `maps.Clone`, which copy one level deep.
For anything deeper, write the copy by hand and keep it next to the type.
*/

type m006Record struct {
	ID   int
	Tags []string
}

// Clone is the hand-written deep copy Go does not give you.
func (r m006Record) Clone() m006Record {
	out := r                                    // copies ID, and the Tags HEADER
	out.Tags = append([]string(nil), r.Tags...) // now give it its own array
	return out
}

func m006AliasingHazards() {
	fmt.Println("\n--- Section 5: Aliasing Hazards and How to Break Them ---")

	// 1. A sub-slice retains the whole array.
	huge := make([]byte, 1<<20) // 1 MB
	leaky := huge[:10]
	fmt.Printf("1. a 10-byte slice of a 1 MB array: len=%d cap=%d <- the whole megabyte stays alive\n",
		len(leaky), cap(leaky))
	safe := append([]byte(nil), huge[:10]...)
	fmt.Printf("   after copying: len=%d cap=%d <- the copy no longer retains the megabyte\n",
		len(safe), cap(safe))

	// 2. Overlapping slices.
	base := []int{1, 2, 3, 4, 5}
	front := base[0:2]
	front = append(front, 99) // spare capacity: clobbers base[2]
	fmt.Printf("2. append through a sub-slice overwrote the parent: base=%v\n", base)
	base = []int{1, 2, 3, 4, 5}
	capped := base[0:2:2] // capacity forced to 2
	capped = append(capped, 99)
	fmt.Printf("   with the three-index form s[0:2:2]: base=%v capped=%v (reallocated)\n", base, capped)

	// 3. A struct copy shares its slice field.
	original := m006Record{ID: 1, Tags: []string{"a", "b"}}
	shallow := original
	shallow.Tags[0] = "CHANGED"
	fmt.Printf("3. shallow copy: original=%v shallow=%v <- Tags is shared\n", original, shallow)

	original = m006Record{ID: 1, Tags: []string{"a", "b"}}
	deep := original.Clone()
	deep.Tags[0] = "CHANGED"
	fmt.Printf("   deep copy:    original=%v deep=%v\n", original, deep)

	// 4. A []*T retains the pointed-to objects.
	fmt.Println("4. deleting from a []*T by re-slicing leaves the tail pointer in place;")
	fmt.Println("   slices.Delete zeroes the vacated slot so the object can be collected")
}

// =================================================================================================
// Section 6: unsafe and structs.HostLayout
// =================================================================================================

/*
## unsafe and structs.HostLayout

- The `unsafe` package is the escape hatch out of the type and memory safety guarantees. Importing
  it is a deliberate act: `go vet` scrutinises it, and code using it is excluded from the Go 1
  compatibility promise.
- What it offers:
    - `unsafe.Sizeof`, `Alignof`, `Offsetof` — compile-time constants describing memory layout.
      These are safe and useful on their own; the rest of the package is not.
    - `unsafe.Pointer` — an untyped pointer that can be converted to and from any `*T`, which is how
      you reinterpret memory.
    - `unsafe.Add`, `unsafe.Slice`, `unsafe.String`, `unsafe.SliceData`, `unsafe.StringData`
      (Go 1.17/1.20) — the sanctioned way to do the pointer arithmetic the language forbids.
- **`uintptr` is not a pointer.** Converting a `unsafe.Pointer` to a `uintptr` produces a plain
  integer that the garbage collector does not track. If the only reference to an object becomes a
  `uintptr`, the object can be collected, or moved by a growing stack, and the integer is then
  garbage. The valid conversions are enumerated in the `unsafe.Pointer` documentation, and anything
  outside that list is a bug even if it appears to work.
- **Struct layout**: fields are laid out in declaration order with **alignment padding** inserted
  between them. Reordering fields largest-first can shrink a struct, which matters when you have
  millions of them. `go vet -vettool=$(which fieldalignment)` reports the savings.
- **`structs.HostLayout`** (Go 1.23) is a zero-size marker you embed in a struct to declare that its
  layout must match the host platform's C ABI. It documents intent for cgo and syscall structs, and
  it is a no-op at runtime.
- **When to use `unsafe`**: essentially never in application code. Its legitimate homes are cgo
  interop, syscall wrappers, and zero-copy `string` ↔ `[]byte` conversion in a hot path that you
  have already profiled.
*/

// Field order affects size: this version pads badly.
type m006Padded struct {
	A bool  // 1 byte + 7 padding
	B int64 // 8
	C bool  // 1 byte + 7 padding
}

// The same fields, ordered largest-first.
type m006Packed struct {
	B int64 // 8
	A bool  // 1
	C bool  // 1 + 6 padding
}

// structs.HostLayout marks a struct whose layout must match the platform C ABI.
type m006CCompatible struct {
	_     structs.HostLayout
	Flags uint32
	Count uint32
}

func m006UnsafeAndLayout() {
	fmt.Println("\n--- Section 6: unsafe and structs.HostLayout ---")

	// --- The safe, useful part of unsafe ---
	fmt.Printf("Sizeof(int)=%d Sizeof(string)=%d Sizeof([]int)=%d Sizeof(map)=%d Sizeof(any)=%d\n",
		unsafe.Sizeof(int(0)), unsafe.Sizeof(""), unsafe.Sizeof([]int(nil)),
		unsafe.Sizeof(map[int]int(nil)), unsafe.Sizeof(any(nil)))
	fmt.Println("  note: a string is 2 words, a slice 3, an interface 2 - exactly as Section 2 said")

	// --- Padding ---
	fmt.Printf("m006Padded{bool,int64,bool} = %d bytes\n", unsafe.Sizeof(m006Padded{}))
	fmt.Printf("m006Packed{int64,bool,bool} = %d bytes  <- same fields, better order\n",
		unsafe.Sizeof(m006Packed{}))
	fmt.Printf("field offsets in the padded version: A=%d B=%d C=%d\n",
		unsafe.Offsetof(m006Padded{}.A), unsafe.Offsetof(m006Padded{}.B), unsafe.Offsetof(m006Padded{}.C))
	fmt.Printf("alignment of int64 on this platform: %d bytes\n", unsafe.Alignof(int64(0)))

	// --- structs.HostLayout is zero-cost ---
	fmt.Printf("m006CCompatible with a structs.HostLayout marker = %d bytes (the marker is free)\n",
		unsafe.Sizeof(m006CCompatible{}))

	// --- Zero-copy string/[]byte, the one pattern worth knowing ---
	b := []byte("zero-copy")
	s := unsafe.String(unsafe.SliceData(b), len(b))
	fmt.Printf("unsafe.String over a []byte: %q (no allocation, but b must never be modified now)\n", s)
	back := unsafe.Slice(unsafe.StringData(s), len(s))
	fmt.Printf("unsafe.Slice back to []byte: %q\n", back)
	fmt.Println("  this is valid ONLY while nothing mutates the bytes - use it after profiling, not before")

	// --- uintptr is not a pointer ---
	fmt.Println("uintptr is a plain integer: the GC does not track it, and stacks move,")
	fmt.Println("  so storing a pointer as a uintptr and dereferencing it later is a bug")
}

// Run006 runs every section of module 006 in order.
func Run006() {
	m006Pointers()
	m006ValueSemantics()
	m006PointerOrValue()
	m006EscapeAnalysis()
	m006AliasingHazards()
	m006UnsafeAndLayout()
}

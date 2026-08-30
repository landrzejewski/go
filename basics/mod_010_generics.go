package basics

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"iter"
	"maps"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
)

/*
# Module 010 — Generics

Generics arrived in **Go 1.18** and have been extended in every release since; **Go 1.27** added the
last big missing piece, generic *methods*. This module is the single source of truth for generics in
this course — read it top to bottom, then run it with `go run . 010` and match the output to the code.

The one sentence to keep in mind: a **constraint is an interface**, and an interface now describes
not just a set of methods but a **set of types**. Everything else follows from that.
*/

// =================================================================================================
// Section 1: Type Parameters
// =================================================================================================

/*
## Type Parameters

- A type parameter list follows the name, in **square brackets**: `func F[T any](x T) T`,
  `type S[T any] struct{ ... }`. Every parameter must be named and must have a constraint.
- Several parameters may share a constraint (`[T, U any]`) or have their own
  (`[K comparable, V any]`).
- **Instantiation** substitutes type arguments: `F[int]` produces an ordinary, non-generic function.
  You may instantiate explicitly (`F[int](3)`) or let **inference** do it (`F(3)`) — Section 5.
- A generic type must be instantiated before use as a type: `Stack[int]`, never bare `Stack`.
- Type parameters are **not** available on:
    - **struct fields**: `type S[T any] struct { F func[U any](U) }` is invalid — only declarations
      may have type parameters.
    - **methods**, until Go 1.27 — a method could use the *receiver's* parameters but could not
      introduce its own. Section 8 covers the change.
- Within the generic body, the only operations allowed on a `T` are those **every** type in the
  constraint's type set supports. `[T any]` therefore permits nothing but assignment, and taking
  the zero value with `var zero T`.
*/

func m010Identity[T any](v T) T { return v }

func m010Swap[T, U any](a T, b U) (U, T) { return b, a }

func m010Zero[T any]() T {
	var zero T // the only way to produce a zero value of an unknown type
	return zero
}

type m010Pair[K comparable, V any] struct {
	Key   K
	Value V
}

func (p m010Pair[K, V]) String() string { return fmt.Sprintf("%v=%v", p.Key, p.Value) }

func m010TypeParameters() {
	fmt.Println("--- Section 1: Type Parameters ---")

	// Explicit instantiation.
	fmt.Printf("  m010Identity[int](42)     = %v\n", m010Identity[int](42))
	// Inferred instantiation - the usual way.
	fmt.Printf("  m010Identity(\"inferred\")  = %v\n", m010Identity("inferred"))

	// Two independent type parameters.
	s, n := m010Swap(1, "one")
	fmt.Printf("  m010Swap(1, \"one\")        = (%v, %v)\n", s, n)

	// Zero values of an unknown type.
	fmt.Printf("  zero values: int=%v string=%q bool=%v slice=%v\n",
		m010Zero[int](), m010Zero[string](), m010Zero[bool](), m010Zero[[]int]())

	// A generic type must be instantiated.
	p := m010Pair[string, int]{Key: "answer", Value: 42}
	fmt.Printf("  m010Pair[string, int]: %v\n", p)
	//	var bare m010Pair // ERROR: cannot use generic type m010Pair[K comparable, V any] without instantiation

	// --- What [T any] permits ---
	// Nothing but assignment and the zero value:
	//	func sum[T any](a, b T) T { return a + b } // ERROR: invalid operation: operator + not defined on a (variable of type T constrained by any)
	fmt.Println("  [T any] allows NO operations - not +, not ==, not <. Constrain to get them.")
}

// =================================================================================================
// Section 2: Constraints as Type Sets
// =================================================================================================

/*
## Constraints as Type Sets

- Since Go 1.18 an interface defines a **type set**. A traditional method-only interface is the set
  of all types with those methods; adding **type elements** puts specific types in the set directly.
- The elements are combined with **`|`** (union):

	type Number interface { int | int64 | float64 }

- **`~T`** means "any type whose *underlying* type is `T`". Without the tilde, only the exact type
  matches, so `type MyInt int` would be excluded. **Almost always write `~`** — otherwise your
  constraint quietly rejects every defined type in the caller's code.
- A constraint may combine **methods and type elements**: a type must be in the type set *and* have
  the methods.
- An interface containing type elements is a **constraint only**. It cannot be used as a variable
  type: `var x Number` is `cannot use type Number outside a type constraint`.
- **`any`** is the empty constraint — every type, no operations.
- **`comparable`** is the set of types supporting `==` and `!=`. Note that since Go 1.20 an
  *ordinary interface* type satisfies `comparable` even though comparing it can panic at run time.
  The spec's phrasing is that `any` **satisfies** `comparable` but does not **implement** it.
- **`cmp.Ordered`** (Go 1.21, standard library) is the set of ordered types: all integers, all
  floats, and `string`. It is what you want for `<`.
- A constraint's **core type** determines what the compiler will let you do structurally: you can
  index or range over a `[T ~[]E]` because every type in the set is a slice, but a union of `[]int`
  and `map[string]int` has no core type and permits neither.

### Do not reach for `golang.org/x/exp/constraints`

Older tutorials — and plenty of code still in the wild — use `constraints.Integer`, `.Float`,
`.Signed` and `.Ordered` from `golang.org/x/exp/constraints`. Avoid them:

  - `x/exp` is **explicitly experimental** and sits outside the Go 1 compatibility promise
  - it is an **extra module dependency** for something you can write in three lines
  - `constraints.Ordered` is simply `cmp.Ordered` with an extra `go get`

If you need "any integer" or "any float", declare the type set yourself, as `m010Number` does below.
It is a one-off cost and it documents exactly what you accept.
*/

// A type set with ~, so defined types are included.
type m010Number interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
		~float32 | ~float64
}

// Without the tilde: exact types only.
type m010ExactInt interface{ int }

// Methods and type elements combined.
type m010PrintableNumber interface {
	m010Number
	String() string
}

// A constraint whose core type is a slice, so the body may index and range.
type m010Slice[E any] interface{ ~[]E }

type m010MyInt int

func (m m010MyInt) String() string { return fmt.Sprintf("MyInt(%d)", int(m)) }

func m010Sum[T m010Number](values []T) T {
	var total T
	for _, v := range values {
		total += v
	}
	return total
}

func m010SumExact[T m010ExactInt](values []T) T {
	var total T
	for _, v := range values {
		total += v
	}
	return total
}

func m010Describe[T m010PrintableNumber](v T) string {
	return fmt.Sprintf("%s (doubled: %v)", v.String(), v*2)
}

func m010First[S m010Slice[E], E any](s S) (E, bool) {
	if len(s) == 0 {
		var zero E
		return zero, false
	}
	return s[0], true
}

func m010ConstraintsAsTypeSets() {
	fmt.Println("\n--- Section 2: Constraints as Type Sets ---")

	fmt.Printf("  m010Sum([]int{1,2,3})       = %v\n", m010Sum([]int{1, 2, 3}))
	fmt.Printf("  m010Sum([]float64{1.5,2.5}) = %v\n", m010Sum([]float64{1.5, 2.5}))

	// ~ is what lets a DEFINED type through.
	mine := []m010MyInt{1, 2, 3}
	fmt.Printf("  m010Sum([]m010MyInt{...})   = %v  <- works because of ~int\n", m010Sum(mine))
	//	fmt.Println(m010SumExact(mine)) // ERROR: m010MyInt does not satisfy m010ExactInt (possibly missing ~ for int in m010ExactInt)
	fmt.Printf("  m010SumExact needs the exact type: %v\n", m010SumExact([]int{1, 2, 3}))
	fmt.Println("  forgetting ~ silently rejects every defined type - almost always write it")

	// --- Declare the type set yourself; do not depend on x/exp ---
	fmt.Println("  m010Number above is hand-written on purpose: golang.org/x/exp/constraints")
	fmt.Println("  is experimental, outside the Go 1 compatibility promise, and an extra")
	fmt.Println("  dependency - while its Ordered is just cmp.Ordered from the standard library:")
	fmt.Printf("  m010Min via cmp.Ordered works on ints and strings alike: %v / %q\n",
		m010Min(5, 3), m010Min("b", "a"))

	// Methods plus type elements.
	fmt.Printf("  m010Describe(m010MyInt(21)) = %s\n", m010Describe(m010MyInt(21)))

	// A constraint with a core type permits structural operations.
	type IDList []string
	v, ok := m010First(IDList{"first", "second"})
	fmt.Printf("  m010First over a DEFINED slice type: %q ok=%t\n", v, ok)

	// --- A constraint is not a type ---
	//	var x m010Number // ERROR: cannot use type m010Number outside a type constraint: interface contains type constraints
	fmt.Println("  an interface with type elements can only be a constraint, never a variable type")

	// --- comparable ---
	fmt.Printf("  m010Contains([]string{...}, \"b\") = %t\n",
		m010Contains([]string{"a", "b"}, "b"))
	// A slice is not comparable, so it cannot be the argument to a comparable parameter:
	//	m010Contains([][]int{{1}}, []int{1}) // ERROR: []int does not satisfy comparable
	fmt.Println("  comparable excludes slices, maps and funcs - at COMPILE time")
	fmt.Println("  but `any` satisfies comparable (Go 1.20), so a nested panic is still possible")
}

func m010Contains[T comparable](s []T, v T) bool {
	for _, e := range s {
		if e == v {
			return true
		}
	}
	return false
}

// =================================================================================================
// Section 3: Generic Functions
// =================================================================================================

/*
## Generic Functions

- The value of a generic function is **type safety without duplication**. Before Go 1.18 the
  alternatives were code generation, copy-paste per type, or `any` plus assertions — the last of
  which moves every error to run time and allocates.
- Constrain to the **weakest** constraint that lets the body compile. `comparable` if you only need
  `==`; `cmp.Ordered` if you need `<`; a custom type set only if you need arithmetic.
- Many of the obvious ones are **already in the standard library** and you should not write them:
  `slices.Contains`, `slices.Index`, `slices.Sort`, `slices.SortFunc`, `slices.Max`, `slices.Min`,
  `slices.Reverse`, `maps.Keys`, `maps.Values`, `min`, `max`, `clear`. Reach for those first.
- There is deliberately **no `Map`/`Filter`/`Reduce`** in the standard library. The Go team's
  position is that a `for` loop is clearer than a chain of higher-order calls, and that the iterator
  package (module 012) covers the composable cases.
- A generic function can take **another generic function** as a parameter, but the parameter's own
  type arguments must be fixed at that point — you cannot pass a still-generic function.
*/

func m010Map[T, U any](s []T, f func(T) U) []U {
	out := make([]U, len(s))
	for i, v := range s {
		out[i] = f(v)
	}
	return out
}

func m010Filter[T any](s []T, keep func(T) bool) []T {
	var out []T
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}

func m010Reduce[T, A any](s []T, initial A, f func(A, T) A) A {
	acc := initial
	for _, v := range s {
		acc = f(acc, v)
	}
	return acc
}

func m010Keys[K comparable, V any](m map[K]V) []K {
	out := make([]K, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func m010GenericFunctions() {
	fmt.Println("\n--- Section 3: Generic Functions ---")

	nums := []int{1, 2, 3, 4, 5}

	doubled := m010Map(nums, func(n int) int { return n * 2 })
	labelled := m010Map(nums, func(n int) string { return fmt.Sprintf("#%d", n) })
	fmt.Printf("  Map to int:    %v\n", doubled)
	fmt.Printf("  Map to string: %v   <- the result type changed, and inference found it\n", labelled)

	evens := m010Filter(nums, func(n int) bool { return n%2 == 0 })
	fmt.Printf("  Filter even:   %v\n", evens)

	total := m010Reduce(nums, 0, func(acc, n int) int { return acc + n })
	joined := m010Reduce(nums, "", func(acc string, n int) string { return acc + fmt.Sprint(n) })
	fmt.Printf("  Reduce to int: %v    Reduce to string: %q\n", total, joined)

	// --- Prefer the standard library ---
	fmt.Printf("  slices.Contains=%t slices.Index=%d slices.Max=%d min/max builtins: %d/%d\n",
		slices.Contains(nums, 3), slices.Index(nums, 3), slices.Max(nums), min(3, 1, 2), max(3, 1, 2))
	sorted := slices.Clone(nums)
	slices.SortFunc(sorted, func(a, b int) int { return cmp.Compare(b, a) })
	fmt.Printf("  slices.SortFunc + cmp.Compare, descending: %v\n", sorted)

	m := map[string]int{"b": 2, "a": 1, "c": 3}
	fmt.Printf("  slices.Sorted(maps.Keys(m)): %v\n", slices.Sorted(maps.Keys(m)))
	fmt.Printf("  a hand-written m010Keys works too, but do not write what exists: %d keys\n",
		len(m010Keys(m)))
}

// =================================================================================================
// Section 4: Generic Types
// =================================================================================================

/*
## Generic Types

- A generic type declares parameters after its name and uses them in its fields:
  `type Stack[T any] struct { items []T }`.
- **A method's receiver must repeat the parameters, but declares no new ones** (until Go 1.27,
  Section 8): `func (s *Stack[T]) Push(v T)`. The names are yours to choose but must match in count.
- The instantiated type `Stack[int]` is an ordinary type: it can have its own methods called, be
  stored in a struct, put in a slice, and compared if its fields are comparable.
- Generic **interfaces** work the same way: `type Container[T any] interface { Add(T); Get(int) T }`.
- The zero value of a generic type is the zero value of its fields, so `var s Stack[int]` is usable
  if the fields' zero values are — the same "useful zero value" rule as everywhere else.
- **Recursion is allowed but constrained**: `type Node[T any] struct { Next *Node[T] }` is fine;
  a type that instantiates itself with a *different* argument at each level is rejected, because it
  would need infinitely many instantiations.
- This repo's `examples/common/stack.go` is the same `Stack[T]`, kept there because it is the answer to
  exercise 2 in `notes.md`.
*/

// m010Stack is the generic stack, mirroring examples/common/stack.go.
type m010Stack[T any] struct {
	items []T
}

func (s *m010Stack[T]) Push(v T) { s.items = append(s.items, v) }

func (s *m010Stack[T]) Pop() (T, bool) {
	var zero T
	if len(s.items) == 0 {
		return zero, false
	}
	v := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return v, true
}

func (s *m010Stack[T]) Size() int { return len(s.items) }

// A generic interface.
type m010Container[T any] interface {
	Add(T)
	Get(i int) T
	Len() int
}

type m010SliceContainer[T any] struct{ items []T }

func (c *m010SliceContainer[T]) Add(v T)     { c.items = append(c.items, v) }
func (c *m010SliceContainer[T]) Get(i int) T { return c.items[i] }
func (c *m010SliceContainer[T]) Len() int    { return len(c.items) }

// Two type parameters, plus a mutex - a cache that keeps its key and value types.
type m010Cache[K comparable, V any] struct {
	mu    sync.RWMutex
	items map[K]V
}

func m010NewCache[K comparable, V any]() *m010Cache[K, V] {
	return &m010Cache[K, V]{items: make(map[K]V)}
}

func (c *m010Cache[K, V]) Set(k K, v V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[k] = v
}

func (c *m010Cache[K, V]) Get(k K) (V, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.items[k]
	return v, ok
}

// A recursive generic type - legal, because every level instantiates Node[T] with the same T.
type m010Node[T any] struct {
	Value T
	Next  *m010Node[T]
}

func m010GenericTypes() {
	fmt.Println("\n--- Section 4: Generic Types ---")

	ints := &m010Stack[int]{}
	ints.Push(1)
	ints.Push(2)
	v, _ := ints.Pop()
	fmt.Printf("  m010Stack[int]:    Pop()=%v (%T) size=%d\n", v, v, ints.Size())

	strs := &m010Stack[string]{}
	strs.Push("a")
	s, _ := strs.Pop()
	fmt.Printf("  m010Stack[string]: Pop()=%q (%T)\n", s, s)

	// Popping an empty stack returns the zero value and false - no panic, no assertion.
	empty := &m010Stack[float64]{}
	z, ok := empty.Pop()
	fmt.Printf("  empty stack:       Pop()=%v ok=%t\n", z, ok)

	// A generic interface, satisfied by a generic type.
	var c m010Container[string] = &m010SliceContainer[string]{}
	c.Add("x")
	c.Add("y")
	fmt.Printf("  m010Container[string]: Get(1)=%q Len()=%d\n", c.Get(1), c.Len())

	// Two parameters.
	cache := m010NewCache[string, []int]()
	cache.Set("primes", []int{2, 3, 5})
	got, _ := cache.Get("primes")
	fmt.Printf("  m010Cache[string, []int]: %v (%T) - no assertion anywhere\n", got, got)

	// Recursive generic type.
	list := &m010Node[string]{Value: "a", Next: &m010Node[string]{Value: "b"}}
	fmt.Print("  m010Node[string] list: ")
	for n := list; n != nil; n = n.Next {
		fmt.Printf("%s ", n.Value)
	}
	fmt.Println()

	fmt.Println("  the same Stack[T] lives in examples/common/stack.go - exercise 2 in notes.md")

	// --- Generic functional options ---
	// Module 005 Section 5 shows the concrete version; the type parameter makes one Option
	// type serve every configurable struct in a package.
	srv := m010New(func(s *m010Endpoint) { s.Host = "example.com" }, m010WithPort(8443))
	fmt.Printf("  generic functional options: %+v\n", *srv)

	// --- Type-safe context keys ---
	// Module 011 Section 5 insists on an unexported key type. Parameterising it goes further:
	// the key carries its VALUE type, so the retrieval assertion can no longer be wrong.
	ctx := m010WithValue(context.Background(), m010UserKey, "ada")
	ctx = m010WithValue(ctx, m010RetriesKey, 3)
	user, userOK := m010Value(ctx, m010UserKey)
	retries, retriesOK := m010Value(ctx, m010RetriesKey)
	fmt.Printf("  type-safe context keys: user=%q(%t) retries=%d(%t)\n",
		user, userOK, retries, retriesOK)
	fmt.Println("  the key's type parameter fixes the value type, so ctx.Value cannot be")
	fmt.Println("  asserted to the wrong type - the compiler decides it, not the caller")
	//	_, _ = m010Value(ctx, m010UserKey) // returns (string, bool); an int is not assignable
}

// Generic functional options: one Option type per configured type, inferred at the call site.
type m010Option[T any] func(*T)

type m010Endpoint struct {
	Host string
	Port int
}

func m010WithPort(p int) m010Option[m010Endpoint] {
	return func(e *m010Endpoint) { e.Port = p }
}

func m010New[T any](opts ...m010Option[T]) *T {
	var t T
	for _, opt := range opts {
		opt(&t)
	}
	return &t
}

// A context key that carries its value's type, so retrieval cannot assert the wrong one.
type m010CtxKey[T any] struct{ name string }

var (
	m010UserKey    = m010CtxKey[string]{"user"}
	m010RetriesKey = m010CtxKey[int]{"retries"}
)

func m010WithValue[T any](ctx context.Context, key m010CtxKey[T], v T) context.Context {
	return context.WithValue(ctx, key, v)
}

func m010Value[T any](ctx context.Context, key m010CtxKey[T]) (T, bool) {
	v, ok := ctx.Value(key).(T)
	return v, ok
}

// =================================================================================================
// Section 5: Type Inference
// =================================================================================================

/*
## Type Inference

Inference is what keeps generic Go readable. It runs in stages:

 1. **Function argument inference** — match the types of the actual arguments against the parameter
    types. `m010Identity(42)` infers `T = int`.
 2. **Constraint type inference** — use the constraints themselves. Given `[S ~[]E, E any]`, knowing
    `S = []int` yields `E = int` even though no argument mentions `E` directly.
 3. **Untyped constants** are reconciled **with each other** first, yielding one common kind, and
    only then matched against any typed arguments. So `m010Min(1, 2.0)` works and gives `float64`;
    it is `m010Min(1, "a")` that fails, because an untyped int and an untyped string share no kind.
    Where it does bite is a **typed** argument: two typed arguments of different types never unify,
    and an untyped constant must be representable in the type the typed arguments determined.

### What it cannot do

- Infer from the **result type**: `var x []string = m010Zero()` cannot work, because inference does
  not look at the assignment target for ordinary calls. Instantiate explicitly.
- Infer a parameter that appears **only** in the results: `func New[T any]() *Stack[T]` always needs
  `New[int]()`.

### Version history

- **Go 1.21** made inference substantially stronger: it can now infer for a generic function
  **assigned to a variable of function type**, **passed as an argument** to another function, and
  for **methods used as values**.
- **Go 1.27** generalised this into an *assignability* rule, so the remaining contexts work too:
  storing a generic function in a **struct field**, a **map or slice value**, or **returning** it.
  Previously each of those needed explicit instantiation.

You may always instantiate explicitly. Do so when inference fails, and also when the explicit type
makes the call clearer than the inference would.
*/

func m010Min[T cmp.Ordered](a, b T) T {
	if a < b {
		return a
	}
	return b
}

// A constraint where E appears only in the constraint, not in a parameter.
func m010Last[S ~[]E, E any](s S) (E, bool) {
	if len(s) == 0 {
		var zero E
		return zero, false
	}
	return s[len(s)-1], true
}

func m010Apply[T any](f func(T) T, v T) T { return f(v) }

func m010TypeInference() {
	fmt.Println("\n--- Section 5: Type Inference ---")

	// 1. Function argument inference.
	fmt.Printf("  m010Min(5, 3)          = %v (T inferred as int)\n", m010Min(5, 3))
	fmt.Printf("  m010Min(\"b\", \"a\")      = %q (T inferred as string)\n", m010Min("b", "a"))

	// 2. Constraint type inference: E comes from S, not from any argument.
	type Names []string
	last, _ := m010Last(Names{"first", "last"})
	fmt.Printf("  m010Last(Names{...})   = %q (S=Names, so E=string by constraint inference)\n", last)

	// 3. Untyped constants are reconciled with EACH OTHER, then with the typed arguments.
	fmt.Printf("  m010Min(3.14, 2.71)    = %v (both untyped floats -> float64)\n", m010Min(3.14, 2.71))
	fmt.Printf("  m010Min(1, 2.0)        = %v (%T) <- an untyped int and float DO reconcile\n",
		m010Min(1, 2.0), m010Min(1, 2.0))
	//	fmt.Println(m010Min(1, "a")) // ERROR: in call to m010Min, mismatched types untyped int and untyped string (cannot infer T)

	// A TYPED argument is what actually constrains things.
	var typedInt int = 1
	fmt.Printf("  m010Min(typedInt, 2.0) = %v (%T) <- 2.0 is representable as an int\n",
		m010Min(typedInt, 2.0), m010Min(typedInt, 2.0))
	//	fmt.Println(m010Min(typedInt, 2.5)) // ERROR: cannot use 2.5 (untyped float constant) as int value in argument to m010Min (truncated)
	//	var typedFloat float64 = 2
	//	fmt.Println(m010Min(typedInt, typedFloat)) // ERROR: in call to m010Min, type float64 of typedFloat does not match inferred type int for T
	fmt.Println("  two TYPED arguments of different types never unify - convert, or instantiate")

	// --- What inference cannot do ---
	//	var x []string = m010Zero() // ERROR: in call to m010Zero, cannot infer T
	fmt.Printf("  a parameter used only in the RESULT must be given: m010Zero[[]string]()=%v\n",
		m010Zero[[]string]())

	// --- Go 1.21: a generic function assigned to a func-typed variable ---
	var minInt func(int, int) int = m010Min // inferred, no [int] needed
	fmt.Printf("  Go 1.21: `var f func(int,int)int = m010Min` infers T: %v\n", minInt(4, 7))

	// --- Go 1.21: a generic function passed straight as an argument ---
	fmt.Printf("  Go 1.21: passing m010Min straight as an argument: %v\n",
		m010CombineWith(m010Min, 9, 2))

	// --- Go 1.27: the remaining assignment contexts (struct fields, maps, returns) ---

	type holder struct{ Reduce func(string, string) string }
	h := holder{Reduce: m010Min} // Go 1.27: inferred into a struct field
	fmt.Printf("  Go 1.27: stored in a struct field with no instantiation: %q\n", h.Reduce("b", "a"))

	table := map[string]func(int) int{"identity": m010Identity} // Go 1.27: inferred into a map value
	fmt.Printf("  Go 1.27: inferred into a map value: %v\n", table["identity"](5))

	fmt.Printf("  m010Apply with an inferred closure: %v\n",
		m010Apply(func(n int) int { return n * 3 }, 7))
}

func m010CombineWith(f func(int, int) int, a, b int) int { return f(a, b) }

// =================================================================================================
// Section 6: Generic Type Aliases (Go 1.24)
// =================================================================================================

/*
## Generic Type Aliases (Go 1.24)

- Before Go 1.24 an alias could not have type parameters: `type Set[T comparable] = map[T]struct{}`
  was a syntax error. Generic *types* could be declared, but not aliased — an odd gap that made
  gradual refactoring of generic code awkward.
- Go 1.24 fixed it. A **parameterised alias** is still an alias: it is the *same type* as its
  target, assignable both ways with no conversion, and it **cannot carry methods**.
- The main uses are the same as for any alias: **moving a generic type between packages** while
  keeping the old name working, and giving a long instantiation a short readable name.
- Do not reach for an alias when you want a distinct type — use a definition. `type Set[T
  comparable] map[T]struct{}` (no `=`) creates a new type that *can* have `Add` and `Has` methods.
*/

// Generic aliases (Go 1.24): other names for the same types.
type (
	m010Set[T comparable]     = map[T]struct{}
	m010Result[T any]         = m010Outcome[T]
	m010StringMap[V any]      = map[string]V
	m010Transformer[T, U any] = func(T) U
)

// The definition an alias points at.
type m010Outcome[T any] struct {
	Value T
	Err   error
}

func (o m010Outcome[T]) IsOK() bool { return o.Err == nil }

func m010OK[T any](v T) m010Outcome[T] { return m010Outcome[T]{Value: v} }

func m010Err[T any](err error) m010Outcome[T] {
	var zero T
	return m010Outcome[T]{Value: zero, Err: err}
}

func m010GenericAliases() {
	fmt.Println("\n--- Section 6: Generic Type Aliases (Go 1.24) ---")

	// An alias IS the target type - assignable both ways, no conversion.
	set := m010Set[string]{"a": {}, "b": {}}
	var asPlainMap map[string]struct{} = set
	var backAgain m010Set[string] = asPlainMap
	fmt.Printf("  m010Set[string] and map[string]struct{} are one type: len=%d\n", len(backAgain))

	// It works for any generic shape, including function types.
	var upper m010Transformer[string, string] = strings.ToUpper
	fmt.Printf("  m010Transformer[string,string] = strings.ToUpper: %q\n", upper("alias"))

	counts := m010StringMap[int]{"one": 1}
	fmt.Printf("  m010StringMap[int]: %v\n", counts)

	// An alias to a generic STRUCT type carries the target's methods, because it IS the target.
	r := m010Result[int](m010OK(42))
	fmt.Printf("  m010Result[int] aliases m010Outcome[int]: value=%v ok=%t\n", r.Value, r.IsOK())
	bad := m010Err[int](errors.New("failed"))
	fmt.Printf("  m010Err[int]: value=%v ok=%t err=%v\n", bad.Value, bad.IsOK(), bad.Err)

	// An alias cannot declare methods of its own:
	//	func (s m010Set[T]) Add(v T) {} // ERROR: cannot define new methods on generic alias type m010Set[T comparable]
	fmt.Println("  an alias cannot have methods - use a type DEFINITION (no =) for that")
}

// =================================================================================================
// Section 7: Generics and Iterators
// =================================================================================================

/*
## Generics and Iterators

- Go 1.23's iterators are **built entirely on generics**: `iter.Seq[V]` is
  `func(yield func(V) bool)` and `iter.Seq2[K, V]` is `func(yield func(K, V) bool)`. They are
  nothing more than named generic function types.
- That combination is where `Map`/`Filter`/`Reduce` finally pay off in Go: over a slice they are
  no better than a `for` loop, but over an `iter.Seq` they compose **lazily**, with no intermediate
  slices, and the chain still short-circuits when the consumer breaks.
- The standard library provides the bridges: `slices.Values`, `slices.All`, `slices.Collect`,
  `slices.Sorted`, `maps.Keys`, `maps.Values`, `maps.All`, `maps.Collect`.
- Module 012 covers iterators in full. This section shows only the generic angle: writing an adapter
  that works for every element type at once.
*/

func m010MapSeq[T, U any](seq iter.Seq[T], f func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for v := range seq {
			if !yield(f(v)) {
				return
			}
		}
	}
}

func m010FilterSeq[T any](seq iter.Seq[T], keep func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range seq {
			if keep(v) && !yield(v) {
				return
			}
		}
	}
}

func m010TakeSeq[T any](seq iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		taken := 0
		for v := range seq {
			if taken >= n {
				return
			}
			if !yield(v) {
				return
			}
			taken++
		}
	}
}

// An infinite sequence, to show that the chain really is lazy.
func m010Naturals() iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 1; ; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

func m010GenericsAndIterators() {
	fmt.Println("\n--- Section 7: Generics and Iterators ---")

	// A lazy pipeline over an INFINITE sequence: nothing is materialised.
	pipeline := m010TakeSeq(
		m010MapSeq(
			m010FilterSeq(m010Naturals(), func(n int) bool { return n%3 == 0 }),
			func(n int) string { return fmt.Sprintf("<%d>", n) },
		), 5)

	fmt.Print("  first 5 multiples of 3, mapped, from an INFINITE source: ")
	for s := range pipeline {
		fmt.Print(s, " ")
	}
	fmt.Println()
	fmt.Println("  no intermediate slice was ever built, and the source never terminates")

	// The standard library's bridges between slices/maps and iterators.
	nums := []int{4, 1, 3, 2}
	evens := slices.Collect(m010FilterSeq(slices.Values(nums), func(n int) bool { return n%2 == 0 }))
	fmt.Printf("  slices.Values -> filter -> slices.Collect: %v\n", evens)
	fmt.Printf("  slices.Sorted(slices.Values(nums)):        %v\n", slices.Sorted(slices.Values(nums)))

	m := map[string]int{"b": 2, "a": 1}
	fmt.Printf("  slices.Sorted(maps.Keys(m)):               %v\n", slices.Sorted(maps.Keys(m)))
}

// =================================================================================================
// Section 8: Generic Methods (Go 1.27)
// =================================================================================================

/*
## Generic Methods (Go 1.27)

This is the headline language change of Go 1.27.

- Until 1.27 a method could use the **receiver's** type parameters but could not declare its own.
  `func (l List[E]) Map[F any](f func(E) F) List[F]` was rejected with
  `method must have no type parameters`. The workaround was a package-level function taking the
  receiver as its first argument — which is why the standard library has `slices.SortFunc(s, f)`
  rather than `s.SortFunc(f)`.
- **Go 1.27 allows a method to declare type parameters of its own**, in addition to any the
  receiver declares. The spec's own example is:

	type List[E any] []E

	func (l List[E]) Apply[F any](f func(E) F) List[F] { ... }

- The standard library adopted it immediately: `math/rand/v2` now has
  `func (r *Rand) N[Int intType](n Int) Int`, a method that was previously only available as the
  package-level `rand.N`.
- **The important restriction remains**: a method with its own type parameters **cannot be used to
  satisfy an interface**. An interface method has a fixed signature, and a generic method has
  infinitely many, so there is nothing for the interface to match. If you need interface
  satisfaction, keep the method non-generic.
- Method values and method expressions work, but the method must be instantiated first:
  `l.Apply[string]` is a value, `l.Apply` on its own is not.
- Use it where a transformation is genuinely a **method on the receiver's own type**. Do not rush to
  convert existing package-level generic functions — `slices.SortFunc` reads perfectly well.
*/

// The spec's example, spelled out.
type m010List[E any] []E

// Apply is a GENERIC METHOD: it declares F in addition to the receiver's E. Go 1.27.
func (l m010List[E]) Apply[F any](f func(E) F) m010List[F] {
	out := make(m010List[F], len(l))
	for i, v := range l {
		out[i] = f(v)
	}
	return out
}

// Fold is a second generic method, this time with an accumulator type.
func (l m010List[E]) Fold[A any](initial A, f func(A, E) A) A {
	acc := initial
	for _, v := range l {
		acc = f(acc, v)
	}
	return acc
}

// A non-generic method on the same type, for contrast - this one CAN satisfy an interface.
func (l m010List[E]) Len() int { return len(l) }

type m010Lener interface{ Len() int }

func m010GenericMethods() {
	fmt.Println("\n--- Section 8: Generic Methods (Go 1.27) ---")

	nums := m010List[int]{1, 2, 3}

	// A method whose RESULT element type differs from the receiver's.
	labels := nums.Apply(func(n int) string { return fmt.Sprintf("n%d", n) })
	fmt.Printf("  m010List[int].Apply -> %v (%T)\n", labels, labels)

	widths := nums.Apply(func(n int) float64 { return float64(n) * 1.5 })
	fmt.Printf("  the same method, another result type: %v (%T)\n", widths, widths)

	// Chaining generic methods - the shape that was impossible before 1.27.
	joined := nums.
		Apply(func(n int) string { return strings.Repeat("*", n) }).
		Fold("", func(acc, s string) string { return acc + s + "|" })
	fmt.Printf("  chained Apply then Fold: %q\n", joined)

	// Explicit instantiation, and a method value.
	explicit := nums.Apply[bool](func(n int) bool { return n%2 == 1 })
	fmt.Printf("  explicit instantiation nums.Apply[bool]: %v\n", explicit)
	asValue := nums.Apply[string] // a method value; the method must be instantiated first
	fmt.Printf("  method value nums.Apply[string]: %v\n", asValue(func(n int) string { return fmt.Sprint(n) }))

	// --- The standard library uses it too ---
	r := rand.New(rand.NewPCG(1, 2))
	fmt.Printf("  math/rand/v2 (*Rand).N is a generic method: N[int](100)=%d N[int64](5)=%d\n",
		r.N(100), r.N(int64(5)))

	// --- The restriction: a generic method cannot satisfy an interface ---
	var l m010Lener = nums // Len is NOT generic, so this works
	fmt.Printf("  the NON-generic Len() satisfies an interface: %d\n", l.Len())
	//	type applier interface { Apply[F any](func(int) F) m010List[F] } // ERROR: interface method must have no type parameters
	fmt.Println("  a generic method cannot appear in an interface - it has no single signature")
	fmt.Println("  keep a method non-generic when interface satisfaction matters")
}

// =================================================================================================
// Section 9: Generics in the Standard Library
// =================================================================================================

/*
## Generics in the Standard Library

The best way to learn what generics are good for is to look at where the standard library used them.

  - **`slices`** (1.21) — `Sort`, `SortFunc`, `Contains`, `Index`, `Max`, `Min`, `Reverse`, `Clone`,
    `Equal`, `Compact`, `Insert`, `Delete`, `BinarySearch`, and the 1.23 iterator bridges
    `All`, `Values`, `Collect`, `Sorted`, `Backward`.
  - **`maps`** (1.21) — `Clone`, `Equal`, `EqualFunc`, `Copy`, `DeleteFunc`; plus the 1.23
    iterator bridges `Keys`, `Values`, `All`, `Collect`, `Insert`.
  - **`cmp`** (1.21) — the `Ordered` constraint, plus `Compare` and `Less`; `Or` is 1.22.
  - **`sync`** (1.21) — `OnceValue[T]`, `OnceValues[T1, T2]`, `OnceFunc`: lazy initialisation with
    no `sync.Once` boilerplate and no package-level variable.
  - **`iter`** (1.23) — `Seq[V]`, `Seq2[K, V]`, `Pull`, `Pull2`.
  - **`unique`** (1.23) — `Make[T]`/`Handle[T]`: interning, so equal values share one allocation and
    compare by pointer.
  - **`weak`** (1.24) — `Pointer[T]`, a weak reference the garbage collector may clear.
  - **`reflect.TypeAssert[T]`** (1.25) — a typed, allocation-free assertion from a `reflect.Value`.
  - **`errors.AsType[T]`** (1.26) — the generic `errors.As` (module 009, Section 4).
  - **`hash/maphash.Hasher[T]`** and **`ComparableHasher[T]`** (1.27) — hashing any comparable type.
  - **`math/rand/v2` `(*Rand).N[Int]`** (1.27) — the generic method from Section 8.
  - **`sync.Map`** is a notable *non*-adopter: it is still `any`-based, because changing it would
    break compatibility.

Note what is **not** there: no `Optional[T]`, no `Result[T]`, no `Set[T]`, no `Map`/`Filter`. The Go
team deliberately kept the library small and let the community decide.
*/

func m010StdlibGenerics() {
	fmt.Println("\n--- Section 9: Generics in the Standard Library ---")

	// sync.OnceValue: lazy initialisation with no boilerplate.
	expensive := sync.OnceValue(func() string {
		fmt.Println("    (the expensive computation runs exactly once)")
		return "computed"
	})
	fmt.Printf("  sync.OnceValue: %q then %q\n", expensive(), expensive())

	// sync.OnceValues for a (value, error) pair.
	load := sync.OnceValues(func() (int, error) { return 42, nil })
	v, err := load()
	fmt.Printf("  sync.OnceValues: %v %v\n", v, err)

	// cmp.Or returns the first non-zero argument - a tidy default-value idiom.
	fmt.Printf("  cmp.Or(\"\", \"\", \"fallback\") = %q\n", cmp.Or("", "", "fallback"))
	fmt.Printf("  cmp.Compare(NaN-safe total order): Compare(1,2)=%d Compare(2,2)=%d\n",
		cmp.Compare(1, 2), cmp.Compare(2, 2))

	// slices and maps, the two most-used generic packages.
	s := []int{3, 1, 2, 2}
	fmt.Printf("  slices.Sorted+Compact: %v\n", slices.Compact(slices.Sorted(slices.Values(s))))
	idx, found := slices.BinarySearch([]int{1, 2, 3}, 2)
	fmt.Printf("  slices.BinarySearch:  index=%d found=%t\n", idx, found)

	// errors.AsType (Go 1.26) - the generic errors.As from module 009.
	wrapped := fmt.Errorf("outer: %w", &m009ParseError{Line: 3, Token: "x"})
	if pe, ok := errors.AsType[*m009ParseError](wrapped); ok {
		fmt.Printf("  errors.AsType[*m009ParseError] (1.26): line=%d\n", pe.Line)
	}

	fmt.Println("  not in the standard library, on purpose: Optional[T], Result[T], Set[T], Map/Filter")
}

// =================================================================================================
// Section 10: Cost, Limits and When Not to Use Generics
// =================================================================================================

/*
## Cost, Limits and When Not to Use Generics

### How it is compiled

Go uses **GC-shape stenciling with dictionaries**. It does not monomorphise every instantiation as
C++ does, nor box everything as Java does. Types sharing a *GC shape* — broadly, the same size and
pointer layout — share one compiled copy, and a hidden **dictionary** argument carries the
type-specific details. So all pointer-shaped instantiations (`*T`, `[]T`, `map[K]V`, interfaces)
share a single stencil, while `int`, `float64` and `struct{a,b int}` each get their own.

The consequences are worth knowing:

  - binary size stays reasonable, unlike full monomorphisation
  - a generic function can be **slower** than a hand-written concrete one, because the dictionary
    adds an indirection and can block inlining
  - it is usually **faster than the `any` + type-assertion** version it replaces, since nothing is
    boxed and nothing is asserted
  - the difference is small; **measure before assuming** (`go test -bench . -benchmem`)

### Hard limits

  - **no type parameters on struct fields** — only declarations may have them
  - **a generic method cannot satisfy an interface** (Section 8)
  - **no specialisation**: you cannot write a faster overload for one concrete `T`
  - **no operator constraints beyond type sets**: there is no way to say "any type supporting `+`"
    other than by listing the types
  - **no variadic type parameters**, no higher-kinded types
  - **`comparable` is compile-time only**; an `any` inside can still panic at run time

### When not to use them

The Go team's own guidance, in order:

 1. **Do not use a type parameter when a method on an interface would do.** If the behaviour differs
    per type, that is what interfaces are for. Generics are for when the *logic* is identical and
    only the *type* changes.
 2. **Do not write a generic function with a single call site.** Write the concrete one; generalise
    when the second caller appears.
 3. **Do not use generics for reflection-shaped problems.** If you need field names or tags, you
    need reflection, not type parameters.
 4. **Do not generify data structures that are only ever used with one type.**

The recurring advice is: **write the concrete code first**, and let the duplication tell you when a
type parameter earns its place.
*/

func m010CostAndLimits() {
	fmt.Println("\n--- Section 10: Cost, Limits and When Not to Use Generics ---")

	fmt.Println("  compiled with GC-shape stenciling plus dictionaries:")
	fmt.Println("    all pointer-shaped instantiations share ONE compiled copy")
	fmt.Println("    int, float64 and each struct shape get their own")
	fmt.Println("    so: smaller binaries than C++, faster than boxing into any")

	// The three ways to write the same thing, so the trade-off is concrete.
	nums := []int{5, 2, 8, 1}
	fmt.Printf("  concrete:  m010MaxInt(%v)      = %d\n", nums, m010MaxInt(nums))
	fmt.Printf("  generic:   slices.Max(%v)      = %d\n", nums, slices.Max(nums))
	fmt.Printf("  any-based: m010MaxAny(...)     = %v  <- boxes, asserts, fails at run time\n",
		m010MaxAny([]any{5, 2, 8, 1}))
	fmt.Printf("  and the any version panics on bad input: %v\n",
		m005CatchPanic(func() { m010MaxAny([]any{1, "two"}) }))

	fmt.Println("  benchmark them yourself: go test -bench . -benchmem ./basics/")

	// --- Limits ---
	//	type bad[T any] struct { f func[U any](U) U } // ERROR: function type must have no type parameters
	fmt.Println("  limits: no type parameters on struct fields; no specialisation;")
	fmt.Println("          no 'any type supporting +' other than by listing the types;")
	fmt.Println("          a generic method cannot satisfy an interface")

	// --- When an interface is the right answer ---
	fmt.Println("  use an INTERFACE when the behaviour differs per type:")
	for _, s := range []m008Shape{m008Circle{R: 1}, m008Square{Side: 2}} {
		fmt.Printf("    %-14T Area()=%.4f  <- each type computes it differently\n", s, s.Area())
	}
	fmt.Println("  use a TYPE PARAMETER when the logic is identical and only the type changes:")
	fmt.Printf("    m010Contains works the same for every comparable T: %t / %t\n",
		m010Contains([]int{1, 2}, 2), m010Contains([]string{"a"}, "b"))
}

func m010MaxInt(s []int) int {
	best := s[0]
	for _, v := range s[1:] {
		if v > best {
			best = v
		}
	}
	return best
}

// The pre-generics version: boxes every element, and fails at RUN time on a bad type.
func m010MaxAny(s []any) any {
	best := s[0].(int)
	for _, v := range s[1:] {
		if v.(int) > best {
			best = v.(int)
		}
	}
	return best
}

// =================================================================================================
// Section 11: Common Errors, and Where to Read More
// =================================================================================================

/*
## Common Errors, and Where to Read More

Generic code fails in a small number of recognisable ways. Every message below was produced by the
Go 1.27 compiler, not paraphrased — the `// ERROR:` comments throughout this module are the same
messages in their original context.

	MESSAGE                                              CAUSE                        FIX
	invalid operation: operator + not defined on a       `any` permits no             constrain to a type set
	  (variable of type T constrained by any)            operations                   that supports +
	X does not satisfy Y (possibly missing ~ for         the constraint lists         add ~ to the type
	  int in Y)                                          exact types                  element
	cannot use type N outside a type constraint:         an interface with type       it is a constraint only
	  interface contains type constraints                elements used as a type
	in call to F, cannot infer T                         T appears only in the        instantiate: F[int](...)
	                                                     results
	in call to F, mismatched types untyped int and       untyped constants whose      make the literals one
	  untyped string (cannot infer T)                    kinds cannot reconcile       kind, or instantiate
	type float64 of f does not match inferred type       two TYPED arguments of       convert one, or
	  int for T                                          different types              instantiate explicitly
	cannot use generic type S[T any] without             a generic type used bare     write S[int]
	  instantiation
	interface method must have no type parameters        a generic method listed      keep the method
	                                                     in an interface              non-generic
	syntax error: function type must have no type        type parameters on a         only declarations may
	  parameters                                         struct field or func type    have type parameters
	[]int does not satisfy comparable                    a slice, map or func where   use a comparable key,
	                                                     comparable is required       or pass a compare func
	cannot define new methods on generic alias type      a method on a generic        use a type DEFINITION
	  S[T comparable]                                    ALIAS (Go 1.24)              (no =) instead

### Where to read more

  - **The Go Programming Language Specification** — https://go.dev/ref/spec
    the sections *Type parameters*, *Type constraints*, *Type unification*, *Instantiations*
  - **An Introduction to Generics** — https://go.dev/blog/intro-generics
  - **When To Use Generics** — https://go.dev/blog/when-generics
    Ian Lance Taylor's guidance, and the source of Section 10's advice
  - **Deconstructing Type Parameters** — https://go.dev/blog/deconstructing-type-parameters
    why signatures like `[S ~[]E, E any]` are shaped the way they are
  - **Range Over Function Types** — https://go.dev/blog/range-functions
    the iterator design that Section 7 builds on
  - **Go 1.27 release notes** — https://go.dev/doc/go1.27
    generic methods, and inference in all assignment contexts

### In this repository

  - `basics/mod_012_iterators_and_collections.go` — `iter`, `slices`, `maps`, `cmp` in depth
  - `basics/mod_016_whats_new_1_21_to_1_27.go` — where each generic feature landed
  - `examples/common/stack.go`, `examples/common/math.go` — generics used in earnest, with the pre-generics
    versions kept as commented teaching steps
*/

func m010ErrorsAndFurtherReading() {
	fmt.Println("\n--- Section 11: Common Errors, and Where to Read More ---")

	type entry struct{ message, fix string }
	for _, e := range []entry{
		{"operator + not defined on a (T constrained by any)", "constrain to a type set with +"},
		{"X does not satisfy Y (possibly missing ~ for int)", "add ~ to the type element"},
		{"cannot use type N outside a type constraint", "it is a constraint, not a type"},
		{"in call to F, cannot infer T", "instantiate explicitly: F[int](...)"},
		{"mismatched types untyped int and untyped string", "make the literals one kind"},
		{"type float64 of f does not match inferred type int", "two typed args never unify"},
		{"cannot use generic type S[T any] without instantiation", "write S[int]"},
		{"interface method must have no type parameters", "keep the method non-generic"},
		{"function type must have no type parameters", "only declarations may have them"},
		{"[]int does not satisfy comparable", "use a comparable key type"},
		{"cannot define new methods on generic alias type", "use a definition, not an alias"},
	} {
		fmt.Printf("  %-54s -> %s\n", e.message, e.fix)
	}

	fmt.Println()
	fmt.Println("  every message above came from the Go 1.27 compiler, not from memory - the")
	fmt.Println("  // ERROR: comments in this module are the same messages in their own context")
	fmt.Println()
	fmt.Println("  read more:")
	fmt.Println("    https://go.dev/ref/spec                             the specification")
	fmt.Println("    https://go.dev/blog/intro-generics                  the tutorial")
	fmt.Println("    https://go.dev/blog/when-generics                   when NOT to use them")
	fmt.Println("    https://go.dev/blog/deconstructing-type-parameters  why [S ~[]E, E any]")
	fmt.Println("    https://go.dev/blog/range-functions                 the iterator design")
	fmt.Println("    https://go.dev/doc/go1.27                           generic methods")
}

// Run010 runs every section of module 010 in order.
func Run010() {
	m010TypeParameters()
	m010ConstraintsAsTypeSets()
	m010GenericFunctions()
	m010GenericTypes()
	m010TypeInference()
	m010GenericAliases()
	m010GenericsAndIterators()
	m010GenericMethods()
	m010StdlibGenerics()
	m010CostAndLimits()
	m010ErrorsAndFurtherReading()
}

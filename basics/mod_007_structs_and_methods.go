package basics

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

/*
# Module 007 — Structs and Methods

Go has **no classes and no inheritance**. What it has instead is:

  - **structs** for data,
  - **methods** that can be attached to any type defined in the same package, not just structs,
  - **embedding** for composition and method promotion,
  - **interfaces** for polymorphism (module 008).

The absence of inheritance is deliberate. There is no `super`, no virtual dispatch on a base class,
and no fragile-base-class problem. Everything that inheritance is normally used for is expressed as
either embedding (reuse) or an interface (substitutability), and those two are kept separate.
*/

// =================================================================================================
// Section 1: Methods
// =================================================================================================

/*
## Methods

- A method is a function with a **receiver** declared before its name:
  `func (r Receiver) Name(params) results`.
- The receiver's **base type must be defined in the same package**. You cannot add a method to
  `int`, to `time.Time`, or to any other package's type. Define your own type from it first —
  which is exactly what module 002b's `m002bCelsius` did.
- The base type may be **any** defined type, not just a struct: a named slice, map, function or
  integer type can all have methods. `http.HandlerFunc` is a method on a *function* type; `sort.
  IntSlice` is a method set on a *slice* type.
- The receiver may **not** be a pointer type or an interface type itself: `func (p *int) f()` is
  invalid because `*int`'s base type `int` is not local.
- Methods live in the **package namespace with the type**, not with functions — so a method `Add` on
  one type does not collide with a method `Add` on another, and this is the only place Go allows
  what looks like overloading.
- A method is just a function with a special first parameter. `T.Method` (a *method expression*)
  gives you exactly that function; `x.Method` (a *method value*) gives you the function with the
  receiver already bound (module 005, Section 4).
- The receiver name is conventionally a **one- or two-letter abbreviation of the type**, used
  consistently across every method. It is never `this` or `self` — Go style guides are explicit
  about this.
*/

type m007Rect struct {
	Width, Height float64
}

func (r m007Rect) Area() float64      { return r.Width * r.Height }
func (r m007Rect) Perimeter() float64 { return 2 * (r.Width + r.Height) }
func (r m007Rect) String() string     { return fmt.Sprintf("%gx%g", r.Width, r.Height) }

// Methods work on any defined type, not only structs.
type m007Celsius float64

func (c m007Celsius) IsFreezing() bool { return c <= 0 }

type m007IntList []int

func (l m007IntList) Sum() (total int) {
	for _, v := range l {
		total += v
	}
	return total
}

type m007StringSet map[string]struct{}

func (s m007StringSet) Add(v string) { s[v] = struct{}{} }
func (s m007StringSet) Has(v string) bool {
	_, ok := s[v]
	return ok
}

// A method on a FUNCTION type - the trick behind http.HandlerFunc.
type m007Validator func(string) bool

func (v m007Validator) Negate() m007Validator {
	return func(s string) bool { return !v(s) }
}

func m007Methods() {
	fmt.Println("--- Section 1: Methods ---")

	r := m007Rect{Width: 3, Height: 4}
	fmt.Printf("rect %v: area=%g perimeter=%g\n", r, r.Area(), r.Perimeter())

	// A method on a numeric type.
	fmt.Printf("m007Celsius(-5).IsFreezing() = %t\n", m007Celsius(-5).IsFreezing())

	// A method on a slice type.
	fmt.Printf("m007IntList{1,2,3}.Sum() = %d\n", m007IntList{1, 2, 3}.Sum())

	// A method on a map type.
	set := m007StringSet{}
	set.Add("go")
	fmt.Printf("m007StringSet.Has(\"go\")=%t Has(\"rust\")=%t\n", set.Has("go"), set.Has("rust"))

	// A method on a function type.
	isEmpty := m007Validator(func(s string) bool { return s == "" })
	notEmpty := isEmpty.Negate()
	fmt.Printf("function type with a method: isEmpty(\"\")=%t notEmpty(\"\")=%t\n",
		isEmpty(""), notEmpty(""))

	// --- What you cannot do ---
	//	func (i int) Double() int { return i * 2 }        // ERROR: cannot define new methods on non-local type int
	//	func (t time.Time) Tomorrow() time.Time { ... }   // ERROR: cannot define new methods on non-local type time.Time
	//	func (p *int) Zero() { *p = 0 }                   // ERROR: cannot define new methods on non-local type int
	fmt.Println("you cannot add methods to int, time.Time, or any other package's type")
	fmt.Println("  define your own type from it - see m002bCelsius in module 002b")

	// Two types may each have a method of the same name - the one place Go looks like overloading.
	fmt.Println("m007Rect.String() and m001bWeekday.String() coexist:", r.String(), "/", m001bMonday.String())
}

// =================================================================================================
// Section 2: Value and Pointer Receivers, and Method Sets
// =================================================================================================

/*
## Value and Pointer Receivers, and Method Sets

- A **value receiver** `func (t T) M()` gets a **copy**. Mutations are invisible to the caller.
- A **pointer receiver** `func (t *T) M()` gets the address. Mutations stick, and no copy is made.
- Go **automatically takes the address or dereferences** at the call site, so both spellings look
  the same in use: `v.PointerMethod()` becomes `(&v).PointerMethod()`, and `p.ValueMethod()` becomes
  `(*p).ValueMethod()`. This works **only when the value is addressable** — which is why you cannot
  call a pointer method on a map entry or on a function's return value.

### The method set rule — the part that actually bites

	the method set of  T  contains only the methods declared with receiver  T
	the method set of *T  contains the methods declared with receiver  T  AND  *T

So a `*T` satisfies more interfaces than a `T` does. If any method of your type has a pointer
receiver, then **only `*T` implements the interface**, and passing a bare `T` is a compile error —
usually reported as `... does not implement I (method M has pointer receiver)`. This is the single
most common confusing error message in Go.

### Choosing

- **Be consistent.** If any method needs a pointer receiver, give them all pointer receivers. A
  mixed set is confusing and makes the method-set rule bite unpredictably.
- Use a **pointer receiver** when the method mutates, when the struct is large, when the type
  contains a `sync.Mutex` or anything else that must not be copied, or when you want `nil` to be a
  meaningful receiver.
- Use a **value receiver** for small immutable types — `time.Time`, `netip.Addr`, coordinates — and
  for the `String() string` method of a small type, so that both `T` and `*T` can be printed.
- A **nil pointer receiver is legal** and does not panic by itself: the method runs with `t == nil`,
  and only dereferencing it panics. Some types exploit this deliberately, so that methods work on a
  nil receiver (a nil tree node returning height 0, for instance).
*/

type m007Account struct {
	Owner   string
	Balance int
}

func (a m007Account) DepositByValue(n int)    { a.Balance += n }
func (a *m007Account) DepositByPointer(n int) { a.Balance += n }
func (a m007Account) Describe() string        { return fmt.Sprintf("%s: %d", a.Owner, a.Balance) }

// A type whose methods work on a nil receiver.
type m007Tree struct {
	Value       int
	Left, Right *m007Tree
}

func (t *m007Tree) Height() int {
	if t == nil { // a nil receiver is perfectly legal
		return 0
	}
	return 1 + max(t.Left.Height(), t.Right.Height())
}

func (t *m007Tree) Sum() int {
	if t == nil {
		return 0
	}
	return t.Value + t.Left.Sum() + t.Right.Sum()
}

// A type that must never be copied.
type m007SafeCounter struct {
	mu sync.Mutex
	n  int
}

func (c *m007SafeCounter) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

func (c *m007SafeCounter) Get() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func m007Receivers() {
	fmt.Println("\n--- Section 2: Value and Pointer Receivers, and Method Sets ---")

	acc := m007Account{Owner: "Ada", Balance: 100}
	acc.DepositByValue(50)
	fmt.Printf("after DepositByValue(50):   %s  (the copy was credited)\n", acc.Describe())
	acc.DepositByPointer(50)
	fmt.Printf("after DepositByPointer(50): %s\n", acc.Describe())

	// --- Automatic addressing works only on addressable values ---
	fmt.Println("`acc.DepositByPointer(50)` is `(&acc).DepositByPointer(50)` - acc is addressable")
	accounts := map[string]m007Account{"ada": {Owner: "Ada"}}
	//	accounts["ada"].DepositByPointer(1) // ERROR: cannot call pointer method DepositByPointer on m007Account
	_ = accounts
	fmt.Println("a map entry is not addressable, so a pointer method cannot be called on it")

	// --- Method sets ---
	fmt.Println("method sets:")
	fmt.Printf("  methods of  m007Account: %v\n", m007MethodNames(reflect.TypeOf(m007Account{})))
	fmt.Printf("  methods of *m007Account: %v\n", m007MethodNames(reflect.TypeOf(&m007Account{})))
	fmt.Println("  *T has both; T has only the value-receiver ones - this is why an interface")
	fmt.Println("  satisfied by a pointer method rejects a plain T (see module 008)")

	// --- A nil receiver is legal ---
	var empty *m007Tree
	fmt.Printf("Height() on a nil *m007Tree: %d (no panic - the method just sees t == nil)\n",
		empty.Height())
	tree := &m007Tree{Value: 1, Left: &m007Tree{Value: 2}, Right: &m007Tree{Value: 3,
		Right: &m007Tree{Value: 4}}}
	fmt.Printf("Height()=%d Sum()=%d on a real tree\n", tree.Height(), tree.Sum())

	// --- A type that must not be copied ---
	c := &m007SafeCounter{}
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(c.Inc) // Go 1.25: WaitGroup.Go replaces Add(1) + go func(){ defer Done() }()
	}
	wg.Wait()
	fmt.Printf("m007SafeCounter after 100 concurrent Inc(): %d\n", c.Get())
	//	copied := *c // go vet: assignment copies lock value: m007SafeCounter contains sync.Mutex
	fmt.Println("copying it by value is caught by `go vet`'s copylocks check")
}

func m007MethodNames(t reflect.Type) []string {
	names := make([]string, 0, t.NumMethod())
	for i := range t.NumMethod() {
		names = append(names, t.Method(i).Name)
	}
	return names
}

// =================================================================================================
// Section 3: Constructors
// =================================================================================================

/*
## Constructors

- Go has **no constructors**. A struct literal is the construction syntax, and the zero value is
  meant to be usable (module 001a, Section 4) so that most types need nothing more.
- When a type *does* need setup — an initialised map, a validated invariant, an unexported field —
  the convention is a plain function named **`New`** (in a package about one type: `list.New()`) or
  **`NewThing`** (`bytes.NewBuffer`, `http.NewRequest`).
- `New` returns `*T` when the type has pointer methods or must not be copied, and `T` when it is a
  small value type. It returns `(T, error)` when construction can fail — never a partly built value
  with a silently ignored problem.
- Use a **`Must` variant** (`MustCompile`) only for inputs that are compile-time constants in
  practice, so a failure means the program itself is wrong (module 005, Section 6).
- For many optional parameters, use the **functional options** pattern (module 005, Section 5) or a
  config struct. Do not write `NewFoo`, `NewFooWithBar`, `NewFooWithBarAndBaz`.
- Making a type's zero value unusable is a design cost. Prefer to arrange the type so `var x T`
  simply works — that is why `sync.Mutex`, `bytes.Buffer` and `strings.Builder` need no constructor.
*/

type m007Registry struct {
	entries map[string]int // unexported and must be non-nil, so a constructor is warranted
	name    string
}

func m007NewRegistry(name string) *m007Registry {
	return &m007Registry{
		entries: make(map[string]int),
		name:    name,
	}
}

func m007NewValidatedRect(w, h float64) (m007Rect, error) {
	if w <= 0 || h <= 0 {
		return m007Rect{}, fmt.Errorf("m007NewValidatedRect: dimensions must be positive, got %gx%g", w, h)
	}
	return m007Rect{Width: w, Height: h}, nil
}

func (r *m007Registry) Set(k string, v int) { r.entries[k] = v }
func (r *m007Registry) Len() int            { return len(r.entries) }

func m007Constructors() {
	fmt.Println("\n--- Section 3: Constructors ---")

	// The constructor exists because the map field must be non-nil.
	reg := m007NewRegistry("services")
	reg.Set("http", 80)
	reg.Set("https", 443)
	fmt.Printf("m007NewRegistry: name=%s len=%d\n", reg.name, reg.Len())

	// Without it, the zero value would panic on the first Set:
	var broken m007Registry
	fmt.Println("the zero value has a nil map, so Set would panic: assignment to entry in nil map")
	_ = broken

	// Construction that can fail returns an error - never a half-built value.
	if r, err := m007NewValidatedRect(3, 4); err == nil {
		fmt.Printf("m007NewValidatedRect(3, 4) = %v\n", r)
	}
	if _, err := m007NewValidatedRect(-1, 4); err != nil {
		fmt.Printf("m007NewValidatedRect(-1, 4) -> %v\n", err)
	}

	// Most types need no constructor at all, because the zero value works.
	var sb strings.Builder
	var mu sync.Mutex
	sb.WriteString("no constructor needed")
	mu.Lock()
	mu.Unlock()
	fmt.Printf("zero values that just work: strings.Builder(%q), sync.Mutex, bytes.Buffer\n", sb.String())
}

// =================================================================================================
// Section 4: Embedding, Promotion and Shadowing
// =================================================================================================

/*
## Embedding, Promotion and Shadowing

- An **embedded field** is written with a type and no name: `struct { Logger; Name string }`. Its
  field name is the type's own name (`Logger`), and — the point of the whole feature — its fields
  and methods are **promoted** into the outer struct and can be used unqualified.
- **This is not inheritance.** Promotion is a compile-time convenience for writing `outer.Inner.M()`
  as `outer.M()`. There is no virtual dispatch: an embedded type's method that calls another method
  calls **its own**, never an override in the outer type. If you come from Java or C++, this is the
  single most important difference — there is no polymorphism here at all, only delegation.
- You can embed a **value** (`Logger`) or a **pointer** (`*Logger`). A pointer embed lets the inner
  value be shared or nil; a value embed keeps everything in one allocation.
- **Shadowing**: if the outer type declares a field or method of the same name, the outer one wins
  and the inner one is still reachable by its qualified name (`c.Address.Show()`).
- **Ambiguity at the same depth is an error, but only when used.** Embedding two types that both
  have `Show()` compiles fine; calling `outer.Show()` is then
  `ambiguous selector outer.Show`. Qualify it to resolve.
- **Depth wins over breadth**: a name at depth 1 shadows the same name at depth 2, with no error.
- Embedding an **interface** in a struct is a real technique: the struct then satisfies that
  interface, forwarding to whatever was stored, and you can override individual methods. It is how
  test doubles are usually written — but a nil embedded interface panics on any method you did not
  override.
- Embedding is also how you compose interfaces (module 008): `interface { io.Reader; io.Writer }`.
- Beware embedding a type in a struct you serialise: promoted fields are **flattened** by
  `encoding/json`, which is often what you want and occasionally a nasty surprise.
*/

type m007Logger struct{ Prefix string }

func (l m007Logger) Log(msg string) string { return l.Prefix + ": " + msg }

// LogTwice calls l.Log - and it will ALWAYS call m007Logger's Log, never an outer override.
func (l m007Logger) LogTwice(msg string) string { return l.Log(msg) + " / " + l.Log(msg) }

type m007Address struct{ City string }

func (a m007Address) Show() string { return "address in " + a.City }

type m007Client struct {
	m007Logger  // embedded value: Prefix, Log and LogTwice are promoted
	m007Address // embedded value: City and Show are promoted
	Name        string
}

// m007Client declares its own Show, which SHADOWS the promoted m007Address.Show.
func (c m007Client) Show() string { return "client " + c.Name }

// m007LoudLogger tries to "override" Log - and demonstrates that it cannot.
type m007LoudLogger struct {
	m007Logger
}

func (l m007LoudLogger) Log(msg string) string { return strings.ToUpper(l.m007Logger.Log(msg)) }

func m007Embedding() {
	fmt.Println("\n--- Section 4: Embedding, Promotion and Shadowing ---")

	c := m007Client{
		m007Logger:  m007Logger{Prefix: "CLIENT"},
		m007Address: m007Address{City: "Warsaw"},
		Name:        "Ada",
	}

	// Promotion: the embedded fields and methods are reachable unqualified.
	fmt.Printf("promoted field:  c.Prefix = %q, c.City = %q\n", c.Prefix, c.City)
	fmt.Printf("promoted method: c.Log(...) = %q\n", c.Log("connected"))

	// The embedded value is still reachable by its type name.
	fmt.Printf("qualified access: c.m007Logger.Prefix = %q\n", c.m007Logger.Prefix)

	// --- Shadowing ---
	fmt.Printf("c.Show()             = %q  <- the outer method wins\n", c.Show())
	fmt.Printf("c.m007Address.Show() = %q  <- the shadowed one is still reachable\n", c.m007Address.Show())

	// --- Embedding is NOT inheritance: no virtual dispatch ---
	loud := m007LoudLogger{m007Logger{Prefix: "app"}}
	fmt.Printf("loud.Log(\"hi\")      = %q  <- the outer method, called directly\n", loud.Log("hi"))
	fmt.Printf("loud.LogTwice(\"hi\") = %q\n", loud.LogTwice("hi"))
	fmt.Println("  LogTwice is promoted from m007Logger and calls m007Logger's OWN Log,")
	fmt.Println("  not m007LoudLogger's. There is no override - embedding is delegation.")

	// --- Ambiguity ---
	fmt.Println("embedding two types with the same method name compiles;")
	fmt.Println("  calling it unqualified is `ambiguous selector` - qualify to resolve")

	// --- Pointer embedding ---
	p := m007PointerClient{m007Logger: &m007Logger{Prefix: "PTR"}}
	fmt.Printf("pointer embed: p.Log(...) = %q (the inner value can be shared or nil)\n", p.Log("x"))

	// --- Embedding and JSON: promoted fields are flattened ---
	encoded, _ := json.Marshal(m007Wire{m007Address: m007Address{City: "Kraków"}, ID: 7})
	fmt.Printf("json.Marshal flattens promoted fields: %s\n", encoded)
}

type m007PointerClient struct {
	*m007Logger
}

type m007Wire struct {
	m007Address
	ID int `json:"id"`
}

// =================================================================================================
// Section 5: Struct Tags and Reflection
// =================================================================================================

/*
## Struct Tags and Reflection

- A **struct tag** is a raw string literal following a field declaration. To the compiler it is an
  opaque string; it exists purely so that reflection-based libraries can read per-field metadata.
- The **conventional format** is space-separated `key:"value"` pairs, with the value's options
  comma-separated: `json:"name,omitempty" db:"user_name" validate:"required,min=3"`. Use a **raw
  string** (backquotes) so the inner quotes need no escaping.
- Because it is only a string, **a typo is silent**: `json:"nane"` compiles and produces the wrong
  field name. `go vet`'s `structtag` check catches malformed tags — run it.
- The most common keys are `json`, `xml`, `yaml`, `db`/`gorm`, `form`, `validate`. `encoding/json`
  understands:
    - `json:"name"` — rename the field
    - `json:"-"` — never serialise this field (note `json:"-,"` means a field literally named `-`)
    - `json:",omitempty"` — omit when the value is empty (false, 0, nil, or an empty string, map,
      slice or array). Note it does **not** omit an empty *struct*.
    - `json:",omitzero"` — **Go 1.24**: omit when the value is the type's zero value, which fixes
      exactly what `omitempty` could not: a zero `time.Time` or an empty struct.
    - `json:",string"` — encode a number or bool as a JSON string
- **Reflection can only see exported fields.** An unexported field is invisible to `encoding/json`
  and cannot be set through `reflect`. This is not a tag issue and no tag can change it.
- Reflection is powerful and slow, and it moves errors from compile time to run time. Use it where
  it belongs — serialisation, ORMs, validators, test helpers — and reach for generics (module 010)
  or code generation before you write reflective application logic.
*/

type m007Product struct {
	ID       int      `json:"id" db:"product_id" validate:"required"`
	Name     string   `json:"name" db:"name" validate:"required,min=3"`
	Price    float64  `json:"price,string" db:"price"`
	Tags     []string `json:"tags,omitempty" db:"-"`
	Internal string   `json:"-"`
	secret   string
}

func m007TagsAndReflection() {
	fmt.Println("\n--- Section 5: Struct Tags and Reflection ---")

	p := m007Product{ID: 1, Name: "Gopher plush", Price: 29.99, Internal: "not serialised", secret: "hidden"}

	// --- What the tags do at encode time ---
	encoded, _ := json.MarshalIndent(p, "  ", "  ")
	fmt.Printf("  %s\n", encoded)
	fmt.Println("  note: price is a STRING (json:\",string\"), tags is absent (omitempty),")
	fmt.Println("  Internal is excluded (json:\"-\"), and `secret` is invisible because it is unexported")

	// --- Reading tags reflectively, which is exactly what encoding/json does ---
	t := reflect.TypeOf(p)
	fmt.Println("  reflected fields:")
	for f := range t.Fields() { // Go 1.26: Type.Fields returns an iter.Seq[StructField]
		fmt.Printf("    %-9s exported=%-5t json=%-16q db=%-12q validate=%q\n",
			f.Name, f.IsExported(), f.Tag.Get("json"), f.Tag.Get("db"), f.Tag.Get("validate"))
	}

	// Lookup distinguishes "absent" from "present but empty".
	nameField, _ := t.FieldByName("Name")
	if v, ok := nameField.Tag.Lookup("yaml"); !ok {
		fmt.Printf("  Tag.Lookup(\"yaml\") reports absent (Get would have returned %q either way)\n", v)
	}

	// --- omitzero vs omitempty (Go 1.24) ---
	type inner struct{ A int }
	type demo struct {
		EmptyStruct  inner `json:"emptyStruct,omitempty"` // NOT omitted: omitempty ignores structs
		ZeroStruct   inner `json:"zeroStruct,omitzero"`   // omitted: Go 1.24
		EmptySlice   []int `json:"emptySlice,omitempty"`  // omitted
		ExplicitZero int   `json:"explicitZero"`          // present
	}
	out, _ := json.Marshal(demo{})
	fmt.Printf("  omitempty vs omitzero: %s\n", out)
	fmt.Println("  omitzero (Go 1.24) omits a zero struct; omitempty never did")

	// --- Reflection cannot touch unexported fields ---
	v := reflect.ValueOf(&p).Elem()
	fmt.Printf("  CanSet(Name)=%t  CanSet(secret)=%t\n",
		v.FieldByName("Name").CanSet(), v.FieldByName("secret").CanSet())
	v.FieldByName("Name").SetString("renamed through reflection")
	fmt.Printf("  after reflective set: %q\n", p.Name)
}

// =================================================================================================
// Section 6: Comparability, Copying and Anonymous Structs
// =================================================================================================

/*
## Comparability, Copying and Anonymous Structs

- A struct is **comparable with `==` if and only if every field is** (module 002b, Section 12).
  A single slice, map or function field makes the whole struct uncomparable, at compile time.
- A struct with an **interface field** is comparable at compile time but can **panic at runtime** if
  the dynamic value it holds turns out to be uncomparable.
- Comparison is **field by field, including unexported fields**. Two structs from different packages
  can therefore compare unequal for reasons you cannot see — which is why `time.Time` documents that
  you must use `Equal`, not `==`.
- Blank fields (`_ [0]func()`) are a known trick to make a struct **deliberately uncomparable**, so
  that callers cannot depend on `==` and you stay free to add a slice field later.
- **Anonymous structs** — `struct{ Name string; Age int }{...}` — declare a type inline. Their real
  homes are table-driven tests (module 014), one-off JSON payloads, and grouping a few values in a
  channel. Two anonymous struct types are identical if their fields, types **and tags** match.
- Copying: assignment copies every field, but a slice or map field still shares (module 006).
  There is no deep copy in the standard library — write a `Clone` method.
*/

func m007ComparabilityAndAnonymous() {
	fmt.Println("\n--- Section 6: Comparability, Copying and Anonymous Structs ---")

	type comparable1 struct {
		A int
		B string
		C [2]int
	}
	x := comparable1{1, "a", [2]int{1, 2}}
	y := comparable1{1, "a", [2]int{1, 2}}
	fmt.Printf("all-comparable fields: x == y is %t\n", x == y)

	type withSlice struct {
		A int
		S []int
	}
	//	fmt.Println(withSlice{} == withSlice{}) // ERROR: invalid operation: withSlice{} == withSlice{} (struct containing []int cannot be compared)
	_ = withSlice{}
	fmt.Println("one slice field makes the whole struct uncomparable, at compile time")

	// An interface field: compiles, but can panic.
	type withAny struct{ V any }
	fmt.Printf("interface field holding ints:   %t\n",
		m003SafeCompare(withAny{V: 1}, withAny{V: 1}) == "true")
	fmt.Printf("interface field holding slices: %v\n",
		m003SafeCompare(withAny{V: []int{1}}, withAny{V: []int{1}}))

	// Deliberately uncomparable, to keep the option open.
	type futureProof struct {
		_ [0]func() // a zero-size, uncomparable field
		A int
	}
	//	fmt.Println(futureProof{A: 1} == futureProof{A: 1}) // ERROR: struct containing [0]func() cannot be compared
	_ = futureProof{A: 1}
	fmt.Println("`_ [0]func()` makes a struct uncomparable on purpose, at zero cost")

	// --- Anonymous structs ---
	config := struct {
		Host string
		Port int
	}{Host: "localhost", Port: 8080}
	fmt.Printf("anonymous struct: %+v (%T)\n", config, config)

	// The idiomatic use: a table-driven test case list (module 014).
	cases := []struct {
		name string
		in   m007Rect
		want float64
	}{
		{"unit", m007Rect{1, 1}, 1},
		{"3x4", m007Rect{3, 4}, 12},
	}
	for _, c := range cases {
		fmt.Printf("  case %-5s Area()=%g want=%g pass=%t\n", c.name, c.in.Area(), c.want,
			c.in.Area() == c.want)
	}

	// A one-off JSON payload, without polluting the package with a named type.
	payload, _ := json.Marshal(struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}{Status: "ok", Count: 2})
	fmt.Printf("  one-off JSON payload: %s\n", payload)
}

// Run007 runs every section of module 007 in order.
func Run007() {
	m007Methods()
	m007Receivers()
	m007Constructors()
	m007Embedding()
	m007TagsAndReflection()
	m007ComparabilityAndAnonymous()
}

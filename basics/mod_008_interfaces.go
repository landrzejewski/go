package basics

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

/*
# Module 008 — Interfaces

Interfaces are the whole of Go's polymorphism, and they work differently from almost every other
language's: **satisfaction is implicit**. A type implements an interface merely by having the right
methods. It never declares that it does, and — crucially — it does not even need to know the
interface exists.

The consequence is that **interfaces belong to the consumer, not the producer**. You define the
interface where it is *used*, listing only what you need, and every existing type that happens to
fit becomes usable. This inverts the dependency direction compared with Java or C#, and it is why
Go code has so few "IFoo" abstraction layers.
*/

// =================================================================================================
// Section 1: Implicit Satisfaction
// =================================================================================================

/*
## Implicit Satisfaction

- An interface type lists **method signatures**. Any type whose method set includes all of them
  satisfies it — no `implements` keyword, no registration, no inheritance.
- Because satisfaction is structural, you can write an interface for a type you do not own:
  `*os.File` satisfies your `Closer` interface without `os` knowing anything about it.
- The **method set rule from module 007 applies here and is the usual stumbling block**: a method
  with a pointer receiver is in the method set of `*T` only. So if any method has a pointer
  receiver, `T` does not satisfy the interface and `*T` does, and you get
  `X does not implement I (method M has pointer receiver)`.
- The **compile-time assertion** `var _ I = (*T)(nil)` documents and enforces that `*T` implements
  `I`, with no runtime cost. Put it next to the type when the relationship matters and nothing else
  in the package would catch a regression.
- **Interfaces should be small.** The most-used interfaces in the standard library have exactly one
  method: `io.Reader`, `io.Writer`, `fmt.Stringer`, `error`, `sort.Interface` (three, and considered
  large). The proverb is: *the bigger the interface, the weaker the abstraction*.
- Naming: a one-method interface is conventionally the method name plus `-er` — `Reader`, `Writer`,
  `Stringer`, `Formatter`, `Closer`.
*/

// A tiny consumer-side interface. Nothing declares that it implements this.
type m008Shape interface {
	Area() float64
}

// A second one, composed from the first in Section 4.
type m008Named interface {
	Name() string
}

type m008Circle struct{ R float64 }
type m008Square struct{ Side float64 }

func (c m008Circle) Area() float64 { return 3.14159265358979 * c.R * c.R }
func (c m008Circle) Name() string  { return "circle" }
func (s m008Square) Area() float64 { return s.Side * s.Side }
func (s m008Square) Name() string  { return "square" }

// A type whose method has a POINTER receiver - so only *m008Counter satisfies m008Shape-like
// interfaces that include it.
type m008Growing struct{ n float64 }

func (g *m008Growing) Area() float64 { g.n++; return g.n }

// Compile-time assertions. These cost nothing and fail the build if the type drifts.
var (
	_ m008Shape = m008Circle{}
	_ m008Shape = (*m008Growing)(nil) // note: the POINTER, because Area has a pointer receiver
)

func m008ImplicitSatisfaction() {
	fmt.Println("--- Section 1: Implicit Satisfaction ---")

	// Both types satisfy m008Shape without ever mentioning it.
	shapes := []m008Shape{m008Circle{R: 1}, m008Square{Side: 2}, &m008Growing{}}
	for _, s := range shapes {
		fmt.Printf("  %-22T Area()=%.4f\n", s, s.Area())
	}

	// --- Satisfying an interface for a type you do not own ---
	// *os.File, *bytes.Buffer and *strings.Reader all satisfy io.Reader without coordination.
	var readers = []io.Reader{
		strings.NewReader("from a strings.Reader"),
		bytes.NewBufferString("from a bytes.Buffer"),
	}
	for _, r := range readers {
		data, _ := io.ReadAll(r)
		fmt.Printf("  io.Reader: %q\n", data)
	}

	// --- The pointer-receiver trap ---
	//	var s m008Shape = m008Growing{} // ERROR: cannot use m008Growing{} (value of struct type m008Growing) as m008Shape value in variable declaration: m008Growing does not implement m008Shape (method Area has pointer receiver)
	var ok m008Shape = &m008Growing{}
	fmt.Printf("  m008Growing needs a POINTER to satisfy m008Shape: %.0f\n", ok.Area())
	fmt.Println("  \"method Area has pointer receiver\" is the most common interface error in Go")

	// --- Small interfaces ---
	fmt.Println("  the standard library's most-used interfaces have ONE method:")
	fmt.Println("    io.Reader, io.Writer, io.Closer, fmt.Stringer, error")
}

// =================================================================================================
// Section 2: Interface Values, and the nil Trap
// =================================================================================================

/*
## Interface Values, and the nil Trap

- An interface value is **two words**: a *type descriptor* and a *data pointer*. It is written
  informally as the pair `(T, value)`.
- An interface is `nil` **only when both words are nil** — that is, when nothing has ever been
  assigned to it.

### The typed-nil trap

Assigning a **nil pointer** to an interface gives you `(*T, nil)` — a non-nil interface holding a
nil pointer. So:

	var p *MyError = nil
	var err error = p
	err == nil            // FALSE — the interface holds the type *MyError

This is the most notorious gotcha in Go, and it almost always appears in this shape:

	func f() error {
	    var e *MyError      // nil
	    if somethingWrong { e = &MyError{} }
	    return e            // ALWAYS non-nil as an error, even when e is nil!
	}

The fix is to **return a literal `nil`** on the success path, and never to declare a concrete error
pointer as the return variable:

	func f() error {
	    if somethingWrong { return &MyError{} }
	    return nil
	}

- A method call on an interface holding a nil pointer **does not necessarily panic** — the method
  runs with a nil receiver, and only a dereference inside it panics (module 007, Section 2).
- Interface values are **comparable**: equal when both the dynamic type and the dynamic value are
  equal. Comparing two interfaces holding an uncomparable dynamic type (a slice, map or function)
  **panics at runtime**.
- `%v` on an interface prints the dynamic value; `%T` prints the dynamic type. These are the two
  fastest debugging tools for this whole section.
*/

type m008MyError struct{ Msg string }

func (e *m008MyError) Error() string {
	if e == nil {
		return "<nil *m008MyError>" // a nil receiver is fine as long as we do not dereference
	}
	return e.Msg
}

// The bug, written out.
func m008BuggyReturn(fail bool) error {
	var e *m008MyError // nil pointer of a CONCRETE type
	if fail {
		e = &m008MyError{Msg: "real failure"}
	}
	return e // converting a nil *m008MyError to error gives a NON-nil interface
}

// The fix.
func m008CorrectReturn(fail bool) error {
	if fail {
		return &m008MyError{Msg: "real failure"}
	}
	return nil // a literal nil interface
}

func m008InterfaceValues() {
	fmt.Println("\n--- Section 2: Interface Values, and the nil Trap ---")

	// --- The two words ---
	var i any
	fmt.Printf("  var i any        -> value=%v type=%T isNil=%t\n", i, i, i == nil)
	i = 42
	fmt.Printf("  i = 42           -> value=%v type=%T isNil=%t\n", i, i, i == nil)
	i = m008Circle{R: 1}
	fmt.Printf("  i = m008Circle{} -> value=%v type=%T isNil=%t\n", i, i, i == nil)

	// --- The typed-nil trap ---
	var p *m008MyError
	var asError error = p
	fmt.Printf("  var p *m008MyError = nil; var e error = p\n")
	fmt.Printf("    p == nil:   %t\n", p == nil)
	fmt.Printf("    e == nil:   %t   <- the interface holds (%T, nil), so it is NOT nil\n", asError == nil, asError)

	// The bug in the wild.
	if err := m008BuggyReturn(false); err != nil {
		fmt.Printf("    m008BuggyReturn(false) reports an error even though nothing failed: %v\n", err)
	}
	if err := m008CorrectReturn(false); err == nil {
		fmt.Println("    m008CorrectReturn(false) correctly returns a nil error")
	}
	fmt.Println("    the fix: return a literal nil, never a nil pointer of a concrete error type")

	// --- Comparison ---
	var a, b any = 1, 1
	fmt.Printf("  two interfaces holding the same int: %t\n", a == b)
	var c, d any = 1, int64(1)
	fmt.Printf("  int(1) vs int64(1) in interfaces:     %t  <- different dynamic types\n", c == d)
	fmt.Printf("  two interfaces holding slices:        %v\n",
		m003SafeCompare([]int{1}, []int{1}))
}

// =================================================================================================
// Section 3: Type Assertions and Type Switches
// =================================================================================================

/*
## Type Assertions and Type Switches

- A **type assertion** `x.(T)` extracts the dynamic value from an interface.
    - Single-value form `v := x.(T)` **panics** if the dynamic type is not `T`.
    - **Comma-ok form** `v, ok := x.(T)` never panics: on failure `ok` is `false` and `v` is `T`'s
      zero value. Use this form unless a failure genuinely means the program is broken.
- Asserting to an **interface type** asks whether the dynamic type also satisfies *that* interface.
  This is how optional behaviour is discovered at runtime: `if c, ok := r.(io.Closer); ok { c.Close() }`.
  The standard library uses it constantly — `io.Copy` checks for `ReaderFrom` and `WriterTo` to take
  a fast path.
- A **type switch** `switch v := x.(type)` handles several cases at once. Inside each `case`, `v`
  has that case's type. In a `case` listing **several types**, `v` keeps the original interface type,
  because there is no single type it could have.
- `case nil` matches a nil interface value. `default` catches everything else.
- Type switching on a **concrete** type set is a design smell: it means you are re-implementing
  dispatch that a method would do for you. Type switching on **interfaces** to discover optional
  capabilities is idiomatic.
- **Go 1.25** added `reflect.TypeAssert[T](v)`, a generic, allocation-free way to do the same thing
  from a `reflect.Value` — useful in reflective code, not in ordinary code.
*/

func m008TypeAssertions() {
	fmt.Println("\n--- Section 3: Type Assertions and Type Switches ---")

	var x any = "a string"

	// Comma-ok: safe.
	if s, ok := x.(string); ok {
		fmt.Printf("  comma-ok assertion succeeded: %q\n", s)
	}
	if n, ok := x.(int); !ok {
		fmt.Printf("  comma-ok assertion failed safely: n=%d ok=%t\n", n, ok)
	}

	// Single-value: panics on mismatch.
	fmt.Printf("  single-value assertion to the wrong type: %v\n",
		m005CatchPanic(func() { _ = x.(int) }))

	// --- Asserting to an INTERFACE discovers optional behaviour ---
	// *os.File has implemented io.ReaderFrom since Go 1.15, so pair *bytes.Buffer
	// with something that genuinely does not: *strings.Builder only has Write.
	for _, w := range []io.Writer{&bytes.Buffer{}, &strings.Builder{}} {
		if _, ok := w.(io.ReaderFrom); ok {
			fmt.Printf("  %-16T also implements io.ReaderFrom (io.Copy takes a fast path)\n", w)
		} else {
			fmt.Printf("  %-16T does not implement io.ReaderFrom\n", w)
		}
	}

	// --- Type switch ---
	for _, v := range []any{42, "text", 3.14, true, []int{1}, m008Circle{R: 2}, nil} {
		fmt.Printf("  %-14s -> %s\n", fmt.Sprint(v), m008Describe(v))
	}
}

func m008Describe(v any) string {
	switch t := v.(type) {
	case nil:
		return "a nil interface"
	case int:
		return fmt.Sprintf("an int, doubled it is %d", t*2)
	case string:
		return fmt.Sprintf("a string of length %d", len(t))
	case float64, float32:
		// Several types in one case, so t keeps the INTERFACE type `any`.
		return fmt.Sprintf("some float, still typed as %T here", t)
	case m008Shape:
		// A case can be an INTERFACE - this is the idiomatic use of a type switch.
		return fmt.Sprintf("something with an Area(): %.2f", t.Area())
	default:
		return fmt.Sprintf("something else (%T)", t)
	}
}

// =================================================================================================
// Section 4: Interface Embedding and Composition
// =================================================================================================

/*
## Interface Embedding and Composition

- An interface may **embed** other interfaces, which simply unions their method sets:

	type ReadWriter interface { Reader; Writer }

  This is how `io.ReadWriter`, `io.ReadCloser`, `io.ReadWriteCloser` and `io.ReadWriteSeeker` are
  defined — every one of them is composed from the one-method interfaces.
- Overlapping method names are allowed since Go 1.14, provided the signatures are identical.
- Since Go 1.18 an interface may also contain **type elements** (`~int | ~string`) as well as
  methods. Such an interface can only be used as a **generic constraint**, never as a value type —
  `var x MyConstraint` is a compile error. Module 010 covers this.
- **Embedding an interface in a struct** is a distinct and useful technique: the struct then
  satisfies the interface by forwarding, and you can override individual methods. It is the standard
  way to write a test double for a large interface without implementing every method — but any
  method you did **not** override panics with a nil pointer dereference if the embedded interface is
  nil, so it is a sharp tool.
- The design guidance: **compose small interfaces at the point of use**. A function that only reads
  should take an `io.Reader`, not a `*os.File`. This is what makes Go code trivially testable —
  `strings.NewReader` substitutes for a file with no mocking framework at all.
*/

type m008Reader interface{ Read() string }
type m008Writer interface{ Write(string) }

// Composed by embedding.
type m008ReadWriter interface {
	m008Reader
	m008Writer
}

// Composed from the interfaces declared in Section 1.
type m008NamedShape interface {
	m008Shape
	m008Named
}

type m008Memory struct{ data []string }

func (m *m008Memory) Read() string   { return strings.Join(m.data, ",") }
func (m *m008Memory) Write(s string) { m.data = append(m.data, s) }

// Embedding an interface in a struct: a test double that overrides one method.
type m008FailingWriter struct {
	io.Writer // embedded interface; nil here, and that is deliberate
}

func (f m008FailingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("simulated write failure")
}

func m008InterfaceComposition() {
	fmt.Println("\n--- Section 4: Interface Embedding and Composition ---")

	// A composed interface.
	var rw m008ReadWriter = &m008Memory{}
	rw.Write("a")
	rw.Write("b")
	fmt.Printf("  m008ReadWriter (embeds Reader and Writer): %q\n", rw.Read())

	// A value satisfying a composed interface can be narrowed to either half.
	var justReader m008Reader = rw
	fmt.Printf("  narrowed to m008Reader: %q\n", justReader.Read())

	// Composition from Section 1's interfaces.
	var ns m008NamedShape = m008Circle{R: 1}
	fmt.Printf("  m008NamedShape: %s with area %.4f\n", ns.Name(), ns.Area())

	// The standard library's io interfaces are built exactly this way.
	fmt.Println("  io.ReadWriteCloser is literally: interface { Reader; Writer; Closer }")

	// --- Embedding an interface in a struct: the test-double technique ---
	var w io.Writer = m008FailingWriter{}
	_, err := w.Write([]byte("anything"))
	fmt.Printf("  a test double overriding one method: %v\n", err)
	fmt.Println("  any method it does NOT override would panic - the embedded interface is nil")

	// --- Accept interfaces, return structs ---
	fmt.Printf("  m008CountLines over a string: %d\n", m008CountLines(strings.NewReader("a\nb\nc")))
	fmt.Printf("  the same function over a buffer: %d\n",
		m008CountLines(bytes.NewBufferString("x\ny")))
	fmt.Println("  taking io.Reader instead of *os.File is what makes this testable with no mocks")
}

// m008CountLines takes the smallest interface it needs - the whole point of the style.
func m008CountLines(r io.Reader) int {
	data, err := io.ReadAll(r)
	if err != nil || len(data) == 0 {
		return 0
	}
	return strings.Count(string(data), "\n") + 1
}

// =================================================================================================
// Section 5: The Empty Interface and any
// =================================================================================================

/*
## The Empty Interface and any

- `interface{}` has no methods, so **every** type satisfies it. Since Go 1.18 `any` is a
  **predeclared alias** for it — the same type, a better name. Write `any` in new code.
- Because it says nothing, a value in an `any` supports **no operations at all** until you assert or
  type-switch it back. `any` is not a supertype and not a dynamic type: it is an erasure.
- Putting a value in an `any` usually **allocates**, because the interface's data word must point at
  something. This is why `fmt.Println(x)` costs an allocation for a non-pointer `x`, and why hot
  loops avoid `any`.
- Legitimate uses: `fmt`'s variadic parameters, `encoding/json` decoding into an unknown shape,
  containers written before generics existed, and `context.WithValue`.
- **Since Go 1.18, most of the old uses are better served by generics** (module 010): a
  `Stack[T]` keeps its element type, needs no assertion and no boxing, where a `Stack` of `any`
  loses the type and pushes every error to run time.
- Decoding arbitrary JSON gives you a documented set of dynamic types, and you must handle them:
  `bool`, `float64` (**every** number, including integers), `string`, `nil`, `[]any`,
  `map[string]any`. Forgetting that `42` decodes as `float64` is a classic bug.
*/

func m008EmptyInterface() {
	fmt.Println("\n--- Section 5: The Empty Interface and any ---")

	// any and interface{} are the SAME type - an alias, not a defined type.
	var a any = 1
	var b interface{} = a // no conversion needed
	fmt.Printf("  any and interface{} are one type: %T holds %v\n", b, b)

	// It supports no operations until you get the type back.
	//	fmt.Println(a + 1) // ERROR: invalid operation: a + 1 (mismatched types any and untyped int)
	fmt.Printf("  a + 1 does not compile; a.(int) + 1 = %d\n", a.(int)+1)

	// --- Decoding unknown JSON ---
	var decoded any
	_ = json.Unmarshal([]byte(`{"name":"Ada","age":36,"tags":["x"],"active":true,"note":null}`), &decoded)
	obj := decoded.(map[string]any)
	fmt.Println("  arbitrary JSON decodes to a documented set of dynamic types:")
	for _, k := range []string{"name", "age", "tags", "active", "note"} {
		fmt.Printf("    %-7s -> %-14T %v\n", k, obj[k], obj[k])
	}
	fmt.Println("    note that 36 arrived as a float64 - EVERY JSON number does")

	// --- any versus generics ---
	anyStack := []any{}
	anyStack = append(anyStack, 1, "two", 3.0)
	fmt.Printf("  a []any container loses the element type: %v\n", anyStack)
	fmt.Println("  every read needs an assertion, and a wrong one panics at run time")

	typed := &m010Stack[int]{} // the generic version from module 010
	typed.Push(1)
	typed.Push(2)
	v, _ := typed.Pop()
	fmt.Printf("  the generic m010Stack[int] keeps the type: Pop() = %d (%T), no assertion\n", v, v)
}

// =================================================================================================
// Section 6: Standard Library Interfaces Worth Knowing
// =================================================================================================

/*
## Standard Library Interfaces Worth Knowing

Implementing these gets your type working with large parts of the ecosystem for free.

  - **`error`** — `Error() string`. Module 009.
  - **`fmt.Stringer`** — `String() string`. Makes `%v`, `%s` and `Println` print your type nicely.
    Beware: calling a formatting verb on the receiver *inside* `String()` recurses infinitely; use
    a conversion to the underlying type to break the cycle.
  - **`io.Reader` / `io.Writer`** — the most important pair in the standard library. Satisfying them
    plugs your type into `io.Copy`, `bufio`, `encoding/*`, `net/http`, compression, hashing.
  - **`io.Closer`, `io.Seeker`, `io.ReaderFrom`, `io.WriterTo`** — optional capabilities discovered
    by type assertion.
  - **`sort.Interface`** — `Len`, `Less`, `Swap`. Since Go 1.21 `slices.SortFunc` is the better
    choice for ordinary sorting; `sort.Interface` remains useful for sorting something that is not
    a slice.
  - **`json.Marshaler` / `json.Unmarshaler`** — custom JSON representation.
  - **`encoding.TextMarshaler` / `TextUnmarshaler`** — one implementation that serves JSON, XML,
    and map keys at once. Usually the better choice.
  - **`fmt.Formatter`, `fmt.GoStringer`** — full control over `%v` and `%#v`. Rarely needed.
  - **`context.Context`** — not something you implement; something you accept as the first parameter.

A type implementing `Stringer` **and** `error` is a common and useful combination; a type
implementing `TextMarshaler` gets JSON, YAML and map-key encoding from one method.
*/

type m008Temperature float64

// Stringer: note the conversion to float64, which prevents infinite recursion.
func (t m008Temperature) String() string { return fmt.Sprintf("%.1f°C", float64(t)) }

// TextMarshaler/TextUnmarshaler: one pair serving JSON, XML and map keys.
func (t m008Temperature) MarshalText() ([]byte, error) { return []byte(t.String()), nil }

func (t *m008Temperature) UnmarshalText(b []byte) error {
	var f float64
	if _, err := fmt.Sscanf(string(b), "%f°C", &f); err != nil {
		return fmt.Errorf("m008Temperature: parsing %q: %w", b, err)
	}
	*t = m008Temperature(f)
	return nil
}

// A Writer that upper-cases everything passing through it.
type m008UpperWriter struct{ w io.Writer }

func (u m008UpperWriter) Write(p []byte) (int, error) {
	n, err := u.w.Write([]byte(strings.ToUpper(string(p))))
	return n, err
}

// sort.Interface on something that is not a plain slice.
type m008ByArea []m008Shape

func (s m008ByArea) Len() int           { return len(s) }
func (s m008ByArea) Less(i, j int) bool { return s[i].Area() < s[j].Area() }
func (s m008ByArea) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func m008StdlibInterfaces() {
	fmt.Println("\n--- Section 6: Standard Library Interfaces Worth Knowing ---")

	// Stringer.
	t := m008Temperature(21.5)
	fmt.Printf("  fmt.Stringer: %v / %s\n", t, t)
	fmt.Println("  inside String(), convert to the underlying type or the verb recurses forever")

	// TextMarshaler - one method pair, three encodings.
	encodedValue, _ := json.Marshal(map[string]m008Temperature{"outside": t})
	fmt.Printf("  encoding.TextMarshaler powers JSON values: %s\n", encodedValue)
	// A map KEY must encode to a string, and TextMarshaler is what supplies it.
	encodedKey, _ := json.Marshal(map[m008Temperature]string{t: "outside"})
	fmt.Printf("  ...and JSON map keys, via the same method: %s\n", encodedKey)
	var parsed m008Temperature
	_ = parsed.UnmarshalText([]byte("18.0°C"))
	fmt.Printf("  TextUnmarshaler round-trip: %v\n", parsed)

	// io.Writer: a decorator, composable with anything.
	var buf bytes.Buffer
	upper := m008UpperWriter{w: &buf}
	fmt.Fprint(upper, "wrapping an io.Writer is trivial")
	fmt.Printf("  io.Writer decorator: %q\n", buf.String())

	// io.Copy works between any Reader and any Writer.
	var dst bytes.Buffer
	n, _ := io.Copy(&dst, strings.NewReader("io.Copy joins any Reader to any Writer"))
	fmt.Printf("  io.Copy moved %d bytes: %q\n", n, dst.String())

	// sort.Interface.
	shapes := m008ByArea{m008Square{Side: 3}, m008Circle{R: 1}, m008Square{Side: 1}}
	sort.Sort(shapes)
	fmt.Print("  sort.Interface, ascending by area: ")
	for _, s := range shapes {
		fmt.Printf("%.2f ", s.Area())
	}
	fmt.Println()
	fmt.Println("  for plain slices prefer slices.SortFunc (Go 1.21) - see module 012")
}

// Run008 runs every section of module 008 in order.
func Run008() {
	m008ImplicitSatisfaction()
	m008InterfaceValues()
	m008TypeAssertions()
	m008InterfaceComposition()
	m008EmptyInterface()
	m008StdlibInterfaces()
}

package basics

import (
	"fmt"
	"os"
	"strings"
)

/*
# Module 001a — Comments, Packages, Variables, Scope, Shadowing

This is the first module of the Go language course. Every module in this package follows the same
shape: a banner, a Markdown theory block, then one runnable demo function per section. `Run001a`
at the bottom calls them all in order.

## A note on the `mNNN` prefixes you will see

In Rust each module is its own namespace, so every file can have a private `fn run()`. Go is
different: **all files of a package share one flat namespace**. `basics` is a single package spread
over two dozen files, so a package-level `add` in module 005 would collide with one in
module 010 — the compiler reports `add redeclared in this block`.

The convention here is therefore: every **package-level** identifier is prefixed with its module
number (`m001aGreeting`, `m007Person`, `m009ErrNotFound`). Identifiers **local to a function** need
no prefix and stay perfectly idiomatic — and that is the great majority of the code you will read.

That is itself the first lesson: a Go package is one namespace, not one file.
*/

// =================================================================================================
// Section 1: Comments
// =================================================================================================

// ## Comments
//
// This one theory block is written with line comments rather than the `/* ... */` form used by every
// other section — precisely because it has to talk about the block-comment terminator, which cannot
// appear inside a block comment.
//
//   - Go has exactly two comment forms: **line comments** `// ...`, running to the end of the line,
//     and **general (block) comments** opened with slash-star and closed with star-slash.
//   - Block comments **do not nest**. The first star-slash closes the comment, however many
//     slash-stars came before it. This differs from Rust, where block comments nest freely. To
//     comment out a region that already contains a block comment you must use line comments — which
//     is why Go-aware editors comment out a selection by prefixing every line with `//` rather than
//     by wrapping it.
//   - A general comment containing no newlines acts like a **space**; any other comment acts like a
//     **newline**. That matters because of semicolon insertion (Section 3): a comment can therefore
//     terminate a statement.
//   - **Doc comments** are not a separate syntax. A comment written *immediately* before a top-level
//     declaration, with no blank line between them, is that declaration's documentation and is what
//     `go doc` and pkg.go.dev display. There is no `///` or `//!` as in Rust.
//   - Convention: a doc comment **begins with the name of the thing it documents** — `// Greet
//     returns ...`, not `// This function returns ...`. `go vet` does not enforce it, but every
//     reviewer will.
//   - A **package comment** precedes the `package` clause. In a multi-file package only one file
//     should carry it, by convention a `doc.go`.
//   - Since Go 1.19 doc comments support a little structured markup: blank-line-separated
//     paragraphs, indented code blocks, `# Headings`, lists, and `[Name]` / `[pkg.Name]` links.
//     `gofmt` reformats doc comments to canonical form.
//   - **Directive comments** are magic and must have **no space** after the `//`: `//go:build
//     linux`, `//go:embed file.txt`, `//go:generate ...`, `//go:noinline`. `// go:build` with a
//     space is just an ordinary comment and is silently ignored — a classic and very quiet bug.

// m001aGreet returns a greeting for name.
//
// This doc comment demonstrates the conventions described above: it starts with the function's own
// name, and the paragraph you are reading is separated by a blank comment line.
//
// Since Go 1.19 a doc comment may link to other identifiers, like [m001aComments], and show
// indented code blocks:
//
//	fmt.Println(m001aGreet("Go"))
func m001aGreet(name string) string {
	return "Hello, " + name + "!"
}

func m001aComments() {
	fmt.Println("--- Section 1: Comments ---")

	// This is a line comment. It runs to the end of the line.
	fmt.Println(m001aGreet("Go")) // a line comment may also trail a statement

	/*
	   This is a general (block) comment.
	   It spans several lines.
	*/

	// Block comments do NOT nest. Uncommenting the following would fail, because the inner `*/`
	// closes the whole comment and the trailing `*/` is then stray syntax:
	//
	//	/* outer /* inner */ still outer? */
	//
	// ERROR: expected statement, found '*'

	// A general comment with no newline in it behaves like a space, so it can sit mid-expression:
	sum := 2 /* the first operand */ + 3
	fmt.Println("2 /* comment */ + 3 =", sum)

	// Use `go doc` to read doc comments from the command line:
	//	go doc training.pl/go/basics
	//	go doc training.pl/go/basics.Run001a
	fmt.Println("run `go doc training.pl/go/basics` to see the doc comments of this package")

	// --- Directive comments ---
	// Directives must have no space after `//`. These are real, load-bearing lines elsewhere in
	// this repo; here they are shown only as text so they stay inert:
	fmt.Println("directive examples: //go:build linux | //go:embed data.txt | //go:generate stringer")
	fmt.Println("`// go:build linux` (with a space) is NOT a directive - it is an ignored comment")
}

// =================================================================================================
// Section 2: Packages and Imports
// =================================================================================================

/*
## Packages and Imports

- Every Go source file begins with a `package` clause. All files in one directory must declare the
  **same package name**, and a directory is the unit of compilation.
- The package **name** and its **import path** are independent. `import "math/rand"` binds the name
  `rand`; `import "gopkg.in/yaml.v3"` binds `yaml`, not `v3`. Never guess the name from the path —
  read the package clause (or let your editor do it).
- Package `main` is special: it produces an executable rather than a library, and must contain
  `func main()`.
- **Visibility is decided by the first letter of the identifier.** Upper case = exported outside the
  package; lower case = package-private. There is no `pub`, `private` or `public` keyword. The rule
  applies to every top-level name, and also to struct fields and methods.
- Visibility is **per package, not per file**: a lowercase identifier is visible to *all* files of
  its package. That is exactly why this package needs the `mNNN` prefixes.
- A directory named `internal` restricts its subtree: only code rooted at `internal`'s parent may
  import it. This is enforced by the toolchain, not by convention.
- **Unused imports are a compile error**, not a warning. So is an unused local variable (Section 5).

### The four import forms

	import "fmt"          // normal:  refer to it as fmt.Println
	import f "fmt"        // alias:   refer to it as f.Println
	import . "fmt"        // dot:     refer to it as Println, unqualified
	import _ "image/png"  // blank:   do not refer to it at all; run its init() for side effects

- The **dot import** dumps a package's exported names into the file scope. It shadows silently and
  makes code unreadable; the only sanctioned use is inside tests of the package itself. Avoid it.
- The **blank import** is for packages whose *only* purpose is a registration side effect: database
  drivers (`_ "github.com/lib/pq"`, as in this repo's `examples/rest.go`), image format decoders,
  `net/http/pprof`. It silences only the "imported and not used" error — the package's `init()`
  functions still run (Section 10, module 001b).
- Import **cycles are forbidden**. If A imports B, B may not import A, directly or transitively.
  There is no forward declaration to escape this; you must extract the shared part into a third
  package, or invert the dependency with an interface (module 008).
- `gofmt` groups and sorts imports; `goimports` additionally adds and removes them. The universal
  convention is: standard library first, blank line, then everything else.
*/

func m001aPackagesAndImports() {
	fmt.Println("\n--- Section 2: Packages and Imports ---")

	// `strings` is imported normally, so it is qualified by the package name.
	fmt.Println("strings.ToUpper:", strings.ToUpper("packages are namespaces"))

	// The package NAME comes from the package clause, not the last path element. `os` happens to
	// match, but e.g. "gopkg.in/yaml.v3" binds `yaml` and "math/rand/v2" binds `rand`.
	fmt.Println("os.Args[0] (the running binary):", os.Args[0])

	// Visibility: `m001aGreet` is lowercase, so it is invisible outside package `basics`.
	// `Run001a` is uppercase, so main.go can call it. Same file, same package, different reach.
	fmt.Println("m001aGreet is package-private; Run001a is exported - only the case differs")

	// Visibility is per PACKAGE, not per file: this line calls a function declared in a different
	// file of package `basics` without any import at all.
	fmt.Println("calling across files in the same package:", m001bConstantsSummary())

	// An unused import is a compile error:
	//	import "bytes" // ERROR: "bytes" imported and not used
}

// =================================================================================================
// Section 3: Variables and Type Inference
// =================================================================================================

/*
## Variables and Type Inference

- Two ways to introduce a variable:
    1. `var name Type = value` — usable anywhere, including at package level. Either the type or
       the value may be omitted, but not both.
    2. `name := value` — the **short variable declaration**. Concise, infers the type, and is
       **only legal inside a function**. There is no `:=` at package level.
- Go's declaration syntax puts the **name before the type** (`var x int`, not `int x`). The stated
  reason is that it reads left-to-right for complex types: `var f func(int) (string, error)` is far
  easier to parse than the C equivalent.
- Multiple declarations can be grouped in a `var ( ... )` block, and several variables can be
  declared or assigned at once: `var a, b int = 1, 2` or `a, b := 1, 2`.
- **Swapping needs no temporary**: `a, b = b, a`. The right-hand side is fully evaluated before any
  assignment happens.
- Inference uses the **default type of an untyped constant**: an integer literal defaults to `int`,
  a float literal to `float64`, `'x'` to `rune` (= `int32`), `"s"` to `string`, `true` to `bool`.
  So `x := 5` gives an `int` even on a 64-bit machine where `int64` might seem more natural, and
  `y := 5.0` gives a `float64`, never a `float32`.
- To get a different type you must say so: `var x int64 = 5`, `x := int64(5)`, or `x := float32(5)`.
- **Semicolons**: the grammar requires them, but the scanner inserts them automatically at the end
  of any line ending in an identifier, a literal, one of `break continue fallthrough return ++ --`,
  or a closing `) ] }`. This is why the opening brace **must** be on the same line as `func`, `if`
  and `for` — put it on the next line and a semicolon is inserted before it, silently changing the
  meaning of the program.

### `var` or `:=`?

- Inside functions, prefer `:=`. It is the community default and reads best.
- Use `var` when you want the **zero value** and no initialiser (`var buf bytes.Buffer`), when you
  need an **explicit type** that inference would get wrong (`var x float32 = 5`), when declaring at
  **package level** (where `:=` is illegal), or when you must declare **before** a branch so the
  variable outlives it.
*/

func m001aVariablesAndTypeInference() {
	fmt.Println("\n--- Section 3: Variables and Type Inference ---")

	// Full form: name, type and value.
	var explicit int = 42
	// The type may be omitted - it is inferred from the value.
	var inferred = 42
	// The value may be omitted - the variable gets its zero value (Section 4).
	var zeroed int
	// Short declaration: the idiomatic choice inside a function.
	short := 42

	fmt.Printf("explicit=%d inferred=%d zeroed=%d short=%d\n", explicit, inferred, zeroed, short)

	// Grouped declaration.
	var (
		name    = "Go"
		version = 1.27
		stable  = true
	)
	fmt.Printf("name=%s version=%v stable=%t\n", name, version, stable)

	// Several variables at once, and the two-value swap.
	a, b := 1, 2
	a, b = b, a // the right-hand side is evaluated in full before anything is assigned
	fmt.Println("after swap: a =", a, "b =", b)

	// --- Default types of untyped constants ---
	// Inference does not pick the "smallest fitting" type; it uses the literal's DEFAULT type.
	i := 5      // int, not int8 and not int64
	f := 5.0    // float64, never float32
	r := 'G'    // rune, i.e. int32 - and it prints as the NUMBER 71, not as "G"
	s := "Go"   // string
	t := true   // bool
	c := 1 + 2i // complex128
	fmt.Printf("%T %T %T %T %T %T\n", i, f, r, s, t, c)
	fmt.Printf("r := 'G' holds %d; use %%c or string(r) to see %c\n", r, r)

	// To get another type, ask for it explicitly.
	var big int64 = 5
	small := float32(5)
	fmt.Printf("var big int64 -> %T, float32(5) -> %T\n", big, small)

	// --- Limits of inference ---
	// `:=` needs something to infer FROM, so it cannot be used with only a type:
	//	x := int // ERROR: int (type) is not an expression
	// and nil has no default type of its own:
	//	p := nil // ERROR: use of untyped nil in assignment

	// `:=` is illegal at package level. Only `var`, `const`, `type` and `func` live there.
	//	count := 0 // ERROR (at package level): syntax error: non-declaration statement outside function body

	// --- Semicolon insertion ---
	// The opening brace must stay on the same line. Written like this:
	//	if a == 2
	//	{
	//	}
	// a semicolon is inserted after `a == 2`, and the compiler reports:
	// ERROR: syntax error: unexpected newline, expected { after if clause
	fmt.Println("braces must open on the same line - semicolon insertion depends on it")
}

// =================================================================================================
// Section 4: Zero Values
// =================================================================================================

/*
## Zero Values

- Go has **no uninitialised memory**. Every variable declared without an explicit value is set to
  its type's **zero value**. There is no "undefined behaviour" for reads, and no `mem::uninit`.
- The zero values are:
    - numeric types (all integers, floats, complex): `0`
    - `bool`: `false`
    - `string`: `""` — the *empty* string, never `nil`; a `string` cannot be nil
    - pointer, function, interface, slice, map, channel: `nil`
    - array: an array of the element type's zero values, of the declared length
    - struct: a struct whose every field holds its own zero value, recursively
- The design goal is the **"useful zero value"**: a type should be usable straight after `var x T`,
  with no constructor. `var buf bytes.Buffer`, `var mu sync.Mutex` and `var wg sync.WaitGroup` all
  work immediately. When you design your own types, aim for the same property.
- Two important exceptions where the nil zero value is only *partly* usable:
    - a **nil slice** supports `len`, `cap`, `range` and `append` — it behaves as an empty slice
      almost everywhere — but it is not identical to `[]T{}` (see module 002b).
    - a **nil map** supports reads (`m[k]` yields the zero value, and comma-ok reports `false`) but
      **panics on write**. A map must be created with `make` or a literal before you assign to it.
- Zero values are why Go has no constructors and no `null` in the Java sense: the language
  guarantees a defined starting state, so most types simply do not need an initialiser.

### The cost of the useful zero value

You cannot distinguish "not set" from "set to the zero value" in a plain field. `Count int` holding
`0` might mean either. When that distinction matters the idioms are a pointer (`*int`, nil = unset),
a comma-ok style second field, or a dedicated sentinel — all of which cost something. Do it only
where the ambiguity is real.
*/

type m001aZeroDemo struct {
	Number  int
	Text    string
	Flag    bool
	Pointer *int
	Slice   []string
	Map     map[string]int
	Nested  struct{ Inner float64 }
}

func m001aZeroValues() {
	fmt.Println("\n--- Section 4: Zero Values ---")

	var (
		i  int
		f  float64
		b  bool
		s  string
		p  *int
		l  []int
		m  map[string]int
		a  [3]int
		c  chan int
		fn func()
		e  error // an interface, so nil
	)

	fmt.Printf("int      %d\n", i)
	fmt.Printf("float64  %v\n", f)
	fmt.Printf("bool     %t\n", b)
	fmt.Printf("string   %q (empty, NOT nil - a string can never be nil)\n", s)
	fmt.Printf("*int     %v\n", p)
	fmt.Printf("[]int    %v (nil? %t, len %d)\n", l, l == nil, len(l))
	fmt.Printf("map      %v (nil? %t, len %d)\n", m, m == nil, len(m))
	fmt.Printf("[3]int   %v (an ARRAY is not nil - it is three zeroed ints)\n", a)
	fmt.Printf("chan int %v\n", c)
	fmt.Printf("func()   %v\n", fn == nil)
	fmt.Printf("error    %v\n", e)

	// Structs zero every field, recursively.
	var z m001aZeroDemo
	fmt.Printf("struct   %+v\n", z)

	// The useful zero value in practice: no constructor needed.
	var sb strings.Builder
	sb.WriteString("a zero-value strings.Builder is ready to use")
	fmt.Println(sb.String())

	// A nil slice is usable: len, cap, range and append all work on it.
	var nilSlice []int
	nilSlice = append(nilSlice, 1, 2, 3) // append allocates on first use
	fmt.Println("append to a nil slice:", nilSlice)

	// A nil map is readable...
	var nilMap map[string]int
	value, ok := nilMap["missing"]
	fmt.Printf("read from a nil map: value=%d ok=%t\n", value, ok)
	// ...but NOT writable:
	//	nilMap["key"] = 1 // panic: assignment to entry in nil map
	fmt.Println("writing to a nil map panics: assignment to entry in nil map")
}

// =================================================================================================
// Section 5: Scope, Blocks and the Unused-Variable Rule
// =================================================================================================

/*
## Scope, Blocks and the Unused-Variable Rule

- Go is **lexically scoped using blocks**. A block is a `{ ... }` pair, plus a few implicit ones:
  the *universe* block (all predeclared identifiers), the *package* block, the *file* block (import
  names live here), and one block per `if`, `for`, `switch`, `select` and each `case` clause.
- A declaration is visible from its point of declaration to the end of the innermost enclosing
  block. Unlike package-level declarations, which are visible to the whole package regardless of
  order, a local variable does **not** exist before the line that declares it.
- `if`, `for` and `switch` may carry an **init statement**: `if err := f(); err != nil { ... }`.
  The variable it declares is scoped to the statement — including all its `else if` and `else`
  branches — and is invisible afterwards. This idiom keeps error variables from leaking, and is the
  single most common use of `:=` in real Go.
- **An unused local variable is a compile error**, and so is an unused import. Package-level
  variables and function parameters are exempt. This is deliberate: unused locals are almost always
  a leftover or a bug, and Go chose an error over a warning because warnings get ignored.
- The escape hatch is the **blank identifier** `_`: it can be assigned to but never read, and it may
  be repeated. `_ = x` silences the rule; `for _, v := range s` discards the index; `_, err := f()`
  discards a result.
- The `:=` **redeclaration rule**: in `a, err := f()` it is enough that *at least one* variable on
  the left is new. The others are plain assignments, provided they were declared **in the same
  block**. If they were declared in an outer block, `:=` declares brand-new shadowing variables
  instead — which is the trap in Section 6.
*/

func m001aScopeAndBlocks() {
	fmt.Println("\n--- Section 5: Scope, Blocks and the Unused-Variable Rule ---")

	outer := "declared in the function block"
	{
		inner := "declared in a nested block"
		fmt.Println("inside the block, both are visible:", outer, "/", inner)
	}
	// `inner` no longer exists here:
	//	fmt.Println(inner) // ERROR: undefined: inner

	// A local does not exist before its declaration, even though package-level names do:
	//	fmt.Println(later) // ERROR: undefined: later
	later := "declared after the line above"
	fmt.Println(later)

	// --- The init statement ---
	// `n` and `err` are scoped to the if/else chain and vanish afterwards.
	if n, err := fmt.Sscanf("7", "%d", new(int)); err != nil {
		fmt.Println("scan failed:", err)
	} else {
		fmt.Println("init statement: scanned", n, "item(s); n and err exist only in here")
	}
	//	fmt.Println(n) // ERROR: undefined: n

	// A `switch` and a `for` open scopes the same way.
	switch size := len(outer); {
	case size > 20:
		fmt.Println("switch init statement: length is", size)
	default:
		fmt.Println("switch init statement: short")
	}

	// --- The unused-variable rule ---
	// This alone does not compile:
	//	unused := 1 // ERROR: declared and not used: unused
	// The blank identifier is the escape hatch:
	computed := 1 + 1
	_ = computed // explicitly discard it; the value is evaluated, the variable is not "unused"
	fmt.Println("`_ = computed` silences the unused-variable error")

	// `_` may be reused as often as you like, and can never be read back:
	_, _ = 1, 2
	//	fmt.Println(_) // ERROR: cannot use _ as value

	// --- The := redeclaration rule ---
	// At least one variable on the left must be NEW; the rest are ordinary assignments,
	// as long as they were declared in the SAME block.
	first, shared := 1, 2
	second, shared := 3, 4 // `second` is new, `shared` is merely assigned - no new variable
	fmt.Println("redeclaration:", first, second, shared)

	// If NO variable on the left is new, `:=` is an error:
	//	first, second := 5, 6 // ERROR: no new variables on left side of :=
}

// =================================================================================================
// Section 6: Shadowing
// =================================================================================================

/*
## Shadowing

- **Shadowing** is declaring, in an inner block, a variable with the same name as one in an outer
  block. The inner one hides the outer one for the rest of the inner block. The outer variable is
  untouched and reappears when the block ends.
- Unlike Rust, Go does **not** allow shadowing within the same block — `x := 1; x := 2` is
  `no new variables on left side of :=`. Shadowing in Go always means *a new, nested block*.
- Go's predeclared identifiers live in the **universe block**, so they can be shadowed too:
  `len`, `cap`, `new`, `make`, `copy`, `close`, `delete`, `min`, `max`, `clear`, `panic`, `recover`,
  `append`, `print`, and the type names `int`, `string`, `error`, `any`. They are not keywords. This
  is why a package-level helper named `close` is a bad idea: it would shadow the builtin `close`
  **for the whole package** and break every channel close in it — prefer a name like `cleanup`.
- The dangerous case is the **shadowed `err`**. Inside an `if` or a nested block, `err := ...`
  creates a *new* `err` and the assignment to the outer one never happens — so the outer `err` stays
  nil and the error is silently swallowed. This is the single most common Go bug of its kind.
- `go vet` does **not** report shadowing by default. The check exists but must be asked for:
  `go vet -vettool=$(which shadow)` after `go install golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow@latest`.
- Legitimate uses do exist: a loop variable per iteration, a deliberately narrowed variable inside a
  short block, or a `tx` that means something different inside a transaction closure. Shadow when it
  clarifies, never when it merely saves a name.
*/

func m001aShadowing() {
	fmt.Println("\n--- Section 6: Shadowing ---")

	x := "outer"
	fmt.Println("before the block, x =", x)
	{
		x := "inner" // a NEW variable that hides the outer x
		fmt.Println("inside the block, x =", x)
	}
	fmt.Println("after the block, x =", x, "- the outer x was never modified")

	// Shadowing inside the SAME block is not allowed:
	//	x := "again" // ERROR: no new variables on left side of :=

	// --- The classic shadowed-err bug ---
	var err error
	if true {
		// `:=` here declares a brand-new `err` scoped to the if-block. The outer one stays nil.
		_, err := fmt.Println("(this inner call sets only the INNER err)")
		_ = err
	}
	fmt.Println("outer err after the block:", err, "- the error was silently swallowed")

	// The fix: use `=` so the assignment reaches the outer variable. Note that this needs the
	// other result to already exist, which is exactly why the bug is easy to write.
	var n int
	if true {
		n, err = fmt.Println("(this inner call sets the OUTER err)")
	}
	fmt.Println("outer n and err now:", n, err)

	// --- Predeclared identifiers can be shadowed too ---
	// They live in the universe block, so any declaration hides them.
	{
		len := "this string now shadows the builtin len"
		fmt.Println(len)
		//	fmt.Println(len("abc")) // ERROR: invalid operation: cannot call len (variable of type string): string is not a function
	}
	fmt.Println("builtin len works again here:", len("abc"))

	// A package-level shadow is far worse: it hides the builtin for EVERY file of the package.
	// That is why a resource-releasing helper should be called `cleanup` rather than `close`.
	fmt.Println("shadowing a builtin at package level breaks it for the whole package")
}

// Run001a runs every section of module 001a in order.
func Run001a() {
	m001aComments()
	m001aPackagesAndImports()
	m001aVariablesAndTypeInference()
	m001aZeroValues()
	m001aScopeAndBlocks()
	m001aShadowing()
}

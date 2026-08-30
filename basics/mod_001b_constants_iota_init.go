package basics

import (
	"fmt"
	"math"
	"time"
)

/*
# Module 001b — Constants, iota, Package Variables and init

Section numbering continues from module 001a, which ended at Section 6.
*/

// =================================================================================================
// Section 7: Constants
// =================================================================================================

/*
## Constants

- A constant is declared with `const` and is fixed at **compile time**. Its value must be a
  *constant expression*: literals, other constants, and calls to a small set of builtins
  (`len`, `cap`, `min`, `max`, `real`, `imag`, `complex`, `unsafe.Sizeof`) on constant arguments.
- Only **basic types** can be constant: booleans, runes, integers, floats, complex numbers and
  strings. There are **no constant slices, maps, structs, arrays or pointers** — those need `var`.
  This is a real limitation: a "constant" default configuration slice has to be a `var`, and a `var`
  is mutable by anyone in the package.
- `const` may appear at package level or inside a function. Unlike variables, an **unused constant
  is not an error**.

### Typed and untyped constants — the important part

- A constant declared **without a type** is *untyped* and behaves like an ideal, arbitrary-precision
  number. It only acquires a type when it is used somewhere that needs one.
- Untyped constants are held at **at least 256 bits of precision**, far beyond any runtime type. So
  `const big = 1 << 200` is legal, and intermediate results in constant expressions do not overflow.
  The constant only has to fit when it is finally *assigned* to a typed destination.
- This is what makes `math.Pi` usable as both a `float32` and a `float64` with no conversion, and
  why `time.Second * 60` compiles while `someInt * time.Second` does not.
- A **typed** constant (`const x int64 = 5`) is bound to that type immediately and obeys the normal
  no-implicit-conversion rule. Prefer untyped constants unless you specifically want the constraint.
- Each untyped constant has a **default type** used when the context supplies none (`:=`,
  interface assignment, variadic `any`): `int`, `rune`, `float64`, `complex128`, `string`, `bool`.

### Grouping and repetition

Inside a `const ( ... )` block, an entry with **no expression repeats the previous expression** —
including its use of `iota`. That rule is what makes the `iota` idiom in Section 8 work.
*/

const (
	// Untyped: usable wherever an integer-ish value is needed, without conversion.
	m001bCurrentYear = 2025

	// Typed: bound to float64 forever.
	m001bTaxRate float64 = 0.23

	// Constant expressions are evaluated at compile time.
	m001bSecondsPerDay = 60 * 60 * 24

	// Arbitrary precision: this value does not fit in any runtime integer type, and that is fine
	// as long as we never assign it to one.
	m001bHuge = 1 << 200

	// `len` of a constant string is itself constant.
	m001bGreetingLen = len("hello")
)

// m001bConstantsSummary is called from module 001a Section 2 to show that package-level identifiers
// are visible across all files of a package.
func m001bConstantsSummary() string {
	return fmt.Sprintf("module 001b defines 5 constants in its first block, e.g. CurrentYear=%d", m001bCurrentYear)
}

func m001bConstants() {
	fmt.Println("--- Section 7: Constants ---")

	fmt.Printf("CurrentYear=%d TaxRate=%.2f SecondsPerDay=%d GreetingLen=%d\n",
		m001bCurrentYear, m001bTaxRate, m001bSecondsPerDay, m001bGreetingLen)

	// --- Untyped constants adapt to their context ---
	// The SAME constant is used as three different types, with no conversion anywhere.
	var asInt int = m001bCurrentYear
	var asInt64 int64 = m001bCurrentYear
	var asFloat float64 = m001bCurrentYear
	fmt.Printf("one untyped constant, three types: %T %T %T\n", asInt, asInt64, asFloat)

	// math.Pi is untyped, so it fits both float widths without a cast.
	var pi32 float32 = math.Pi
	var pi64 float64 = math.Pi
	fmt.Printf("math.Pi as float32=%.7f as float64=%.15f\n", pi32, pi64)

	// A TYPED constant does not adapt - it obeys the ordinary conversion rules:
	//	var n int = m001bTaxRate // ERROR: cannot use m001bTaxRate (constant 0.23 of type float64) as int value in variable declaration

	// --- Arbitrary precision ---
	// The huge constant only has to fit when it lands in a typed variable. Shifting it back down
	// keeps full precision through the intermediate step.
	const backDown = m001bHuge >> 190
	fmt.Println("1<<200 >> 190 =", backDown)
	//	var overflow int64 = m001bHuge // ERROR: cannot use m001bHuge (untyped int constant 1606938044258990275541962092341162602522202993782792835301376) as int64 value in variable declaration (overflows)

	// This is also why durations compose so naturally: `60` is untyped and takes on time.Duration.
	timeout := 60 * time.Second
	fmt.Println("60 * time.Second =", timeout)
	// But an int VARIABLE has a type already, and there is no implicit conversion:
	//	n := 60
	//	bad := n * time.Second // ERROR: invalid operation: n * time.Second (mismatched types int and time.Duration)

	// --- What cannot be constant ---
	//	const list = []int{1, 2, 3}     // ERROR: []int{…} (value of type []int) is not constant
	//	const table = map[string]int{}  // ERROR: map[string]int{} (value of type map[string]int) is not constant
	//	const now = time.Now()          // ERROR: time.Now() (value of type time.Time) is not constant
	fmt.Println("slices, maps, structs and function results cannot be constants - use var")

	// --- Repetition inside a const block ---
	const (
		a = 10 // 10
		b      // repeats the previous expression: also 10
		c      // also 10
	)
	fmt.Println("omitted expressions repeat the previous one:", a, b, c)

	// An unused constant is fine - unlike an unused variable.
	const unusedIsFine = "no compile error here"
}

// =================================================================================================
// Section 8: iota and the Enum Idiom
// =================================================================================================

/*
## iota and the Enum Idiom

- Go has **no enum type**. The idiom is a *defined type* plus a `const` block, and `iota` supplies
  the values.
- `iota` is a predeclared identifier that equals the **index of the current ConstSpec line** inside
  a `const` block, counting from 0. It resets to 0 in every new `const` block.
- Crucially, `iota` counts **lines, not identifiers**. A line that is skipped with `_` still
  advances it, and a line declaring `a, b = iota, iota` gives both the *same* value.
- Combined with the "omitted expression repeats the previous one" rule from Section 7, this gives
  the familiar shorthand where only the first line carries an expression.
- Because the whole expression repeats, `iota` composes with arithmetic and shifts: `1 << (10 *
  iota)` produces KB, MB, GB; `iota + 1` starts at 1.
- Use a **defined type** (`type Weekday int`), not an alias (`type Weekday = int`). Only a defined
  type can carry methods, which is what lets you attach `String()` and get readable output from
  `fmt`. An alias would print as a bare number and would accept any `int`.
- Implement `fmt.Stringer` (`String() string`) so `%v`, `%s` and `Println` show the name. The
  `stringer` tool (`go generate` + `//go:generate stringer -type=Weekday`) writes it for you.

### When iota is the wrong tool

`iota` is only appropriate when the values are **consecutive and their numeric value is arbitrary**.
If the numbers are meaningful and non-consecutive — HTTP status codes 200 / 204 / 404, POSIX errnos,
wire-protocol tags — write them out literally. An earlier version of this repo's `main.go` used
`100 + iota` for HTTP statuses and produced 100 / 101 / 102, which are simply the wrong codes.

Also beware: `iota` bakes ordinal values into your API. Inserting a constant in the middle silently
renumbers everything after it, which breaks any persisted or transmitted value.
*/

// m001bWeekday is a DEFINED type (no `=`), so it can carry methods.
type m001bWeekday int

const (
	m001bSunday  m001bWeekday = iota // 0
	m001bMonday                      // 1 - expression repeats, iota advances
	m001bTuesday                     // 2
	m001bWednesday
	m001bThursday
	m001bFriday
	m001bSaturday // 6
)

// String makes m001bWeekday satisfy fmt.Stringer, so %v and Println print the name.
func (d m001bWeekday) String() string {
	names := [...]string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	if d < 0 || int(d) >= len(names) {
		return fmt.Sprintf("m001bWeekday(%d)", int(d))
	}
	return names[d]
}

// Byte sizes: the whole expression repeats, so the shift grows with iota.
// Note these constants are deliberately left UNTYPED, so they can be used wherever
// a numeric type is expected. Giving them a defined type would force a conversion
// at every use.
const (
	_       = iota             // skip 0; the line still advances iota
	m001bKB = 1 << (10 * iota) // 1 << 10
	m001bMB                    // 1 << 20
	m001bGB                    // 1 << 30
	m001bTB                    // 1 << 40
)

// Bit flags: each constant is a distinct bit, combinable with |.
type m001bPermission uint8

const (
	m001bRead    m001bPermission = 1 << iota // 1
	m001bWrite                               // 2
	m001bExecute                             // 4
)

// Non-consecutive, meaningful values: written out literally, NOT with iota.
type m001bResponseStatus int

const (
	m001bStatusOK        m001bResponseStatus = 200
	m001bStatusNoContent m001bResponseStatus = 204
	m001bStatusNotFound  m001bResponseStatus = 404
)

func (s m001bResponseStatus) String() string {
	switch s {
	case m001bStatusOK:
		return "OK"
	case m001bStatusNoContent:
		return "No Content"
	case m001bStatusNotFound:
		return "Not Found"
	default:
		return fmt.Sprintf("Unknown(%d)", int(s))
	}
}

func m001bIotaAndEnums() {
	fmt.Println("\n--- Section 8: iota and the Enum Idiom ---")

	// Because m001bWeekday has a String method, %v prints the name, not the number.
	fmt.Printf("Sunday=%v(%d) Wednesday=%v(%d) Saturday=%v(%d)\n",
		m001bSunday, m001bSunday, m001bWednesday, m001bWednesday, m001bSaturday, m001bSaturday)

	// Out-of-range values are still valid m001bWeekday values - Go enums are not closed sets.
	fmt.Println("an enum is just an int, so this is legal:", m001bWeekday(42))

	// Byte sizes via a shifting iota.
	// Untyped constants take their default type (int) when passed through an
	// interface, so %d is the matching verb - no conversion needed.
	fmt.Printf("KB=%d MB=%d GB=%d TB=%d\n",
		m001bKB, m001bMB, m001bGB, m001bTB)

	// --- Bit flags ---
	perms := m001bRead | m001bWrite
	fmt.Printf("perms=%03b readable=%t writable=%t executable=%t\n",
		perms, perms&m001bRead != 0, perms&m001bWrite != 0, perms&m001bExecute != 0)
	perms &^= m001bWrite // AND NOT - clear the write bit (see module 003)
	fmt.Printf("after clearing write: %03b writable=%t\n", perms, perms&m001bWrite != 0)

	// --- iota counts LINES, not identifiers ---
	const (
		zero         = iota            // 0
		one                            // 1
		_                              // 2 - skipped, but the line still advances iota
		three                          // 3
		fourA, fourB = iota, iota * 10 // both on line 4: 4 and 40
		fiveA, fiveB                   // the whole expression repeats: 5 and 50
	)
	fmt.Println("iota counts lines:", zero, one, three, fourA, fourB, fiveA, fiveB)

	// --- The wrong tool ---
	// These values are meaningful, so they are written out rather than derived from iota.
	fmt.Printf("status codes: %d=%v %d=%v %d=%v\n",
		m001bStatusOK, m001bStatusOK,
		m001bStatusNoContent, m001bStatusNoContent,
		m001bStatusNotFound, m001bStatusNotFound)
	fmt.Println("`100 + iota` would have produced 100/101/102 - not real HTTP status codes")
}

// =================================================================================================
// Section 9: Package-Level Variables and Initialisation Order
// =================================================================================================

/*
## Package-Level Variables and Initialisation Order

- A package-level `var` may be declared **in any order**: unlike locals, it is visible to the whole
  package regardless of where it appears, and it may reference declarations that come later in the
  file, or in another file entirely.
- Initialisation is **dependency-ordered, not textual**. The compiler builds a dependency graph of
  package-level variables and initialises each one only after everything it references. Within a
  single dependency level the order is the order of declaration, with files taken in the order the
  build system presents them (`go build` sorts them by filename).
- A **dependency cycle** among package-level variables is a compile error:
  `initialization cycle: a refers to b, b refers to a`.
- Unlike constants, a package-level variable **may** be initialised by a function call, so this is
  where expensive or non-constant setup goes.
- The full startup order for a program is:
    1. all imported packages are initialised first, recursively, each exactly once
    2. within a package: package-level variables, in dependency order
    3. within a package: every `init()` function, in declaration order across files
    4. finally `main.main()`
- Package-level variables are effectively **global mutable state**. They are exempt from the
  unused-variable rule, they are visible to every file of the package, and if the package is
  imported by concurrent code they are shared across goroutines with no synchronisation. Prefer
  passing values explicitly; keep package-level `var` for genuinely process-wide things such as
  sentinel errors (module 009).
*/

// Declared before the things they depend on - order does not matter at package level.
var (
	m001bDerived  = m001bBase * 2      // depends on m001bBase, so it is initialised second
	m001bBase     = m001bComputeBase() // depends on a function call
	m001bStarted  = time.Now()         // non-constant initialiser: only var can do this
	m001bTraceLog []string             // zero value: a nil slice
)

func m001bComputeBase() int {
	m001bTraceLog = append(m001bTraceLog, "m001bComputeBase() ran")
	return 21
}

func m001bPackageVariables() {
	fmt.Println("\n--- Section 9: Package-Level Variables and Initialisation Order ---")

	// m001bDerived is declared BEFORE m001bBase, yet it holds the right value: initialisation
	// follows the dependency graph, not the text.
	fmt.Printf("m001bBase=%d m001bDerived=%d (declared first, initialised second)\n",
		m001bBase, m001bDerived)

	fmt.Println("m001bStarted was set by a function call at init time:",
		!m001bStarted.IsZero())

	// A cycle is rejected at compile time:
	//	var x = y + 1
	//	var y = x + 1 // ERROR: initialization cycle for x: x refers to y, y refers to x

	fmt.Println("initialisation trace so far:", m001bTraceLog)
}

// =================================================================================================
// Section 10: init Functions
// =================================================================================================

/*
## init Functions

- `func init()` is a special function run automatically at package initialisation, after all
  package-level variables have been initialised and before `main`.
- It takes **no parameters and returns nothing**. It **cannot be called** from ordinary code —
  `undefined: init` — and it cannot be referenced as a value.
- A package may declare **many** `init` functions, in one file or spread across files. They are not
  in the package namespace at all, so unlike every other identifier they may share a name. Within a
  file they run top to bottom; across files, in the order the compiler receives the files (for
  `go build`, sorted by filename).
- Each imported package is initialised **exactly once**, however many packages import it.
- The main legitimate uses are: registering with a registry (`database/sql` drivers,
  `image` decoders, `encoding/gob` types), validating package-level state that cannot be expressed
  as a constant, and precomputing lookup tables.
- The **blank import** exists precisely for this: `import _ "github.com/lib/pq"` runs the driver's
  `init`, which calls `sql.Register("postgres", ...)`, and nothing else.
- Use `init` sparingly. It runs implicitly, its ordering is subtle, it cannot return an error (so
  its only failure mode is `panic`, which kills the program before `main` starts), and it makes
  tests harder because it always runs. Explicit setup called from `main` is usually better.
*/

var m001bLookupTable map[string]int

func init() {
	// Precomputing a table is a legitimate use: it cannot be a constant, and it must exist before
	// anything in the package uses it.
	m001bLookupTable = map[string]int{"one": 1, "two": 2, "three": 3}
	m001bTraceLog = append(m001bTraceLog, "first init() ran")
}

// A second init in the same file - perfectly legal, and it runs after the first.
func init() {
	m001bTraceLog = append(m001bTraceLog, "second init() ran")
}

func m001bInitFunctions() {
	fmt.Println("\n--- Section 10: init Functions ---")

	fmt.Println("the lookup table was built by init():", m001bLookupTable)
	fmt.Println("full initialisation trace:", m001bTraceLog)
	fmt.Println("order was: package vars (dependency order) -> init()s (declaration order) -> main")

	// init cannot be called explicitly:
	//	init() // ERROR: undefined: init

	fmt.Println("`import _ \"github.com/lib/pq\"` in examples/rest.go exists only to run its init()")
}

// Run001b runs every section of module 001b in order.
func Run001b() {
	m001bConstants()
	m001bIotaAndEnums()
	m001bPackageVariables()
	m001bInitFunctions()
}

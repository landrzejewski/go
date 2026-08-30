package basics

import (
	"fmt"
	"math"
	"math/bits"
	"slices"
)

/*
# Module 003 — Operators

Go's operator set is deliberately small. There is **no operator overloading**, so `+` always means
what the language says it means for the given type, and reading an expression never requires
knowing what someone defined elsewhere. There is also no ternary `?:`, no `**` for exponentiation,
no `<=>`, and no null-coalescing operator.
*/

// =================================================================================================
// Section 1: Arithmetic Operators
// =================================================================================================

/*
## Arithmetic Operators

- Five binary arithmetic operators: `+` `-` `*` `/` `%`, plus unary `+` and `-`.
- `+` is also **string concatenation**. It is the only operator that is overloaded at all, and it is
  built into the language rather than definable.
- `%` (remainder) applies to **integers only**. `5.5 % 2.0` does not compile — use `math.Mod` for
  floats. Contrast Rust, where `%` works on floats too.
- `^` is **bitwise XOR**, not exponentiation. Go has no exponentiation operator at all; use
  `math.Pow` (which returns a `float64`), or a loop for exact integer powers.
- **Both operands must have the same type.** There is no promotion between `int` and `int64`, or
  between `int` and `float64`. Untyped constants are the exception: they adapt to the other operand.
- Integer division **truncates toward zero**; `%` keeps the **left** operand's sign. Integer
  division or remainder by zero **panics**; the float equivalents give `±Inf` / `NaN` (module 002a).
- **Overflow wraps silently** for integers. There is no checked arithmetic in the language; the
  `math/bits` package provides `Add64`, `Mul64` and friends with explicit carry/overflow outputs.
- `++` and `--` are **statements, not operators** — and postfix only. You cannot write `x = y++`,
  `++x`, or use them inside an expression. This removes a whole class of sequence-point questions.
*/

func m003ArithmeticOperators() {
	fmt.Println("--- Section 1: Arithmetic Operators ---")

	a, b := 20, 7
	fmt.Printf("%d + %d = %d\n", a, b, a+b)
	fmt.Printf("%d - %d = %d\n", a, b, a-b)
	fmt.Printf("%d * %d = %d\n", a, b, a*b)
	fmt.Printf("%d / %d = %d  (integer division truncates)\n", a, b, a/b)
	fmt.Printf("%d %% %d = %d  (remainder)\n", a, b, a%b)
	fmt.Printf("unary: -%d = %d\n", a, -a)

	// `+` also concatenates strings - the one built-in overload.
	fmt.Println("\"Go\" + \"lang\" =", "Go"+"lang")

	// --- Truncation and remainder signs ---
	fmt.Printf("-7 / 2 = %d (toward zero, not -4)\n", -7/2)
	fmt.Printf("-7 %% 3 = %d, 7 %% -3 = %d (sign follows the LEFT operand)\n", -7%3, 7%-3)

	// --- % is integers only ---
	//	fmt.Println(5.5 % 2.0) // ERROR: invalid operation: operator % not defined on 5.5 (untyped float constant)
	fmt.Printf("math.Mod(5.5, 2.0) = %v  (the float equivalent)\n", math.Mod(5.5, 2.0))

	// --- ^ is XOR, not exponentiation ---
	fmt.Printf("2 ^ 10 = %d  <- this is XOR, not 1024!\n", 2^10)
	fmt.Printf("math.Pow(2, 10) = %v  (returns float64)\n", math.Pow(2, 10))
	fmt.Printf("exact integer power by loop: 2^10 = %d\n", m003IntPow(2, 10))

	// --- Mixed types do not compile ---
	var i int = 5
	var f float64 = 2.0
	fmt.Printf("float64(i) * f = %v (explicit conversion required)\n", float64(i)*f)
	//	fmt.Println(i * f) // ERROR: invalid operation: i * f (mismatched types int and float64)
	// But an untyped CONSTANT adapts to the other operand:
	fmt.Printf("i * 2 = %d and f * 2 = %v - the untyped 2 becomes int, then float64\n", i*2, f*2)

	// --- Overflow wraps; math/bits detects it ---
	var m8 uint8 = 255
	fmt.Printf("uint8 255 + 1 = %d (wrapped)\n", m8+1)
	sum, carry := bits.Add64(math.MaxUint64, 1, 0)
	fmt.Printf("bits.Add64(MaxUint64, 1, 0) = sum %d, carry %d (explicit overflow detection)\n", sum, carry)
	hi, lo := bits.Mul64(math.MaxUint64, 2)
	fmt.Printf("bits.Mul64 gives a 128-bit result: hi=%d lo=%d\n", hi, lo)

	// --- ++ and -- are statements ---
	counter := 0
	counter++ // a complete statement, on its own line
	counter--
	counter++
	fmt.Println("counter after ++/--/++ =", counter)
	//	x := counter++  // ERROR: syntax error: unexpected ++ at end of statement
	//	++counter       // ERROR: syntax error: unexpected ++, expected }
	fmt.Println("`x = y++` and `++x` do not exist in Go - ++ is a statement, postfix only")
}

// m003IntPow computes base**exp exactly, in integers.
func m003IntPow(base, exp int) int {
	result := 1
	for range exp {
		result *= base
	}
	return result
}

// =================================================================================================
// Section 2: Comparison Operators
// =================================================================================================

/*
## Comparison Operators

- Six operators: `==` `!=` `<` `<=` `>` `>=`. The result is always an **untyped boolean**.
- **Equality** `==` / `!=` works on any *comparable* type. **Ordering** `<` `<=` `>` `>=` works only
  on *ordered* types: integers, floats and strings. You cannot order structs, arrays, pointers or
  booleans, even though you can compare them for equality.
- Comparability, precisely:
    - comparable: booleans, numbers, strings, pointers, channels, interfaces, and structs and arrays
      whose elements are all comparable
    - **not** comparable: slices, maps, functions — they may only be compared against `nil`
    - an **interface** is comparable at compile time but can **panic at runtime** if the dynamic
      value turns out to be uncomparable
- Strings compare **byte-wise, lexicographically** — not by locale, and not by Unicode collation.
- Floats are only **partially** ordered because `NaN` compares `false` against everything, itself
  included. `cmp.Compare` defines a total order (NaN sorts lowest) so sorting works.
- Comparisons **cannot be chained**: `a < b < c` is a type error, because `a < b` yields a `bool`
  and `bool < c` is not defined.
- Comparing values of different types is a compile error, even for two integer types. Convert first.
- For deep comparison use `slices.Equal` / `maps.Equal` (fast, typed) or `reflect.DeepEqual` (slow,
  reflective, and subtly different — it treats a nil and an empty slice as unequal).
*/

func m003ComparisonOperators() {
	fmt.Println("\n--- Section 2: Comparison Operators ---")

	a, b := 10, 20
	fmt.Printf("%d==%d:%t  !=:%t  <:%t  <=:%t  >:%t  >=:%t\n",
		a, b, a == b, a != b, a < b, a <= b, a > b, a >= b)

	// Strings order byte-wise.
	fmt.Printf("\"apple\" < \"banana\" = %t\n", "apple" < "banana")
	fmt.Printf("\"Z\" < \"a\" = %t (ASCII: uppercase comes first)\n", "Z" < "a")
	fmt.Printf("\"ą\" > \"z\" = %t (byte-wise, not alphabetical in Polish)\n", "ą" > "z")

	// Booleans and pointers: equality only, no ordering.
	x, y := true, false
	fmt.Printf("true != false = %t\n", x != y)
	//	fmt.Println(x < y) // ERROR: invalid operation: x < y (operator < not defined on bool)

	p, q := &a, &b
	fmt.Printf("pointer equality compares ADDRESSES: p == q is %t, p == &a is %t\n", p == q, p == &a)
	//	fmt.Println(p < q) // ERROR: invalid operation: p < q (operator < not defined on pointer)

	// --- No chaining ---
	//	fmt.Println(1 < 2 < 3) // ERROR: invalid operation: 1 < 2 < 3 (mismatched types untyped bool and untyped int)
	fmt.Printf("write `a < b && b < c` instead: %t\n", a < b && b < 30)

	// --- No cross-type comparison ---
	var i32 int32 = 10
	fmt.Printf("int(i32) == a is %t (conversion required)\n", int(i32) == a)
	//	fmt.Println(i32 == a) // ERROR: invalid operation: i32 == a (mismatched types int32 and int)

	// --- NaN ---
	nan := math.NaN()
	fmt.Printf("NaN == NaN: %t, NaN < 1: %t, NaN > 1: %t - all false\n", nan == nan, nan < 1, nan > 1)

	// --- Uncomparable types ---
	s1, s2 := []int{1, 2}, []int{1, 2}
	//	fmt.Println(s1 == s2) // ERROR: invalid operation: s1 == s2 (slice can only be compared to nil)
	fmt.Printf("slices compare only to nil; use slices.Equal: %t\n", slices.Equal(s1, s2))

	// An interface holding an uncomparable value panics at RUNTIME, not compile time.
	var i1, i2 any = []int{1}, []int{1}
	fmt.Println("comparing two interfaces holding slices panics at runtime:",
		m003SafeCompare(i1, i2))
}

// m003SafeCompare shows that interface comparison can panic, and recovers from it.
func m003SafeCompare(a, b any) (result string) {
	defer func() {
		if r := recover(); r != nil {
			result = fmt.Sprintf("recovered from %v", r)
		}
	}()
	return fmt.Sprintf("%t", a == b)
}

// =================================================================================================
// Section 3: Logical Operators
// =================================================================================================

/*
## Logical Operators

- Three operators: `&&` (and), `||` (or), `!` (not). They apply to `bool` **only** — there is no
  truthiness and no conversion from other types (module 002a, Section 4).
- `&&` and `||` **short-circuit**: the right operand is evaluated only when the left does not decide
  the result. This is a guarantee of the language, not an optimisation, so it is safe to rely on for
  nil guards and bounds checks: `if p != nil && p.Valid()`, `if i < len(s) && s[i] == 'x'`.
- Precedence: `!` binds tightest, then `&&`, then `||`. All three bind **looser** than every
  comparison, so `a == b && c != d` parses the way you would hope, with no parentheses needed.
- Go has **no logical XOR** operator for booleans — use `!=`, which does exactly that for `bool`.
- There is **no ternary operator**. The official position is that `if/else` is clearer; the cost is
  that a simple conditional assignment takes four lines, or a helper. Since Go 1.21 `min` and `max`
  cover the most common case that people reached for `?:` to express.
*/

func m003LogicalOperators() {
	fmt.Println("\n--- Section 3: Logical Operators ---")

	t, f := true, false
	fmt.Printf("t&&f=%t  t||f=%t  !t=%t\n", t && f, t || f, !t)

	// --- Short-circuiting is guaranteed ---
	evaluated := 0
	track := func(v bool) bool { evaluated++; return v }

	evaluated = 0
	_ = f && track(true)
	fmt.Printf("false && f(): right side evaluated %d times\n", evaluated)

	evaluated = 0
	_ = t || track(true)
	fmt.Printf("true  || f(): right side evaluated %d times\n", evaluated)

	evaluated = 0
	_ = t && track(true)
	fmt.Printf("true  && f(): right side evaluated %d times\n", evaluated)

	// The practical payoff: guards that would otherwise panic.
	var s []int
	if len(s) > 0 && s[0] == 1 {
		fmt.Println("unreachable")
	}
	fmt.Println("`len(s) > 0 && s[0] == 1` never indexes an empty slice - guaranteed by the language")

	// --- Precedence: comparisons bind tighter than && and || ---
	a, b, c, d := 1, 1, 2, 3
	fmt.Printf("a==b && c<d || false  =>  %t (parses as ((a==b) && (c<d)) || false)\n",
		a == b && c < d || false)

	// --- No boolean XOR operator; != is the XOR ---
	fmt.Printf("XOR via !=: true!=false=%t  true!=true=%t\n", t != f, t != t)

	// --- No ternary ---
	// The Go spelling of `max := a > b ? a : b`:
	value := 0
	if a > c {
		value = a
	} else {
		value = c
	}
	fmt.Printf("no ternary in Go; if/else gives %d, and since 1.21 max(a, c) gives %d\n", value, max(a, c))
}

// =================================================================================================
// Section 4: Assignment and Compound Assignment
// =================================================================================================

/*
## Assignment and Compound Assignment

- `=` assigns; `:=` declares and assigns (module 001a, Section 3).
- **Assignment is a statement, not an expression.** `a = b = c` and `if x = f()` do not compile.
  This single rule removes the classic `if (x = 0)` typo entirely.
- **Tuple assignment** assigns several values at once: `a, b = b, a`. The right-hand side is
  evaluated **completely** before anything on the left is assigned — including index expressions —
  so a swap needs no temporary, and `i, s[i] = 0, 1` uses the *old* `i` to index.
- Compound assignment exists for every binary arithmetic and bitwise operator:
  `+= -= *= /= %= &= |= ^= <<= >>= &^=`. `x op= y` is `x = x op y` except that `x` is evaluated
  **once** — which matters when `x` is `m[expensive()]` or `s[i()]`.
- The **blank identifier** may appear on the left of any assignment to discard a value.
- **Addressability**: the left-hand side must be a variable, a pointer dereference, a slice element,
  or a field of an addressable struct. A **map entry** and a **string index** are not addressable,
  so `m[k].Field = v` and `s[0] = 'x'` are compile errors.
*/

func m003AssignmentOperators() {
	fmt.Println("\n--- Section 4: Assignment and Compound Assignment ---")

	x := 10
	fmt.Printf("start x=%d\n", x)
	x += 5
	fmt.Printf("x += 5  -> %d\n", x)
	x -= 3
	fmt.Printf("x -= 3  -> %d\n", x)
	x *= 2
	fmt.Printf("x *= 2  -> %d\n", x)
	x /= 4
	fmt.Printf("x /= 4  -> %d\n", x)
	x %= 4
	fmt.Printf("x %%= 4  -> %d\n", x)

	// Bitwise compound forms.
	bitsVal := 0b1100
	bitsVal &= 0b1010
	fmt.Printf("0b1100 &= 0b1010 -> %04b\n", bitsVal)
	bitsVal |= 0b0001
	fmt.Printf("       |= 0b0001 -> %04b\n", bitsVal)
	bitsVal ^= 0b1111
	fmt.Printf("       ^= 0b1111 -> %04b\n", bitsVal)
	bitsVal <<= 2
	fmt.Printf("       <<= 2     -> %06b\n", bitsVal)
	bitsVal &^= 0b100000
	fmt.Printf("       &^= 0b100000 (AND NOT) -> %06b\n", bitsVal)

	// --- Assignment is a statement ---
	//	a := (b = 5) // ERROR: syntax error: unexpected =, expected )
	//	if y = 5; ... // assignment in a condition simply cannot be written as a comparison typo
	fmt.Println("assignment is a statement, so `if x = y` cannot be a typo for `if x == y`")

	// --- Tuple assignment: the whole RHS is evaluated first ---
	a, b := 1, 2
	a, b = b, a
	fmt.Printf("swap without a temporary: a=%d b=%d\n", a, b)

	s := []int{0, 10, 20}
	i := 1
	i, s[i] = 0, 99 // s[i] uses the OLD i (1), not the newly assigned 0
	fmt.Printf("i, s[i] = 0, 99  ->  i=%d s=%v (the index used the old i)\n", i, s)

	// --- x op= y evaluates x once ---
	calls := 0
	index := func() int { calls++; return 0 }
	arr := []int{5}
	arr[index()] += 10
	fmt.Printf("arr[index()] += 10 called index() %d time(s); arr=%v\n", calls, arr)

	// --- Blank identifier on the left ---
	_, second := 1, 2
	fmt.Println("discarding a value with _:", second)

	// --- Addressability ---
	type box struct{ N int }
	m := map[string]box{"a": {N: 1}}
	//	m["a"].N = 2 // ERROR: cannot assign to struct field m["a"].N in map
	str := "hello"
	//	str[0] = 'H' // ERROR: cannot assign to str[0] (neither addressable nor a map index expression)
	_ = m
	_ = str
	fmt.Println("map entries and string indices are not addressable - see modules 002b and 002a")
}

// =================================================================================================
// Section 5: Bitwise and Shift Operators
// =================================================================================================

/*
## Bitwise and Shift Operators

- `&` AND, `|` OR, `^` XOR, `&^` **AND NOT**, `<<` left shift, `>>` right shift.
- **`&^` is Go-specific**: `a &^ b` clears in `a` every bit that is set in `b`. It is exactly
  `a & ^b`, but as one operator, and it is the idiomatic way to clear flags. Most languages make you
  write `a & ~b`.
- **Unary `^x`** is bitwise NOT (complement). Go reuses `^` rather than introducing `~`. On an
  unsigned type `^0` is "all ones"; on a signed type `^x == -x - 1`.
- Shifts:
    - The **right operand must be non-negative**. A negative shift count **panics** at runtime
      (`runtime error: negative shift amount`); a negative constant is a compile error.
    - A shift count **larger than the width is not undefined behaviour** as in C — it simply yields
      0 (or -1 for a right shift of a negative number). This is defined and portable.
    - `>>` on a **signed** type is an *arithmetic* shift: the sign bit is replicated, so `-8 >> 1`
      is `-4`. On an **unsigned** type it is a *logical* shift, filling with zeros.
    - Since Go 1.13 the shift count may be a **signed** integer; before that it had to be unsigned.
- Shifts of untyped constants happen at arbitrary precision, which is what makes `1 << 62` and the
  `1 << (10 * iota)` idiom from module 001b work.
- `math/bits` provides the operations the operators do not: `OnesCount` (population count),
  `LeadingZeros`, `TrailingZeros`, `RotateLeft`, `Reverse`, `Len`.
*/

func m003BitwiseOperators() {
	fmt.Println("\n--- Section 5: Bitwise and Shift Operators ---")

	var a, b uint8 = 0b1100, 0b1010
	fmt.Printf("a    = %04b\n", a)
	fmt.Printf("b    = %04b\n", b)
	fmt.Printf("a&b  = %04b  (AND)\n", a&b)
	fmt.Printf("a|b  = %04b  (OR)\n", a|b)
	fmt.Printf("a^b  = %04b  (XOR)\n", a^b)
	fmt.Printf("a&^b = %04b  (AND NOT - clears a's bits that are set in b)\n", a&^b)
	fmt.Printf("^a   = %08b  (unary ^ is NOT; Go has no ~)\n", ^a)

	// &^ is exactly a & ^b, but as one operator.
	fmt.Printf("a&^b == a&(^b): %t\n", a&^b == a&(^b))

	// The flag-clearing idiom.
	const (
		flagRead  = 1 << 0
		flagWrite = 1 << 1
		flagExec  = 1 << 2
	)
	perms := flagRead | flagWrite | flagExec
	fmt.Printf("perms=%03b, after &^= flagWrite -> %03b\n", perms, perms&^flagWrite)

	// --- Shifts ---
	var v uint8 = 0b0000_0011
	fmt.Printf("%08b << 2 = %08b\n", v, v<<2)
	fmt.Printf("%08b >> 1 = %08b\n", v, v>>1)

	// Overshifting is DEFINED in Go: it yields zero, not undefined behaviour.
	// (The count is a variable here only so `go vet` does not flag the constant form as a bug —
	// its "too small for shift" check assumes an overshift is a mistake, which it usually is.)
	overshift := uint(100)
	fmt.Printf("uint8(3) << 100 = %d (defined as 0, not UB as in C)\n", v<<overshift)

	// Signed >> is arithmetic (sign-extending); unsigned >> is logical.
	var signed int8 = -8
	var unsigned uint8 = 0b1111_1000
	fmt.Printf("int8(-8) >> 1  = %d   (arithmetic: sign bit replicated)\n", signed>>1)
	fmt.Printf("uint8 %08b >> 1 = %08b (logical: filled with zeros)\n", unsigned, unsigned>>1)
	fmt.Printf("int8(-1) >> 100 = %d (defined: stays -1)\n", int8(-1)>>overshift)

	// A negative shift count panics at runtime.
	//	shift := -1; fmt.Println(1 << shift) // panic: runtime error: negative shift amount
	//	fmt.Println(1 << -1)                 // ERROR: invalid operation: negative shift count -1 (untyped int constant)
	fmt.Println("a negative shift count is a compile error as a constant, a panic as a variable")

	// Constant shifts use arbitrary precision.
	const huge = 1 << 62
	fmt.Printf("const 1 << 62 = %d\n", huge)

	// --- math/bits ---
	var n uint = 0b1011_0110
	fmt.Printf("n=%08b OnesCount=%d LeadingZeros8=%d TrailingZeros=%d Len=%d\n",
		n, bits.OnesCount(n), bits.LeadingZeros8(uint8(n)), bits.TrailingZeros(n), bits.Len(n))
	fmt.Printf("RotateLeft8(%08b, 2) = %08b   Reverse8 = %08b\n",
		uint8(n), bits.RotateLeft8(uint8(n), 2), bits.Reverse8(uint8(n)))
}

// =================================================================================================
// Section 6: Operator Precedence and Associativity
// =================================================================================================

/*
## Operator Precedence and Associativity

Go has only **five** precedence levels for binary operators — Rust has twelve, C has fifteen. The
whole table fits in a few lines, and every binary operator is **left-associative**.

	5 (highest)  *   /   %   <<   >>   &   &^
	4            +   -   |    ^
	3            ==  !=  <   <=   >    >=
	2            &&
	1 (lowest)   ||

Unary operators — `+` `-` `!` `^` `*` (dereference) `&` (address-of) `<-` (receive) — bind tighter
than every binary operator.

- The consequence that surprises people: **`&` and `<<` bind tighter than `+`**, and `|` and `^`
  bind at the *same* level as `+`. In C, `&` binds *looser* than `==`, which is why C programmers
  learn to parenthesise `(a & b) == c`. In Go that expression already means what it looks like.
- `a + b | c` parses as `(a + b) | c` in Go, because `+` and `|` share level 4 and association is
  left to right. C happens to agree here, because C also binds `+` tighter than `|`.
- The genuine flip side is the *shift* and *XOR* levels, where Go and C really do disagree:
  `<<`/`>>` are level 5 in Go (tighter than `+`) but looser than `+` in C, and `^` is level 4 in Go
  but much looser in C. So `1 + 2<<3` is **17** in Go and **24** in C, and `3 ^ 5 + 1` is **7** in
  Go and **5** in C.
- Comparisons bind looser than all arithmetic and bitwise operators, and tighter than `&&` / `||`,
  so `a+1 == b && c > d` needs no parentheses.
- `*` is context-dependent: binary multiplication at level 5, unary pointer dereference otherwise.
  Likewise `&` is binary AND or unary address-of, and `<-` is send or receive.
- **Guidance:** knowing the table is worth it for *reading* code. When *writing* it, parenthesise
  anything a reader would have to look up. `gofmt` helps by spacing operators according to
  precedence — `a*b + c` rather than `a * b + c` — which makes the grouping visible.
*/

func m003PrecedenceAndAssociativity() {
	fmt.Println("\n--- Section 6: Operator Precedence and Associativity ---")

	// Level 5 beats level 4.
	fmt.Printf("2 + 3*4     = %d   (* before +)\n", 2+3*4)
	fmt.Printf("1 + 2<<3    = %d   (<< is level 5, so it happens first: 1 + 16)\n", 1+2<<3)
	fmt.Printf("6 & 3 + 1   = %d   (& is level 5: (6&3) + 1 = 2 + 1)\n", 6&3+1)

	// The C trap that does NOT exist in Go: & binds tighter than ==.
	fmt.Printf("6 & 3 == 2  = %t   (parses as (6&3) == 2; in C this would be 6 & (3==2))\n", 6&3 == 2)

	// + and | share level 4, so this is (3+5) | 6. C parses it the same way.
	fmt.Printf("3 + 5 | 6   = %d  (level 4, left to right: (3+5) | 6 = 8|6)\n", 3+5|6)

	// The Go traps that C programmers really do not expect: << is TIGHTER than +
	// in Go but looser in C, and ^ (XOR) sits at level 4 in Go but much lower in C.
	fmt.Printf("1 + 2<<3    = %d  (Go: 1 + (2<<3) = 17; C would give (1+2)<<3 = 24)\n", 1+2<<3)
	fmt.Printf("3 ^ 5 + 1   = %d   (Go: 3 ^ (5+1) = 7; C would give (3^5) + 1 = 5)\n", 3^5+1)

	// Comparisons are looser than arithmetic, tighter than && and ||.
	a, b, c, d := 1, 2, 3, 4
	fmt.Printf("a+1 == b && c < d  = %t   (no parentheses needed)\n", a+1 == b && c < d)

	// && binds tighter than ||.
	yes, no, alsoNo := true, false, false
	fmt.Printf("yes || no && alsoNo   = %t  (&& first, so it is yes || (no && alsoNo))\n",
		yes || no && alsoNo)
	fmt.Printf("(yes || no) && alsoNo = %t  (forcing the other grouping)\n",
		(yes || no) && alsoNo)

	// Left associativity.
	fmt.Printf("100 / 10 / 2 = %d   ((100/10)/2, not 100/(10/2))\n", 100/10/2)
	fmt.Printf("10 - 3 - 2   = %d   ((10-3)-2)\n", 10-3-2)

	// gofmt communicates precedence through spacing - note how it formatted these:
	x := 2
	fmt.Printf("gofmt writes tighter binding with less space: %d\n", 1+x*3)

	// Unary operators bind tighter than any binary one.
	p := &a
	fmt.Printf("*p + 1 = %d  (dereference first)\n", *p+1)
	fmt.Printf("-a * 2 = %d  (negate first)\n", -a*2)
	fmt.Printf("!(a == b) = %t\n", !(a == b))

	_ = d
}

// Run003 runs every section of module 003 in order.
func Run003() {
	m003ArithmeticOperators()
	m003ComparisonOperators()
	m003LogicalOperators()
	m003AssignmentOperators()
	m003BitwiseOperators()
	m003PrecedenceAndAssociativity()
}

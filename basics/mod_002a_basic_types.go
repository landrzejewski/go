package basics

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

/*
# Module 002a — Basic Types

Go's type system starts from a small set of predeclared types. This module covers the scalar ones
plus strings; module 002b covers the composite ones (arrays, slices, maps, structs).

The single rule that shapes everything here: **Go has no implicit numeric conversion.** Not between
integer sizes, not between signed and unsigned, not between integers and floats. Every mixing of
types needs an explicit conversion `T(v)`. This is verbose, and it is also why Go programs have
almost no accidental-truncation bugs.
*/

// =================================================================================================
// Section 1: Overview and Value Semantics
// =================================================================================================

/*
## Overview and Value Semantics

- The predeclared types are:
    - booleans: `bool`
    - integers: `int8 int16 int32 int64`, `uint8 uint16 uint32 uint64`, `int`, `uint`, `uintptr`
    - aliases: `byte` = `uint8`, `rune` = `int32`
    - floats: `float32 float64`
    - complex: `complex64 complex128`
    - strings: `string`
- **Everything in Go is a value type.** Assignment and argument passing always copy. There are no
  reference types and no implicit sharing — a point covered in depth in module 006. Some types (a
  slice, a map, a channel) *contain* a pointer, so copying them shares the underlying data, but the
  copy itself is still a copy of a small header.
- A **defined type** created from another type (`type Celsius float64`) is a *distinct* type. It
  shares the underlying representation but not the identity: you cannot assign a `Celsius` to a
  `float64` without a conversion, and that is the point (module 002b, Section 13).
- Conversions are written `T(v)` and are only permitted between types with compatible underlying
  representations. A conversion may lose information — silently. `int8(300)` is `44`, and
  `int(3.9)` is `3`, truncated toward zero. The compiler will not warn you.
- Conversions of a **constant** are checked at compile time, so `int8(300)` written literally is
  rejected — only conversions of *variables* can silently wrap.
*/

func m002aOverview() {
	fmt.Println("--- Section 1: Overview and Value Semantics ---")

	// Assignment copies. `b` is an independent value.
	a := 10
	b := a
	b = 20
	fmt.Printf("a=%d b=%d - assignment copied, it did not alias\n", a, b)

	// No implicit conversion, not even between int and int64.
	var i32 int32 = 5
	var i64 int64 = int64(i32) // the conversion is mandatory
	fmt.Printf("int32 %d -> int64 %d (explicit conversion required)\n", i32, i64)
	//	var bad int64 = i32 // ERROR: cannot use i32 (variable of type int32) as int64 value in variable declaration

	// Not even between int and its own alias-sized cousin:
	var plain int = 5
	//	var also int64 = plain // ERROR: cannot use plain (variable of type int) as int64 value in variable declaration
	fmt.Println("`int` is its own type, distinct from int64 even on a 64-bit machine:", plain)

	// --- Conversions can lose information silently ---
	large := 300
	fmt.Printf("int8(300) via a variable = %d (wrapped, no warning)\n", int8(large))

	pi := 3.9
	fmt.Printf("int(3.9) = %d (truncated toward zero, not rounded)\n", int(pi))
	fmt.Printf("int(-3.9) = %d (also toward zero)\n", int(-pi))
	fmt.Printf("to round, use math.Round: %d\n", int(math.Round(pi)))

	// A CONSTANT conversion is checked at compile time, so the literal form is caught:
	//	fmt.Println(int8(300)) // ERROR: constant 300 overflows int8
	fmt.Println("int8(300) written literally is a compile error; only variables wrap silently")
}

// =================================================================================================
// Section 2: Integer Types
// =================================================================================================

/*
## Integer Types

- Signed: `int8`, `int16`, `int32`, `int64`. Unsigned: `uint8`, `uint16`, `uint32`, `uint64`.
- `int` and `uint` are **platform-dependent**: 64 bits on all modern targets (amd64, arm64), 32 bits
  on 386/arm. They are *distinct types* from `int64`/`int32` even when the same size.
- `int` is the default. Use a sized type only when the size matters: binary formats, wire protocols,
  memory-critical arrays, or interop. Do not reach for `int32` to "save memory" in ordinary code.
- `uintptr` is an integer big enough to hold a pointer bit pattern. It is for `unsafe` work only,
  and it does **not** keep the pointed-to object alive from the garbage collector's point of view.
- **Overflow wraps silently.** Signed overflow is defined as two's-complement wraparound — it is not
  undefined behaviour as in C, and it does not panic as in Rust's debug builds. `math.MaxInt8 + 1`
  computed on variables gives `-128`, quietly. There are no `checked_add` / `saturating_add`
  helpers; if you need overflow detection you must write the check yourself, or use `math/bits`.
- **Division by zero panics** for integers (`runtime error: integer divide by zero`). This differs
  from floats, which produce `±Inf` or `NaN` (Section 3).
- Integer division **truncates toward zero** and `%` is a **remainder**, keeping the sign of the
  left operand: `-7 / 2 == -3` and `-7 % 3 == -1`. Go has no built-in Euclidean modulus, so the
  usual idiom for a non-negative result is `((a % n) + n) % n`.
- Limits live in `math` as **untyped constants**: `math.MaxInt`, `math.MinInt64`, `math.MaxUint8`
  and friends. Because they are untyped, `math.MaxUint64` cannot be assigned to an `int` — but it
  fits a `uint64` perfectly.
- Literals may be written decimal, binary `0b1010`, octal `0o755` (or the legacy `0755`), hex
  `0xFF`, and may carry `_` separators for readability: `1_000_000`.
*/

func m002aIntegerTypes() {
	fmt.Println("\n--- Section 2: Integer Types ---")

	// Sizes and ranges.
	fmt.Printf("int8   %d..%d\n", math.MinInt8, math.MaxInt8)
	fmt.Printf("int16  %d..%d\n", math.MinInt16, math.MaxInt16)
	fmt.Printf("int32  %d..%d\n", math.MinInt32, math.MaxInt32)
	fmt.Printf("int64  %d..%d\n", math.MinInt64, math.MaxInt64)
	fmt.Printf("uint8  0..%d\n", math.MaxUint8)
	fmt.Printf("int on this platform is %d bits\n", strconv.IntSize)

	// Literal forms, including underscore separators.
	decimal, binary, octal, hexa := 1_000_000, 0b1010, 0o755, 0xFF
	fmt.Printf("1_000_000=%d 0b1010=%d 0o755=%d 0xFF=%d\n", decimal, binary, octal, hexa)

	// --- Overflow wraps silently ---
	var maxByte uint8 = math.MaxUint8
	maxByte++ // no panic, no warning
	fmt.Printf("uint8 255 + 1 = %d (wrapped around)\n", maxByte)

	var minInt8 int8 = math.MinInt8
	minInt8--
	fmt.Printf("int8 -128 - 1 = %d (two's-complement wraparound, defined behaviour)\n", minInt8)

	// There is no checked_add; detect overflow yourself.
	x, y := math.MaxInt64, 1
	if x > math.MaxInt64-y {
		fmt.Println("overflow detected before it happens (the only way to do it in Go)")
	}

	// --- Division and remainder ---
	fmt.Printf("7/2=%d  -7/2=%d (truncates toward zero, not floor)\n", 7/2, -7/2)
	fmt.Printf("-7%%3=%d  7%%-3=%d (remainder keeps the LEFT operand's sign)\n", -7%3, 7%-3)

	// Go has no rem_euclid; this is the idiom for a non-negative modulus.
	euclid := func(a, n int) int { return ((a % n) + n) % n }
	fmt.Printf("euclid(-7, 3) = %d (always non-negative)\n", euclid(-7, 3))

	// Integer division by zero PANICS (floats do not - see Section 3).
	//	fmt.Println(1 / 0)        // ERROR (constant): invalid operation: division by zero
	//	zero := 0; fmt.Println(1 / zero) // panic: runtime error: integer divide by zero
	fmt.Println("integer division by zero panics; float division by zero does not")

	// Unsigned subtraction below zero wraps to a huge number - a classic loop bug.
	var count uint = 0
	count--
	fmt.Printf("uint 0 - 1 = %d - beware `for i := uint(len(s)-1); i >= 0; i--`, it never ends\n", count)

	// math.MaxUint64 is an untyped constant that fits uint64 but not int:
	var big uint64 = math.MaxUint64
	fmt.Println("math.MaxUint64 as uint64:", big)
	//	var bad int = math.MaxUint64 // ERROR: cannot use math.MaxUint64 (untyped int constant 18446744073709551615) as int value in variable declaration (overflows)
}

// =================================================================================================
// Section 3: Floating-Point and Complex Numbers
// =================================================================================================

/*
## Floating-Point and Complex Numbers

- `float32` (IEEE-754 single, ~7 significant decimal digits) and `float64` (double, ~15 digits).
  **`float64` is the default** for an untyped float constant, and the right choice unless you have a
  specific reason — graphics, ML tensors, or huge arrays — to halve the precision.
- Float division by zero does **not** panic: it yields `+Inf`, `-Inf`, or `NaN` for `0.0/0.0`.
  Use `math.IsInf` and `math.IsNaN` to test for them.
- **`NaN` is not equal to anything, including itself.** `nan == nan` is `false`. That makes floats
  only *partially* ordered, which is why `cmp.Compare` exists — it defines a total order where NaN
  sorts before everything, so `slices.Sort` on floats is well defined.
- Because a NaN key can never be found again, **never use a float as a map key** unless you have
  ruled NaN out.
- Binary floating point cannot represent most decimal fractions exactly, so `0.1 + 0.2 != 0.3`.
  Compare with a tolerance (`math.Abs(a-b) < epsilon`), and for money use integer minor units — this
  repo's `examples/monetary_amount.go` exists for exactly that reason.
- Go has **no implicit int-to-float promotion**. `someInt * someFloat` is a compile error; you must
  write `float64(someInt) * someFloat`.
- Complex numbers are built in: `complex64` and `complex128`, with literals like `1 + 2i` and the
  builtins `complex(re, im)`, `real(c)`, `imag(c)`. They are genuinely rare outside signal
  processing, but they are part of the language rather than a library.
*/

func m002aFloatingPointAndComplex() {
	fmt.Println("\n--- Section 3: Floating-Point and Complex Numbers ---")

	var f32 float32 = 1.0 / 3.0
	var f64 float64 = 1.0 / 3.0
	fmt.Printf("float32 1/3 = %.10f (~7 digits of precision)\n", f32)
	fmt.Printf("float64 1/3 = %.17f (~15 digits)\n", f64)

	// --- Division by zero does not panic ---
	zero := 0.0
	posInf, negInf, nan := 1.0/zero, -1.0/zero, zero/zero
	fmt.Printf("1.0/0.0=%v  -1.0/0.0=%v  0.0/0.0=%v\n", posInf, negInf, nan)
	fmt.Printf("math.IsInf(+Inf, 1)=%t math.IsNaN(nan)=%t\n", math.IsInf(posInf, 1), math.IsNaN(nan))

	// --- NaN equals nothing, not even itself ---
	fmt.Printf("nan == nan is %t - this is why floats are only partially ordered\n", nan == nan)
	fmt.Println("a NaN used as a map key can never be looked up again")

	// --- The precision trap ---
	sum := 0.1 + 0.2
	fmt.Printf("0.1+0.2 = %.20f, == 0.3 is %t\n", sum, sum == 0.3)
	const epsilon = 1e-9
	fmt.Printf("comparing with a tolerance instead: %t\n", math.Abs(sum-0.3) < epsilon)
	fmt.Println("for money, use integer minor units - see examples/monetary_amount.go")

	// --- No implicit promotion ---
	count := 3
	price := 1.5
	fmt.Printf("float64(count) * price = %.2f\n", float64(count)*price)
	//	fmt.Println(count * price) // ERROR: invalid operation: count * price (mismatched types int and float64)

	// --- Complex numbers ---
	c := 3 + 4i
	fmt.Printf("c=%v real=%.0f imag=%.0f abs=%.0f\n", c, real(c), imag(c), math.Hypot(real(c), imag(c)))
	fmt.Printf("complex(1, -1) squared = %v\n", complex(1, -1)*complex(1, -1))
}

// =================================================================================================
// Section 4: Booleans
// =================================================================================================

/*
## Booleans

- `bool` has exactly two values, `true` and `false`, which are **predeclared constants**, not
  keywords — so they can be shadowed (module 001a, Section 6).
- A `bool` is **not** a number. There is no truthiness in Go: `if 1 {}` and `if someString {}` are
  compile errors, and `bool` cannot be converted to or from `int`. This eliminates a whole family of
  C bugs, at the cost of writing `if n != 0` explicitly.
- `&&` and `||` **short-circuit**: the right operand is not evaluated when the left decides the
  result. This is what makes `if p != nil && p.Field > 0` safe.
- `&&` binds tighter than `||`, and both bind looser than every comparison — so
  `a == b && c > d || e` parses as `((a == b) && (c > d)) || e`. See module 003 for the full table.
- The zero value is `false`, which is usually the right default: `var found bool` starts out
  correctly meaning "not found yet".
*/

func m002aBooleans() {
	fmt.Println("\n--- Section 4: Booleans ---")

	var yes, no = true, false
	fmt.Printf("yes=%t no=%t !yes=%t\n", yes, no, !yes)

	// No truthiness, and no conversion to or from numbers.
	//	if 1 { }                  // ERROR: non-boolean condition in if statement
	//	var n int = int(yes)      // ERROR: cannot convert yes (variable of type bool) to type int
	n := 0
	fmt.Println("`if n != 0` must be written out; there is no truthiness:", n != 0)

	// The idiom when you really do need 0/1:
	toInt := func(b bool) int {
		if b {
			return 1
		}
		return 0
	}
	fmt.Println("toInt(true)=", toInt(true), "toInt(false)=", toInt(false))

	// --- Short-circuit evaluation ---
	calls := 0
	sideEffect := func() bool { calls++; return true }
	_ = false && sideEffect() // right operand never runs
	_ = true || sideEffect()  // right operand never runs
	fmt.Printf("after two short-circuited expressions, sideEffect ran %d times\n", calls)

	// This is what makes the nil guard safe: the dereference is never reached when p is nil.
	var p *m002aPoint
	if p != nil && p.X > 0 {
		fmt.Println("unreachable")
	}
	fmt.Println("`p != nil && p.X > 0` is safe because && short-circuits")
}

type m002aPoint struct{ X, Y int }

// =================================================================================================
// Section 5: Strings
// =================================================================================================

/*
## Strings

- A `string` is an **immutable, read-only slice of bytes**. Internally it is a two-word header: a
  pointer to the bytes and a length. It is *not* nil-terminated, and its zero value is `""`, never
  nil.
- **A Go string is not guaranteed to be valid UTF-8.** Source-code string literals are UTF-8 because
  Go source is, but a string built from arbitrary bytes — a file, a socket, a database blob — may
  hold anything. `utf8.ValidString` is how you check.
- **Indexing yields a byte, not a character**: `s[0]` has type `byte` (`uint8`). For any non-ASCII
  text this is not the first character. `len(s)` is likewise the length **in bytes**.
- Strings are immutable, so `s[0] = 'X'` does not compile. To modify, convert to `[]byte` or
  `[]rune`, change, and convert back — each conversion **copies**.
- Slicing `s[i:j]` is a byte range and is **O(1)**: it shares the underlying bytes rather than
  copying. Slicing in the middle of a multi-byte rune silently produces invalid UTF-8; unlike Rust,
  Go does not panic on a non-boundary index.
- Concatenation with `+` allocates a new string every time. In a loop that is quadratic: use
  `strings.Builder` (or `bytes.Buffer`), which amortises the allocations.
- Comparison operators work directly on strings and compare **byte-wise, lexicographically**. That
  is not the same as a locale-aware or Unicode-normalised comparison — `golang.org/x/text/collate`
  exists for that.
- Two literal forms:
    - **Interpreted** `"..."`: escapes are processed (`\n`, `\t`, `\"`, `\\`, `\u00e9`, `\x41`) and
      the literal cannot span lines.
    - **Raw** `` `...` ``: no escapes at all, backslashes are literal, and it *can* span lines.
      This is the right form for regular expressions, Windows paths, JSON fixtures and struct tags.
      The only thing it cannot contain is a backquote.
*/

func m002aStrings() {
	fmt.Println("\n--- Section 5: Strings ---")

	s := "Hello, Gophers"
	fmt.Printf("s=%q len=%d (BYTES, not characters)\n", s, len(s))

	// Indexing gives a byte. Note %d vs %c.
	fmt.Printf("s[0] = %d as a number, %c as a character, type %T\n", s[0], s[0], s[0])

	// --- Bytes, not characters ---
	polish := "Zażółć gęślą jaźń"
	fmt.Printf("%q: len=%d bytes but %d runes\n", polish, len(polish), utf8.RuneCountInString(polish))
	fmt.Printf("polish[1] = %d - a fragment of a two-byte rune, not 'a'\n", polish[1])

	// A string need not be valid UTF-8.
	invalid := string([]byte{0xff, 0xfe, 0x41})
	fmt.Printf("arbitrary bytes as a string: valid UTF-8? %t\n", utf8.ValidString(invalid))

	// --- Immutability ---
	//	s[0] = 'h' // ERROR: cannot assign to s[0] (neither addressable nor a map index expression)
	modified := []byte(s) // copies
	modified[0] = 'h'
	fmt.Printf("to modify, round-trip through []byte: %q (original still %q)\n", string(modified), s)

	// --- Slicing is by byte and is O(1) ---
	fmt.Printf("s[7:14] = %q (byte range, shares memory, no copy)\n", s[7:14])
	fmt.Printf("cutting a rune in half: %q -> %q (invalid UTF-8, no panic)\n",
		polish[:3], polish[:2])

	// --- Building strings ---
	var sb strings.Builder
	for i := range 3 {
		fmt.Fprintf(&sb, "part%d ", i) // Builder implements io.Writer
	}
	fmt.Printf("strings.Builder result: %q\n", strings.TrimSpace(sb.String()))
	fmt.Println("`s += x` in a loop is quadratic; Builder is linear")

	// --- Comparison is byte-wise ---
	fmt.Printf("\"apple\" < \"banana\" = %t, \"Z\" < \"a\" = %t (uppercase sorts first in ASCII)\n",
		"apple" < "banana", "Z" < "a")

	// --- Literal forms ---
	interpreted := "line1\nline2\ttabbed \u00e9 \x41"
	raw := `no escapes here: \n stays literal, C:\Users\lukas`
	fmt.Printf("interpreted: %q\n", interpreted)
	fmt.Printf("raw:         %s\n", raw)
	multiline := `raw strings
can span
several lines`
	fmt.Printf("raw multiline has %d lines\n", strings.Count(multiline, "\n")+1)
}

// =================================================================================================
// Section 6: Bytes, Runes and Iterating Text
// =================================================================================================

/*
## Bytes, Runes and Iterating Text

- `byte` is an alias for `uint8`; `rune` is an alias for `int32`. They are **true aliases**, not
  defined types, so `byte` and `uint8` are the same type and interchangeable everywhere. The names
  exist purely to document intent: "this is a raw byte" versus "this is a Unicode code point".
- A **rune is a Unicode code point**, not a character as a user perceives one. A user-perceived
  character (a *grapheme cluster*) may be several runes: `é` can be one code point or `e` followed
  by a combining accent, and an emoji with a skin-tone modifier is several runes. Neither `len` nor
  `RuneCountInString` gives you "the number of characters"; for that you need
  `golang.org/x/text/unicode/norm` or a grapheme-cluster library.
- Rune literals use single quotes: `'A'`, `'ż'`, `'\n'`, `'\u00e9'`. Their type is `rune`, so they
  print as **numbers** unless you use `%c`, `%q` or `string(r)`.
- **`for i := range s` over a string decodes UTF-8**: `i` is the *byte* offset of each rune, `r` is
  the rune. The index therefore jumps by more than one for multi-byte runes. Invalid bytes decode to
  `utf8.RuneError` (U+FFFD) and advance by one.
- Indexing `s[i]` does **not** decode — it is a raw byte. This is the crucial asymmetry: `range`
  gives runes, `[]` gives bytes.
- Conversions: `[]byte(s)` and `[]rune(s)` both **copy**. `string(bytes)` and `string(runes)` copy
  back. `string(65)` is `"A"` (a rune conversion), while `strconv.Itoa(65)` is `"65"` — `go vet`
  flags the former as a likely mistake.
- `unicode` classifies runes (`IsLetter`, `IsDigit`, `IsSpace`, `ToUpper`); `unicode/utf8` handles
  encoding (`RuneCountInString`, `DecodeRuneInString`, `ValidString`).
*/

func m002aBytesAndRunes() {
	fmt.Println("\n--- Section 6: Bytes, Runes and Iterating Text ---")

	// byte and rune are aliases, so these types are identical - no conversion needed.
	var b byte = 65
	var u8 uint8 = b // legal: byte IS uint8
	var r rune = 'A'
	var i32 int32 = r // legal: rune IS int32
	fmt.Printf("byte=%d uint8=%d rune=%d int32=%d, %%c gives %c\n", b, u8, r, i32, r)

	// --- range decodes UTF-8; indexing does not ---
	s := "Gęś"
	fmt.Printf("%q: len=%d bytes, %d runes\n", s, len(s), utf8.RuneCountInString(s))

	fmt.Println("range over the string (byte offset, rune):")
	for i, r := range s {
		fmt.Printf("  i=%d rune=%q codepoint=U+%04X width=%d\n", i, r, r, utf8.RuneLen(r))
	}

	fmt.Println("indexing the same string (raw bytes):")
	for i := range len(s) {
		fmt.Printf("  s[%d]=%d(0x%02X)\n", i, s[i], s[i])
	}

	// Invalid bytes decode to U+FFFD and advance by one.
	broken := string([]byte{0x47, 0xff, 0x53})
	for i, r := range broken {
		fmt.Printf("  broken i=%d rune=%q isRuneError=%t\n", i, r, r == utf8.RuneError)
	}

	// --- Conversions ---
	runes := []rune(s)
	fmt.Printf("[]rune gives random access by character: runes[1]=%q, len=%d\n", runes[1], len(runes))
	fmt.Printf("reversed by runes: %q\n", m002aReverse(s))

	// string(int) is a RUNE conversion, not a number-to-text conversion.
	fmt.Printf("string(rune(65))=%q but strconv.Itoa(65)=%q\n", string(rune(65)), strconv.Itoa(65))
	fmt.Println("`string(someInt)` is flagged by go vet: it means the code point, not the digits")

	// --- unicode classification ---
	for _, r := range []rune{'G', 'ę', '7', ' ', '!'} {
		fmt.Printf("  %q letter=%t digit=%t space=%t upper=%q\n",
			r, unicode.IsLetter(r), unicode.IsDigit(r), unicode.IsSpace(r), unicode.ToUpper(r))
	}

	// A grapheme cluster can be several runes: neither len nor RuneCount is "characters".
	combining := "e\u0301" // 'e' + combining acute accent, displayed as é
	fmt.Printf("%q: %d bytes, %d runes, but 1 perceived character\n",
		combining, len(combining), utf8.RuneCountInString(combining))
}

// m002aReverse reverses a string by runes, so multi-byte characters survive intact.
func m002aReverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// =================================================================================================
// Section 7: Conversions and strconv
// =================================================================================================

/*
## Conversions and strconv

- Two different operations are easy to confuse:
    - **Conversion** `T(v)` reinterprets a value as another type. `float64(i)`, `[]byte(s)`,
      `MyInt(n)`. It never fails and never returns an error.
    - **Parsing / formatting** turns text into a value or back. That is `strconv`, and it *can*
      fail, so it returns an `error`.
- `strconv` is the right package, not `fmt`, for single values: `Atoi`/`Itoa` for ints,
  `ParseInt`/`FormatInt` with an explicit base and bit size, `ParseFloat`/`FormatFloat`,
  `ParseBool`/`FormatBool`, `Quote`/`Unquote`. It is substantially faster than `fmt.Sprintf` and its
  errors are typed (`*strconv.NumError`, wrapping `strconv.ErrSyntax` or `strconv.ErrRange`).
- `ParseInt(s, base, bitSize)`: `base` 0 means "infer from the prefix" (`0x`, `0b`, `0o`, leading
  `0`), and `bitSize` is the width the result must fit in — it still *returns* an `int64`, which you
  then convert.
- `FormatFloat(f, fmt, prec, bitSize)`: `'f'` for plain decimal, `'e'` for scientific, `'g'` for the
  shortest of the two; `prec` of `-1` means "the fewest digits that round-trips exactly".
- **Never use `string(someInt)`** to render a number — it produces the character at that code point.
  Use `strconv.Itoa`.
*/

func m002aConversionsAndStrconv() {
	fmt.Println("\n--- Section 7: Conversions and strconv ---")

	// --- Conversion: cannot fail, may truncate ---
	i := 42
	fmt.Printf("conversions: float64(%d)=%v int8(%d)=%d byte(%d)=%d\n",
		i, float64(i), i, int8(i), i, byte(i))

	// --- Parsing: can fail, so it returns an error ---
	if n, err := strconv.Atoi("123"); err == nil {
		fmt.Printf("strconv.Atoi(\"123\") = %d\n", n)
	}
	if _, err := strconv.Atoi("12x"); err != nil {
		fmt.Printf("strconv.Atoi(\"12x\") failed: %v\n", err)
		// The error is typed and carries the reason.
		var numErr *strconv.NumError
		if ok := m002aAsNumError(err, &numErr); ok {
			fmt.Printf("  typed error: func=%s input=%q reason=%v\n",
				numErr.Func, numErr.Num, numErr.Err)
		}
	}

	// Range errors are distinct from syntax errors.
	if _, err := strconv.ParseInt("999999999999999999999", 10, 64); err != nil {
		fmt.Printf("out of range: %v\n", err)
	}

	// base 0 infers the prefix; bitSize constrains the width.
	for _, in := range []string{"0xFF", "0b1010", "0o755", "255"} {
		v, err := strconv.ParseInt(in, 0, 64)
		fmt.Printf("ParseInt(%q, base 0) = %d (err=%v)\n", in, v, err)
	}

	// --- Formatting ---
	fmt.Printf("Itoa(255)=%q FormatInt(255,2)=%q FormatInt(255,16)=%q\n",
		strconv.Itoa(255), strconv.FormatInt(255, 2), strconv.FormatInt(255, 16))
	fmt.Printf("FormatFloat(1/3, 'f', 4)=%q  prec -1 (shortest round-trip)=%q\n",
		strconv.FormatFloat(1.0/3.0, 'f', 4, 64), strconv.FormatFloat(1.0/3.0, 'f', -1, 64))
	fmt.Printf("Quote(%s)=%s\n", `"a\tb"`, strconv.Quote("a\tb"))

	fmt.Printf("ParseBool(\"1\")/(\"true\")/(\"T\") all work: ")
	for _, in := range []string{"1", "true", "T"} {
		v, _ := strconv.ParseBool(in)
		fmt.Printf("%t ", v)
	}
	fmt.Println()
}

// m002aAsNumError is a tiny helper standing in for errors.As, which module 009 covers properly.
func m002aAsNumError(err error, target **strconv.NumError) bool {
	if ne, ok := err.(*strconv.NumError); ok {
		*target = ne
		return true
	}
	return false
}

// Run002a runs every section of module 002a in order.
func Run002a() {
	m002aOverview()
	m002aIntegerTypes()
	m002aFloatingPointAndComplex()
	m002aBooleans()
	m002aStrings()
	m002aBytesAndRunes()
	m002aConversionsAndStrconv()
}

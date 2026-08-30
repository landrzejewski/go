package basics

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"uuid"
)

/*
# Module 013 — Standard Library Essentials

Go's standard library is unusually large and unusually good, and knowing it is most of knowing Go.
This module covers the packages you will reach for in almost every program. Modules 011 and 012
already covered `sync`, `context`, `slices`, `maps`, `cmp` and `iter`; module 014 covers `testing`.

The newest material is at the end: **`encoding/json/v2`** and the **`uuid`** package, both new and
stable in Go 1.27.
*/

// =================================================================================================
// Section 1: fmt
// =================================================================================================

/*
## fmt

- Three families, each with the same verb set:
    - **print to stdout**: `Print`, `Println`, `Printf`
    - **return a string**: `Sprint`, `Sprintln`, `Sprintf`
    - **write to an io.Writer**: `Fprint`, `Fprintln`, `Fprintf`
  plus `Errorf` (module 009) and the `Scan` family for input.
- The verbs worth memorising:

	%v   the default format          %+v  with struct field names
	%#v  Go syntax                   %T   the TYPE, not the value
	%d   decimal    %b binary        %o octal   %x/%X hex
	%f   decimal float               %e scientific   %g the shorter of the two
	%s   string     %q quoted        %c the character for a rune code point
	%t   boolean    %p pointer       %%  a literal percent

- **Width and precision**: `%8.2f` is width 8, 2 decimals. `%-10s` left-aligns. `%08d` zero-pads.
  A `*` takes the width from an argument: `%*d`.
- `%v` on a type implementing **`fmt.Stringer`** calls `String()`; on an `error` it calls `Error()`.
  Calling a verb on the receiver *inside* `String()` recurses infinitely — convert to the underlying
  type first (module 008, Section 6).
- **`Print` adds spaces only between operands when neither is a string**; `Println` always adds
  spaces and a newline. This asymmetry surprises people constantly.
- **`go vet`'s printf check** is one of the highest-value checks in the toolchain: it catches verb
  and argument mismatches, and it understands your own wrappers if they are named `...f`.
- For performance-critical formatting, `strconv` beats `fmt` substantially (module 002a, Section 7).
*/

type m013Point struct {
	X, Y int
	name string
}

func m013Fmt() {
	fmt.Println("--- Section 1: fmt ---")

	p := m013Point{X: 1, Y: 2, name: "origin-ish"}
	fmt.Printf("  %%v   %v\n", p)
	fmt.Printf("  %%+v  %+v\n", p)
	fmt.Printf("  %%#v  %#v\n", p)
	fmt.Printf("  %%T   %T\n", p)

	// Numeric verbs.
	n := 255
	fmt.Printf("  %%d=%d %%b=%b %%o=%o %%x=%x %%X=%X %%c=%c\n", n, n, n, n, n, 65)

	// Float verbs.
	f := 1234.5678
	fmt.Printf("  %%f=%f %%.2f=%.2f %%e=%e %%g=%g\n", f, f, f, f)

	// Width, alignment and padding.
	fmt.Printf("  |%8.2f| |%-10s| |%08d| |%*d|\n", f, "left", 42, 6, 7)

	// String verbs.
	s := "quoted\ttext"
	fmt.Printf("  %%s=%s %%q=%q\n", s, s)

	// Print versus Println.
	fmt.Print("  Print: ")
	fmt.Print(1, 2, "a", "b", 3)
	fmt.Println()
	fmt.Print("  Println: ")
	fmt.Println(1, 2, "a", "b", 3)
	fmt.Println("  Print adds a space only between two non-strings; Println always does")

	// Sprintf and Fprintf.
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Fprintf writes to any io.Writer")
	fmt.Printf("  Sprintf: %q\n", fmt.Sprintf("%s-%03d", "id", 7))
	fmt.Printf("  Fprintf: %q\n", buf.String())

	// Stringer.
	fmt.Printf("  a Stringer is used by %%v: %v\n", m008Temperature(21.5))

	// Scanning.
	var a int
	var word string
	_, _ = fmt.Sscanf("42 gopher", "%d %s", &a, &word)
	fmt.Printf("  Sscanf parsed: %d %q\n", a, word)

	fmt.Println("  `go vet` checks every verb against its argument - heed it")
}

// =================================================================================================
// Section 2: strings, strconv and bytes
// =================================================================================================

/*
## strings, strconv and bytes

- **`strings`** is the workhorse: `Contains`, `HasPrefix`, `HasSuffix`, `Index`, `LastIndex`,
  `Split`, `SplitN`, `Join`, `Replace`, `ReplaceAll`, `Trim*`, `ToUpper`, `ToLower`, `Fields`,
  `Repeat`, `EqualFold`, `Count`, `Title` (deprecated — use `golang.org/x/text/cases`).
- **`Cut`** (Go 1.18) is almost always what you want instead of `Split`: `before, after, found :=
  strings.Cut(s, sep)` splits at the **first** separator and tells you whether it was there.
- **`CutPrefix` / `CutSuffix`** (Go 1.20) replace the `HasPrefix` + slice pair.
- **`CutLast`** (**Go 1.27**) is the missing mirror of `Cut`: it splits at the **last** separator.
  It is exactly what you need for splitting a path from its base, or a name from its extension.
- **`strings.Builder`** for building strings in a loop — `+=` is quadratic. `Builder` implements
  `io.Writer`, so `fmt.Fprintf(&sb, ...)` works. It must not be copied after first use.
- **`strings.NewReader`** turns a string into an `io.Reader`, which is how you test any function
  taking a reader without touching the filesystem.
- **`strings.NewReplacer`** does many replacements in a single pass and is safe for concurrent use.
- **`bytes`** mirrors `strings` almost function for function, for `[]byte`. Use it when you already
  have bytes: converting to a `string` copies. `bytes.Buffer` is the mutable counterpart to
  `strings.Builder` and also implements `io.Reader`. **Go 1.26** added `Buffer.Peek`, to look at
  the next bytes without consuming them.
- **`strconv`** for number/string conversion (module 002a, Section 7). Never `string(someInt)`.
*/

func m013StringsAndBytes() {
	fmt.Println("\n--- Section 2: strings, strconv and bytes ---")

	s := "Go is expressive, concise, clean, and efficient"
	fmt.Printf("  Contains=%t HasPrefix=%t Index=%d Count(\", \")=%d\n",
		strings.Contains(s, "clean"), strings.HasPrefix(s, "Go"),
		strings.Index(s, "concise"), strings.Count(s, ", "))
	fmt.Printf("  Fields: %v\n", strings.Fields("  split   on   whitespace  "))
	fmt.Printf("  Split/Join round trip: %q\n", strings.Join(strings.Split("a,b,c", ","), " | "))
	fmt.Printf("  EqualFold(\"Go\", \"GO\")=%t (case-insensitive, Unicode-aware)\n",
		strings.EqualFold("Go", "GO"))
	fmt.Printf("  TrimSpace=%q TrimPrefix=%q\n",
		strings.TrimSpace("  padded  "), strings.TrimPrefix("prefix-value", "prefix-"))

	// --- Cut, CutPrefix, CutSuffix ---
	before, after, found := strings.Cut("key=value=extra", "=")
	fmt.Printf("  Cut at the FIRST separator:  before=%q after=%q found=%t\n", before, after, found)

	// --- CutLast: new in Go 1.27 ---
	dir, base, found := strings.CutLast("/usr/local/bin/go", "/")
	fmt.Printf("  CutLast at the LAST separator (Go 1.27): dir=%q base=%q found=%t\n", dir, base, found)
	name, ext, _ := strings.CutLast("archive.tar.gz", ".")
	fmt.Printf("  CutLast for an extension: name=%q ext=%q\n", name, ext)
	fmt.Println("  before 1.27 this needed LastIndex plus two slice expressions")

	if rest, ok := strings.CutPrefix("v1.27.0", "v"); ok {
		fmt.Printf("  CutPrefix (Go 1.20): %q\n", rest)
	}

	// --- Builder ---
	var sb strings.Builder
	sb.Grow(64) // preallocate when you know roughly how much you need
	for i := range 4 {
		fmt.Fprintf(&sb, "[%d]", i)
	}
	fmt.Printf("  strings.Builder: %q (it implements io.Writer)\n", sb.String())

	// --- Replacer: many replacements in one pass ---
	r := strings.NewReplacer("<", "&lt;", ">", "&gt;", "&", "&amp;")
	fmt.Printf("  NewReplacer: %q\n", r.Replace("<b>a & b</b>"))

	// --- bytes ---
	b := []byte("bytes mirrors strings")
	fmt.Printf("  bytes.Contains=%t bytes.ToUpper=%q\n",
		bytes.Contains(b, []byte("mirror")), bytes.ToUpper(b[:5]))

	buf := bytes.NewBufferString("peek at me")
	peeked, _ := buf.Peek(4) // Go 1.26: look without consuming
	fmt.Printf("  bytes.Buffer.Peek (Go 1.26): peeked=%q, buffer still has %d bytes\n",
		peeked, buf.Len())

	// --- strconv, once more ---
	fmt.Printf("  strconv.Quote(%q)=%s  Itoa(42)=%q  FormatBool(true)=%q\n",
		"tab\there", strconv.Quote("tab\there"), strconv.Itoa(42), strconv.FormatBool(true))
}

// =================================================================================================
// Section 3: time
// =================================================================================================

/*
## time

- Two distinct types: **`time.Time`** is an instant, **`time.Duration`** is an `int64` count of
  nanoseconds. They are values and are copied freely.
- **`Duration` is an integer type**, so an untyped constant adapts to it: `5 * time.Second` works,
  but `n * time.Second` for an `int` variable `n` does **not** — write
  `time.Duration(n) * time.Second`.
- **Never compare `time.Time` with `==`.** A `Time` carries a wall clock, a monotonic reading and a
  location, so two instants that represent the same moment can compare unequal. Use `Equal`,
  `Before`, `After`, and `Compare` (Go 1.20).
- The **reference layout** is the single strangest thing in the standard library: instead of
  `%Y-%m-%d`, you write the *specific date* `Mon Jan 2 15:04:05 MST 2006` — that is
  `01/02 03:04:05PM '06 -0700`, the digits 1 through 7. Use the `time.RFC3339` constant and friends
  whenever you can.
- `time.Now()` includes a **monotonic clock** reading, which is what makes `time.Since` immune to
  wall-clock adjustments. `Add`/`AddDate` keep it, but `Round`, `Truncate`, `UTC`, `Local` and
  `In` strip it, so compare durations before converting time zones.
- **Timers**: `time.After` in a `select` is convenient but allocates a timer per call; before Go
  1.23 it was not collected until it fired, which leaked in hot loops. `time.NewTimer` +
  `defer t.Stop()` is the careful form. `time.Tick` leaks its ticker forever — use
  `time.NewTicker` and `Stop` it.
- **Testing time is hard**, and the answer is now `testing/synctest` (Go 1.25), which gives a test a
  fake clock so that a one-hour timeout completes instantly and deterministically. Module 014.
*/

func m013Time() {
	fmt.Println("\n--- Section 3: time ---")

	// A fixed instant, so this module's output is reproducible.
	t := time.Date(2026, time.August, 30, 14, 5, 6, 0, time.UTC)
	fmt.Printf("  the instant: %v\n", t)

	// --- The reference layout ---
	fmt.Printf("  RFC3339:  %s\n", t.Format(time.RFC3339))
	fmt.Printf("  custom:   %s\n", t.Format("2006-01-02 15:04:05"))
	fmt.Printf("  friendly: %s\n", t.Format("Mon, 02 Jan 2006 at 3:04 PM"))
	fmt.Println("  the layout is the DATE 2006-01-02 15:04:05, not a percent-code string like in C")

	parsed, err := time.Parse(time.RFC3339, "2026-08-30T14:05:06Z")
	fmt.Printf("  Parse round trip: %v (err=%v, Equal=%t)\n", parsed, err, parsed.Equal(t))

	// --- Durations ---
	d := 90 * time.Minute
	fmt.Printf("  90*time.Minute = %v = %.1f hours = %d ns\n", d, d.Hours(), d.Nanoseconds())
	fmt.Printf("  ParseDuration(\"1h30m\") = %v\n", m013MustDuration("1h30m"))
	fmt.Printf("  Truncate/Round: %v / %v\n",
		(93 * time.Minute).Truncate(time.Hour), (93 * time.Minute).Round(time.Hour))

	// An int VARIABLE needs a conversion.
	n := 5
	fmt.Printf("  time.Duration(n) * time.Second = %v\n", time.Duration(n)*time.Second)
	//	fmt.Println(n * time.Second) // ERROR: invalid operation: n * time.Second (mismatched types int and time.Duration)

	// --- Arithmetic and comparison ---
	later := t.Add(36 * time.Hour)
	fmt.Printf("  Add(36h) = %v, Sub = %v\n", later.Format(time.RFC3339), later.Sub(t))
	fmt.Printf("  Before=%t After=%t Compare=%d\n", t.Before(later), t.After(later), t.Compare(later))
	fmt.Println("  never compare two time.Time with == : use Equal, Before, After, Compare")

	// --- Components and locations ---
	fmt.Printf("  Year=%d Month=%v Day=%d Weekday=%v YearDay=%d\n",
		t.Year(), t.Month(), t.Day(), t.Weekday(), t.YearDay())
	if warsaw, err := time.LoadLocation("Europe/Warsaw"); err == nil {
		fmt.Printf("  in Europe/Warsaw: %s\n", t.In(warsaw).Format(time.RFC3339))
	}

	// --- Measuring ---
	start := time.Now()
	time.Sleep(2 * time.Millisecond)
	fmt.Printf("  time.Since uses the MONOTONIC clock, so it is immune to clock changes: %v\n",
		time.Since(start).Round(time.Millisecond))

	fmt.Println("  time.Tick leaks its ticker forever - use time.NewTicker and Stop it")
	fmt.Println("  to test timeouts deterministically, use testing/synctest (module 014)")
}

func m013MustDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		panic(err)
	}
	return d
}

// =================================================================================================
// Section 4: os, io, bufio and os.Root
// =================================================================================================

/*
## os, io, bufio and os.Root

- **`os`** is the process and filesystem interface: `Args`, `Getenv`/`LookupEnv`, `Exit`,
  `Open`/`Create`/`OpenFile`, `ReadFile`/`WriteFile`, `Remove`, `MkdirAll`, `CreateTemp`, `Stat`.
- `os.ReadFile` and `os.WriteFile` are the one-line forms and should be your default for anything
  that fits in memory. Reach for `os.Open` plus a `bufio.Scanner` only when streaming.
- **`os.Exit` does not run deferred functions.** Never call it from anywhere but the very top of
  `main`, and never in a library.
- **`io`** is the interface layer: `Reader`, `Writer`, `Closer`, `Copy`, `ReadAll`, `WriteString`,
  `MultiReader`, `MultiWriter`, `TeeReader`, `LimitReader`, `Discard`, and the `io.EOF` sentinel.
- **`bufio`** wraps a reader or writer with a buffer: `Scanner` for line-at-a-time reading (with
  `Buffer` to raise its 64 KB line cap), `Reader` for `ReadString`, `Writer` — whose `Flush` you
  **must** call, and whose deferred `Flush` error you should check.
- **`os.Root`** (**Go 1.24**, extended in 1.25) is the answer to path-traversal attacks: it opens a
  directory and confines every subsequent operation to it. A `..` or an absolute path or a symlink
  escaping the root is refused by the OS, not by string checking. Go 1.25 added `ReadFile`,
  `WriteFile`, `MkdirAll`, `RemoveAll`, `Rename`, `Symlink` and more to it. **Use it whenever a
  path comes from outside your program.**
- **`io/fs`** is the read-only filesystem abstraction: `fs.FS`, `fs.WalkDir`, `fs.Sub`,
  `embed.FS` and `os.DirFS` all satisfy it, which is what makes `//go:embed` and testing with
  `fstest.MapFS` work.
*/

func m013OsAndIo() {
	fmt.Println("\n--- Section 4: os, io, bufio and os.Root ---")

	fmt.Printf("  os.Args[0]=%s\n", os.Args[0])
	if home, ok := os.LookupEnv("HOME"); ok {
		fmt.Printf("  LookupEnv distinguishes unset from empty: HOME is set (%d chars)\n", len(home))
	}
	if _, ok := os.LookupEnv("DEFINITELY_NOT_SET_12345"); !ok {
		fmt.Println("  LookupEnv on an unset variable: ok=false (Getenv would return \"\")")
	}

	// A scratch directory for the rest of this section.
	dir, err := os.MkdirTemp("", "m013-*")
	if err != nil {
		fmt.Println("  could not create a temp dir:", err)
		return
	}
	defer os.RemoveAll(dir)

	// --- The one-line forms ---
	path := filepath.Join(dir, "notes.txt")
	content := "first line\nsecond line\nthird line\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		fmt.Println("  write failed:", err)
		return
	}
	data, _ := os.ReadFile(path)
	fmt.Printf("  os.WriteFile + os.ReadFile: %d bytes\n", len(data))

	info, _ := os.Stat(path)
	fmt.Printf("  os.Stat: name=%s size=%d mode=%v\n", info.Name(), info.Size(), info.Mode())

	// --- Streaming with bufio.Scanner ---
	f, _ := os.Open(path)
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		count++
	}
	_ = f.Close()
	fmt.Printf("  bufio.Scanner read %d lines (err=%v)\n", count, scanner.Err())
	fmt.Println("  Scanner has a 64 KB line limit by default - raise it with scanner.Buffer")

	// --- io composition ---
	var captured bytes.Buffer
	tee := io.TeeReader(strings.NewReader("teed"), &captured)
	consumed, _ := io.ReadAll(tee)
	fmt.Printf("  io.TeeReader: consumed=%q and a copy was captured=%q\n", consumed, captured.String())

	limited, _ := io.ReadAll(io.LimitReader(strings.NewReader("truncate me here"), 8))
	fmt.Printf("  io.LimitReader(8): %q\n", limited)

	var fanout bytes.Buffer
	mw := io.MultiWriter(&fanout, io.Discard)
	fmt.Fprint(mw, "io.MultiWriter fans one write out to several writers")
	fmt.Printf("  io.MultiWriter: %q\n", fanout.String())

	// --- os.Root: path traversal is refused by the OS ---
	root, err := os.OpenRoot(dir)
	if err == nil {
		defer root.Close()
		inside, rerr := root.ReadFile("notes.txt") // Go 1.25 added ReadFile to Root
		fmt.Printf("  os.Root.ReadFile(\"notes.txt\"): %d bytes, err=%v\n", len(inside), rerr)

		_, escapeErr := root.Open("../../../etc/passwd")
		fmt.Printf("  os.Root.Open(\"../../../etc/passwd\") -> %v\n", escapeErr)
		fmt.Println("  the escape is refused by the OS, not by string inspection - use os.Root")
		fmt.Println("  (Go 1.24) whenever a path comes from outside your program")
	}
}

// =================================================================================================
// Section 5: encoding/json — v1
// =================================================================================================

/*
## encoding/json — v1

- `json.Marshal(v)` and `json.Unmarshal(data, &v)` for whole values in memory; `json.NewEncoder(w)`
  and `json.NewDecoder(r)` for streams. `MarshalIndent` for human-readable output.
- **Only exported fields are visible.** Reflection cannot see the rest, and no tag changes that.
- The tag options (module 007, Section 5): rename, `-` to exclude, `omitempty`, `,string`, and
  **`omitzero`** (Go 1.24), which finally omits a zero struct or `time.Time` where `omitempty`
  could not.
- **Unmarshal into `any` gives you a fixed set of dynamic types**: `bool`, **`float64` for every
  number**, `string`, `nil`, `[]any`, `map[string]any`. Decoding `{"id": 1}` and then asserting
  `.(int)` panics — it is a `float64`. Use `json.Number` (via `Decoder.UseNumber`) to keep the exact
  text.
- **Unknown fields are silently ignored** by default. `Decoder.DisallowUnknownFields()` makes them
  an error, which is what you want for configuration files.
- **Missing fields are left at their zero value**, so you cannot tell "absent" from "explicitly
  zero" without a pointer field or a `json.RawMessage`.
- Implement `json.Marshaler`/`Unmarshaler` for full control, or better
  `encoding.TextMarshaler`/`TextUnmarshaler`, which also serves XML and map keys (module 008).
- Known v1 weaknesses, all fixed in v2: it is slow, it cannot stream a large array without manual
  token work, its error messages are poor, case-insensitive field matching is on by default, and
  `omitempty` has odd semantics. Hence Section 6.
*/

type m013User struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitzero"` // Go 1.24: omitted when zero
	Internal  string    `json:"-"`
	password  string
}

func m013JSONv1() {
	fmt.Println("\n--- Section 5: encoding/json — v1 ---")

	u := m013User{ID: 1, Name: "Ada", Internal: "hidden", password: "secret"}
	b, _ := json.Marshal(u)
	fmt.Printf("  Marshal:       %s\n", b)
	fmt.Println("  Email and CreatedAt are absent (omitempty / omitzero); Internal is excluded;")
	fmt.Println("  `password` is invisible because it is unexported")

	indented, _ := json.MarshalIndent(m013User{ID: 2, Name: "Grace",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}, "  ", "  ")
	fmt.Printf("  MarshalIndent:\n  %s\n", indented)

	// --- The float64 trap ---
	var loose any
	_ = json.Unmarshal([]byte(`{"id":1,"ratio":0.5,"tags":["a"],"nested":{"k":true}}`), &loose)
	obj := loose.(map[string]any)
	fmt.Printf("  every JSON number decodes as float64: id is %T\n", obj["id"])
	fmt.Printf("  the full set: %T %T %T %T\n", obj["id"], obj["tags"], obj["nested"], loose)

	// UseNumber keeps the exact text.
	dec := json.NewDecoder(strings.NewReader(`{"big": 123456789012345678901}`))
	dec.UseNumber()
	var exact map[string]any
	_ = dec.Decode(&exact)
	fmt.Printf("  Decoder.UseNumber keeps precision: %v (%T)\n", exact["big"], exact["big"])

	// --- Unknown fields ---
	var target m013User
	strict := json.NewDecoder(strings.NewReader(`{"id":1,"typo":"oops"}`))
	strict.DisallowUnknownFields()
	fmt.Printf("  DisallowUnknownFields: %v\n", strict.Decode(&target))
	fmt.Println("  by default an unknown field is silently ignored - always set this for config")

	// --- Absent versus zero ---
	type patch struct {
		Name *string `json:"name"` // a pointer distinguishes absent (nil) from ""
	}
	var absent, explicit patch
	_ = json.Unmarshal([]byte(`{}`), &absent)
	_ = json.Unmarshal([]byte(`{"name":""}`), &explicit)
	fmt.Printf("  absent name is %v; explicitly empty is %q - a *string tells them apart\n",
		absent.Name, *explicit.Name)

	// --- Streaming ---
	var streamed bytes.Buffer
	enc := json.NewEncoder(&streamed)
	for _, v := range []m013User{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}} {
		_ = enc.Encode(v) // one JSON value per line
	}
	fmt.Printf("  Encoder writes newline-delimited JSON:\n%s",
		m013Indent(streamed.String(), "    "))
}

func m013Indent(s, prefix string) string {
	var sb strings.Builder
	for line := range strings.SplitSeq(strings.TrimRight(s, "\n"), "\n") {
		sb.WriteString(prefix + line + "\n")
	}
	return sb.String()
}

// =================================================================================================
// Section 6: encoding/json/v2 and jsontext (Go 1.27)
// =================================================================================================

/*
## encoding/json/v2 and jsontext (Go 1.27)

Go 1.27 promoted the long-incubating JSON rewrite to the standard library. It is a **new package**,
not a breaking change: `encoding/json` still exists, still behaves exactly as before, and is now
implemented on top of the new machinery.

The split is:

  - **`encoding/json/jsontext`** — the *syntactic* layer. Tokens and values, an `Encoder` and
    `Decoder` that know nothing about Go types, and formatting options (`WithIndent`,
    `EscapeForHTML`, `Multiline`, `SpaceAfterColon`, `Canonicalize*`). Use it to stream, reformat or
    validate JSON without unmarshalling.
  - **`encoding/json/v2`** — the *semantic* layer: `Marshal`/`Unmarshal` between Go values and JSON,
    built on `jsontext`.

What v2 changes:

  - **Options are arguments, not decoder state.** `json.Marshal(v, jsontext.WithIndent("  "))`
    replaces `MarshalIndent`, and every option composes.
  - **Field matching is case-sensitive by default**; v1's case-insensitive matching was a frequent
    source of surprise. `json.MatchCaseInsensitiveNames(true)` restores it.
  - **Unknown fields, duplicate names and invalid UTF-8** are handled explicitly rather than by
    silent defaults.
  - **`omitzero` and `omitempty` have sane, distinct meanings.**
  - **It refuses to guess.** v1 marshalled a `time.Duration` as a bare nanosecond count — an
    undocumented choice that read back as a meaningless integer. v2 reports
    `cannot marshal from Go time.Duration: no default representation` and makes you choose one,
    normally with a small wrapper type implementing `encoding.TextMarshaler`. (The `format:` struct
    tag option that will eventually cover this is not yet enabled in 1.27.)
  - It **streams properly**: you can decode a huge array element by element.
  - It is **substantially faster**, especially at unmarshalling.
  - New method names for custom types: `MarshalJSONTo(*jsontext.Encoder)` and
    `UnmarshalJSONFrom(*jsontext.Decoder)`, which stream instead of allocating a `[]byte`.

`encoding/json` (v1) also gained `DefaultOptionsV1()` and a set of `...WithLegacySemantics` options,
so you can opt into v1 behaviour piece by piece while migrating.

**Which to use**: v1 is not deprecated and is fine for existing code. Reach for v2 in new code that
cares about performance, strictness, or streaming.
*/

type m013Config struct {
	Name     string      `json:"name"`
	Port     int         `json:"port"`
	Timeout  m013Timeout `json:"timeout"`
	Replicas int         `json:"replicas,omitzero"`
}

// m013Timeout wraps time.Duration with a TEXT representation. v2 refuses to guess one, so this
// wrapper is how you carry a duration through json/v2 today.
type m013Timeout time.Duration

func (d m013Timeout) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

func (d *m013Timeout) UnmarshalText(b []byte) error {
	parsed, err := time.ParseDuration(string(b))
	if err != nil {
		return fmt.Errorf("m013Timeout: %w", err)
	}
	*d = m013Timeout(parsed)
	return nil
}

// The v1 equivalent, for the comparison below.
type m013ConfigV1 struct {
	Name    string        `json:"name"`
	Timeout time.Duration `json:"timeout"`
}

func m013JSONv2() {
	fmt.Println("\n--- Section 6: encoding/json/v2 and jsontext (Go 1.27) ---")

	cfg := m013Config{Name: "api", Port: 8080, Timeout: m013Timeout(30 * time.Second)}

	// --- Options are arguments now, not decoder state ---
	plain, _ := jsonv2.Marshal(cfg)
	fmt.Printf("  v2 Marshal:            %s\n", plain)

	indented, _ := jsonv2.Marshal(cfg, jsontext.WithIndent("  "))
	fmt.Printf("  v2 with WithIndent:\n%s", m013Indent(string(indented), "    "))

	spaced, _ := jsonv2.Marshal(cfg, jsontext.SpaceAfterColon(true), jsontext.SpaceAfterComma(true))
	fmt.Printf("  v2 formatting options compose: %s\n", spaced)

	// --- v2 refuses to guess a representation where v1 silently picked one ---
	type bare struct {
		Timeout time.Duration `json:"timeout"`
	}
	_, durErr := jsonv2.Marshal(bare{Timeout: 30 * time.Second})
	fmt.Printf("  v2 on a raw time.Duration: %v\n", durErr)
	v1Bytes, _ := json.Marshal(m013ConfigV1{Name: "api", Timeout: 30 * time.Second})
	fmt.Printf("  v1 silently emitted nanoseconds:  %s\n", v1Bytes)
	fmt.Println("  v1's choice was undocumented and lossy to read back; v2 makes you decide.")
	fmt.Println("  The fix is a wrapper type with MarshalText/UnmarshalText - m013Timeout above.")

	var roundTrip m013Config
	_ = jsonv2.Unmarshal([]byte(`{"name":"api","port":1,"timeout":"1m30s"}`), &roundTrip)
	fmt.Printf("  and it round-trips as readable text: %v\n", time.Duration(roundTrip.Timeout))

	// --- Case sensitivity ---
	var strictTarget m013Config
	strictErr := jsonv2.Unmarshal([]byte(`{"NAME":"x","port":1}`), &strictTarget)
	fmt.Printf("  v2 is case-SENSITIVE by default: name=%q (err=%v)\n", strictTarget.Name, strictErr)

	var looseTarget m013Config
	_ = jsonv2.Unmarshal([]byte(`{"NAME":"x","port":1}`), &looseTarget,
		jsonv2.MatchCaseInsensitiveNames(true))
	fmt.Printf("  with MatchCaseInsensitiveNames(true): name=%q\n", looseTarget.Name)

	var v1Target m013ConfigV1
	_ = json.Unmarshal([]byte(`{"NAME":"x"}`), &v1Target)
	fmt.Printf("  v1 was case-INsensitive, which surprised people: name=%q\n", v1Target.Name)

	// --- Duplicate names are rejected by default in v2 ---
	var dup m013Config
	dupErr := jsonv2.Unmarshal([]byte(`{"port":1,"port":2}`), &dup)
	fmt.Printf("  v2 rejects duplicate object names: %v\n", dupErr != nil)
	var dupV1 m013ConfigV1
	fmt.Printf("  v1 accepted them, last one winning:  %v\n",
		json.Unmarshal([]byte(`{"name":"a","name":"b"}`), &dupV1) == nil)

	// --- jsontext: syntax without semantics ---
	value := jsontext.Value(`{"b":2,"a":{"d":4,"c":3}}`)
	_ = value.Canonicalize() // sort object names, compact
	fmt.Printf("  jsontext.Value.Canonicalize: %s\n", value)

	pretty := jsontext.Value(`{"a":[1,2],"b":null}`)
	_ = pretty.Indent(jsontext.WithIndent("  "))
	fmt.Printf("  jsontext.Value.Indent:\n%s", m013Indent(string(pretty), "    "))

	// Streaming at the token level, with no Go type at all.
	dec := jsontext.NewDecoder(strings.NewReader(`{"id":1,"tags":["a","b"]}`))
	var kinds []string
	for {
		tok, err := dec.ReadToken()
		if err != nil {
			break
		}
		kinds = append(kinds, tok.Kind().String())
	}
	fmt.Printf("  jsontext token stream: %v\n", strings.Join(kinds, " "))
	fmt.Println("  this is how you stream a huge array without holding it all in memory")

	fmt.Println("  v1 is not deprecated; reach for v2 for speed, strictness or streaming")
}

// =================================================================================================
// Section 7: log/slog
// =================================================================================================

/*
## log/slog

- **`log/slog`** (Go 1.21) is the standard structured logger. The old `log` package still exists and
  is fine for a CLI, but anything that ships logs to a system should use `slog`.
- Levels are `Debug`, `Info`, `Warn`, `Error`, as `slog.Level` values (`Debug=-4`, `Info=0`,
  `Warn=4`, `Error=8`), so custom levels fit in between.
- Two handlers ship with it: **`slog.NewTextHandler`** (`key=value`, for humans) and
  **`slog.NewJSONHandler`** (for log aggregation). `HandlerOptions` sets the minimum level,
  `AddSource`, and a `ReplaceAttr` hook — which is how you redact fields or rename keys.
- Attributes come in two forms: the **variadic key/value** form `slog.Info("msg", "key", val)`,
  which is convenient but unchecked, and the **typed** form `slog.String`, `slog.Int`,
  `slog.Duration`, `slog.Any`, which is faster and cannot get out of step. `go vet` checks the
  former for odd argument counts.
- **`logger.With(...)`** returns a logger carrying those attributes on every record — the right way
  to attach a request ID. **`WithGroup`** nests subsequent attributes under a name.
- **`slog.GroupAttrs`** (Go 1.25) builds a group from a slice of `Attr`, which the older
  `slog.Group` could not do ergonomically.
- **`slog.NewMultiHandler`** (**Go 1.26**) fans one record out to several handlers — human-readable
  text to the console and JSON to a file, from one logger, with no third-party package.
- Use the **`Context` variants** (`InfoContext`, `ErrorContext`) so a handler can pull trace IDs out
  of the context.
*/

func m013Slog() {
	fmt.Println("\n--- Section 7: log/slog ---")

	// A text handler, with the time removed so the output is reproducible here.
	stripTime := func(groups []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return a
	}
	text := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug, ReplaceAttr: stripTime,
	}))

	fmt.Println("  TextHandler:")
	text.Info("server started", "port", 8080, "tls", false)
	text.Warn("high latency", slog.Duration("p99", 250*time.Millisecond))
	text.Error("request failed", slog.String("path", "/api"), slog.Int("status", 500))

	// The typed attribute constructors are faster and cannot get out of step.
	fmt.Println("  typed attributes are preferred over the loose key/value form")

	// --- With and WithGroup ---
	fmt.Println("  With() attaches attributes to every later record:")
	reqLogger := text.With(slog.String("requestID", "req-42"), slog.String("user", "ada"))
	reqLogger.Info("handling request")
	reqLogger.Info("request complete", slog.Int("ms", 12))

	fmt.Println("  WithGroup nests them:")
	text.WithGroup("db").Info("query", slog.String("table", "users"), slog.Int("rows", 3))

	// slog.GroupAttrs (Go 1.25): build a group from a slice.
	attrs := []slog.Attr{slog.String("region", "eu"), slog.Int("shard", 2)}
	// context.Background(), not nil: module 011 states the rule - never pass a nil Context.
	text.LogAttrs(context.Background(), slog.LevelInfo, "placement", slog.GroupAttrs("location", attrs...))
	fmt.Println("  ^ slog.GroupAttrs (Go 1.25) builds a group from a []slog.Attr")

	// --- JSON handler ---
	fmt.Println("  JSONHandler, for log aggregation:")
	jsonLogger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{ReplaceAttr: stripTime}))
	jsonLogger.Info("structured", slog.String("env", "prod"), slog.Int("version", 27))

	// --- MultiHandler (Go 1.26): one logger, several destinations ---
	var captured bytes.Buffer
	multi := slog.New(slog.NewMultiHandler(
		slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{ReplaceAttr: stripTime}),
		slog.NewJSONHandler(&captured, &slog.HandlerOptions{ReplaceAttr: stripTime}),
	))
	fmt.Println("  MultiHandler (Go 1.26) writes text here AND JSON to a buffer:")
	multi.Info("fanned out", slog.String("to", "two handlers"))
	fmt.Printf("  the buffer received: %s", captured.String())
}

// =================================================================================================
// Section 8: regexp and uuid
// =================================================================================================

/*
## regexp and uuid

### regexp

- `regexp` implements **RE2**, which guarantees linear time and therefore **has no backtracking**:
  no backreferences, no lookahead, no lookbehind. In exchange it cannot suffer catastrophic
  backtracking — a whole class of denial-of-service bug simply does not exist.
- Two constructors: `Compile` returns `(*Regexp, error)`; **`MustCompile` panics** and is the right
  choice for a package-level pattern, which is a compile-time constant in practice.
- **Compile once**, at package level. Compiling inside a loop is a common and expensive mistake.
- The method names are systematic: `Find` / `FindAll` / `FindString` / `FindStringSubmatch` /
  `FindAllStringSubmatch`, plus `MatchString`, `ReplaceAllString`, `ReplaceAllStringFunc`, `Split`.
  `Submatch` means capture groups; `Index` means byte offsets; `All` takes an `n` limit (`-1` for
  all).
- **Named groups** `(?P<name>...)` plus `SubexpNames` make a match self-documenting.
- `*Regexp` is safe for concurrent use.
- For a fixed substring, `strings.Contains` is far faster. Do not reach for a regexp by reflex.

### uuid (Go 1.27)

New in the standard library, implementing **RFC 9562**. Before 1.27 every Go program needing a UUID
pulled in `github.com/google/uuid`.

  - `uuid.New()` — the general-purpose choice
  - `uuid.NewV4()` — 122 random bits
  - **`uuid.NewV7()`** — time-ordered: the first 48 bits are a millisecond timestamp, so UUIDs sort
    chronologically. **This is what you want for a database primary key**, because random v4 keys
    scatter B-tree inserts and destroy index locality.
  - `uuid.Parse` / `MustParse`, `Compare`, `Nil()`, `Max()`, and `MarshalText`/`UnmarshalText` — so
    a `UUID` works in JSON and as a map key with no extra code.
  - Random bits come from a **cryptographically secure** source.
*/

// Compiled once, at package level - never inside a function that runs repeatedly.
var (
	m013EmailRe = regexp.MustCompile(`^[\w.+-]+@[\w-]+\.[\w.]+$`)
	m013LogRe   = regexp.MustCompile(`^(?P<level>\w+)\s+(?P<time>[\d:]+)\s+(?P<msg>.*)$`)
)

func m013RegexpAndUUID() {
	fmt.Println("\n--- Section 8: regexp and uuid ---")

	for _, e := range []string{"ada@example.com", "not-an-email", "a.b+c@sub.example.co.uk"} {
		fmt.Printf("  MatchString(%-24q) = %t\n", e, m013EmailRe.MatchString(e))
	}

	// Named capture groups.
	line := "ERROR 14:05:06 database connection lost"
	if m := m013LogRe.FindStringSubmatch(line); m != nil {
		fmt.Println("  named groups:")
		for i, name := range m013LogRe.SubexpNames() {
			if i > 0 && name != "" {
				fmt.Printf("    %-6s = %q\n", name, m[i])
			}
		}
	}

	// FindAll with a limit, and replacement.
	words := regexp.MustCompile(`\w+`)
	fmt.Printf("  FindAllString(-1): %v\n", words.FindAllString("one two three", -1))
	fmt.Printf("  FindAllString(2):  %v\n", words.FindAllString("one two three", 2))
	fmt.Printf("  ReplaceAllString:  %q\n",
		regexp.MustCompile(`\d+`).ReplaceAllString("a1b22c333", "#"))
	fmt.Printf("  ReplaceAllStringFunc: %q\n",
		words.ReplaceAllStringFunc("go is fun", strings.ToUpper))

	// RE2 has no backreferences or lookahead - by design.
	_, err := regexp.Compile(`(a)\1`)
	fmt.Printf("  backreferences are not supported (RE2): %v\n", err)
	fmt.Println("  in exchange, matching is linear time - no catastrophic backtracking, ever")
	fmt.Println("  for a fixed substring, strings.Contains is much faster than a regexp")

	// --- uuid (Go 1.27) ---
	v4 := uuid.NewV4()
	fmt.Printf("  uuid.NewV4(): %v (122 random bits)\n", v4)

	// v7 values are time-ordered, so they sort chronologically.
	var seq []uuid.UUID
	for range 3 {
		seq = append(seq, uuid.NewV7())
		time.Sleep(2 * time.Millisecond)
	}
	fmt.Println("  uuid.NewV7() is time-ordered, so generated values already sort:")
	for i, id := range seq {
		ordered := i == 0 || id.Compare(seq[i-1]) > 0
		fmt.Printf("    %v  greaterThanPrevious=%t\n", id, ordered)
	}
	fmt.Println("  use v7 for database keys: v4 scatters B-tree inserts and ruins index locality")

	parsed, err := uuid.Parse(v4.String())
	fmt.Printf("  Parse round trip: equal=%t err=%v\n", parsed == v4, err)
	fmt.Printf("  Nil()=%v\n", uuid.Nil())

	// It marshals as text, so JSON and map keys work with no extra code.
	encoded, _ := json.Marshal(map[string]uuid.UUID{"id": parsed})
	fmt.Printf("  JSON via MarshalText: %s\n", encoded)
}

// Run013 runs every section of module 013 in order.
func Run013() {
	m013Fmt()
	m013StringsAndBytes()
	m013Time()
	m013OsAndIo()
	m013JSONv1()
	m013JSONv2()
	m013Slog()
	m013RegexpAndUUID()
}

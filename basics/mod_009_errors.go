package basics

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
)

/*
# Module 009 — Errors

Go has **no exceptions**. An error is an ordinary value of the interface type `error`, returned
alongside the result, and checked with an `if`. The whole mechanism is:

	type error interface {
	    Error() string
	}

The trade is explicitness for verbosity. Every call that can fail visibly can fail, and the control
flow is on the page rather than in an invisible unwinding path. The cost is `if err != nil` — the
most-typed three words in Go, and the most common complaint about the language.

`panic` exists, but it is for programmer bugs, not for expected failures (module 005, Section 6).
*/

// =================================================================================================
// Section 1: Errors Are Values
// =================================================================================================

/*
## Errors Are Values

- `error` is a plain interface with one method. Anything with `Error() string` is an error — your
  own struct, a defined string type, anything.
- The convention is that the error is the **last** return value, and that a non-nil error means the
  other results are **not meaningful** (with documented exceptions such as `io.Reader`, which may
  return `n > 0` together with an error).
- Create simple errors with `errors.New("...")` or `fmt.Errorf("...: %v", x)`. Both give you an
  opaque value whose only guaranteed capability is its message.
- **Error message conventions**, from the Go style guide and enforced socially rather than by tools:
    - lower case, **no trailing punctuation**, **no capital first letter** — because errors are
      routinely wrapped and concatenated: `failed to open config: open /etc/app.conf: no such file`
    - no "error" or "failed to" prefix on the innermost message; the context comes from wrapping
    - include the **values** that made it fail (the path, the key, the index), never just "invalid
      input"
- Do **not** ignore errors silently. `_ = f()` is a deliberate statement that you have thought about
  it; leaving an error unchecked entirely is what `errcheck` and many CI setups reject.
- `errors.New` returns a **pointer** internally, so two `errors.New("x")` values are never equal.
  That is deliberate — it is what makes sentinel comparison meaningful (Section 2).
*/

// A custom error type: any type with an Error() string method.
type m009ValidationError struct {
	Field string
	Value any
}

func (e m009ValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %v", e.Field, e.Value)
}

// Even a defined string type can be an error - and this one can be a CONSTANT,
// which errors.New cannot.
type m009ConstError string

func (e m009ConstError) Error() string { return string(e) }

const m009ErrClosed = m009ConstError("connection closed")

func m009ErrorsAreValues() {
	fmt.Println("--- Section 1: Errors Are Values ---")

	// The canonical shape.
	if v, err := m009ParsePort("8080"); err == nil {
		fmt.Printf("  m009ParsePort(\"8080\") = %d\n", v)
	}
	if _, err := m009ParsePort("http"); err != nil {
		fmt.Printf("  m009ParsePort(\"http\") -> %v\n", err)
	}
	if _, err := m009ParsePort("99999"); err != nil {
		fmt.Printf("  m009ParsePort(\"99999\") -> %v\n", err)
	}

	// A custom error type is just a value.
	var err error = m009ValidationError{Field: "age", Value: -1}
	fmt.Printf("  a struct as an error: %v (%T)\n", err, err)

	// A constant error - impossible with errors.New, which is a function call.
	fmt.Printf("  a CONSTANT error: %v (%T)\n", m009ErrClosed, m009ErrClosed)

	// errors.New values are never equal, even with identical text.
	a, b := errors.New("same text"), errors.New("same text")
	fmt.Printf("  errors.New(\"x\") == errors.New(\"x\"): %t  <- identity, not text\n", a == b)

	// --- Message conventions ---
	fmt.Println("  message style: lower case, no trailing punctuation, include the values")
	fmt.Printf("    good: %v\n", fmt.Errorf("parse port %q: value out of range", "99999"))
	fmt.Println("    bad:  \"Error: Failed to parse the port!\"")
}

func m009ParsePort(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		// Wrap so the caller can still reach the strconv error - see Section 3.
		return 0, fmt.Errorf("parse port %q: %w", s, err)
	}
	if n < 1 || n > 65535 {
		return 0, fmt.Errorf("parse port %q: out of range 1..65535", s)
	}
	return n, nil
}

// =================================================================================================
// Section 2: Sentinel Errors
// =================================================================================================

/*
## Sentinel Errors

- A **sentinel** is a package-level error value that callers compare against, so they can react to a
  specific condition: `io.EOF`, `sql.ErrNoRows`, `os.ErrNotExist`, `context.Canceled`.
- Convention: the name starts with **`Err`**, it is a `var` at package level, and it is created with
  `errors.New`. Exported when callers need to branch on it; unexported when they do not.
- **Compare with `errors.Is`, not `==`.** `==` only works when the error was returned unwrapped;
  `errors.Is` walks the wrap chain (Section 3). Since almost every real error gets wrapped
  somewhere, `==` is a latent bug.
- A sentinel becomes **part of your API**. Once callers branch on it you cannot remove it or change
  when it is returned without breaking them. Add sentinels sparingly, and only for conditions a
  caller can genuinely act on differently.
- **Alternatives**, in rough order of preference:
    1. no sentinel at all — the caller just reports the error
    2. a **behaviour** the caller can test for: an interface such as `interface{ Timeout() bool }`,
       which `net.Error` uses
    3. a **custom error type** plus `errors.As`, when the caller needs the *details* and not just
       the fact (Section 4)
    4. a sentinel, when the condition is a simple binary fact with no payload
- `errors.ErrUnsupported` (Go 1.21) is a standard sentinel for "this operation is not supported
  here"; wrap it rather than inventing your own.
*/

// The conventional shape: package-level, Err-prefixed, created with errors.New.
var (
	m009ErrNotFound     = errors.New("not found")
	m009ErrPermission   = errors.New("permission denied")
	m009ErrAlreadyExist = errors.New("already exists")
)

type m009Store struct{ data map[string]string }

func (s *m009Store) Get(key string) (string, error) {
	v, ok := s.data[key]
	if !ok {
		// Wrapped, so the caller gets both the context and the sentinel.
		return "", fmt.Errorf("store get %q: %w", key, m009ErrNotFound)
	}
	return v, nil
}

func (s *m009Store) Put(key, value string) error {
	if _, exists := s.data[key]; exists {
		return fmt.Errorf("store put %q: %w", key, m009ErrAlreadyExist)
	}
	if strings.HasPrefix(key, "_") {
		return fmt.Errorf("store put %q: %w", key, m009ErrPermission)
	}
	s.data[key] = value
	return nil
}

func m009SentinelErrors() {
	fmt.Println("\n--- Section 2: Sentinel Errors ---")

	store := &m009Store{data: map[string]string{"a": "1"}}

	// Branching on a sentinel with errors.Is.
	for _, key := range []string{"a", "missing"} {
		v, err := store.Get(key)
		switch {
		case err == nil:
			fmt.Printf("  Get(%q) = %q\n", key, v)
		case errors.Is(err, m009ErrNotFound):
			fmt.Printf("  Get(%q): not found, using a default (full error: %v)\n", key, err)
		default:
			fmt.Printf("  Get(%q): unexpected: %v\n", key, err)
		}
	}

	for _, key := range []string{"b", "a", "_private"} {
		err := store.Put(key, "x")
		fmt.Printf("  Put(%q): %v  [notFound=%t exists=%t permission=%t]\n", key, err,
			errors.Is(err, m009ErrNotFound),
			errors.Is(err, m009ErrAlreadyExist),
			errors.Is(err, m009ErrPermission))
	}

	// --- Why == is a bug ---
	_, wrapped := store.Get("missing")
	fmt.Printf("  wrapped == m009ErrNotFound:            %t  <- the wrapping broke it\n",
		wrapped == m009ErrNotFound)
	fmt.Printf("  errors.Is(wrapped, m009ErrNotFound):   %t  <- always use this\n",
		errors.Is(wrapped, m009ErrNotFound))

	// --- Standard library sentinels ---
	_, err := os.Open("/definitely/not/here")
	fmt.Printf("  os.Open on a missing path: errors.Is(err, fs.ErrNotExist) = %t\n",
		errors.Is(err, fs.ErrNotExist))
	fmt.Println("  familiar sentinels: io.EOF, sql.ErrNoRows, os.ErrNotExist, context.Canceled,")
	fmt.Println("  context.DeadlineExceeded, errors.ErrUnsupported (Go 1.21)")
}

// =================================================================================================
// Section 3: Wrapping — %w, Unwrap, Is and Join
// =================================================================================================

/*
## Wrapping — %w, Unwrap, Is and Join

- `fmt.Errorf` with the **`%w` verb** produces an error that *wraps* the original, adding context
  while keeping the original reachable. `%v` formats the original into the message and **throws it
  away** — the distinction is the whole point.
- The wrap chain is walked by:
    - **`errors.Is(err, target)`** — is `target` anywhere in the chain? Use it for sentinels.
    - **`errors.As(err, &target)`** — is there an error of `target`'s *type* in the chain? If so it
      is assigned to `target`. Use it for custom types with payloads (Section 4).
    - **`errors.Unwrap(err)`** — one step down the chain. You rarely call it directly.
- `fmt.Errorf` accepts **several `%w` verbs** (Go 1.20), producing an error that wraps a tree.
- **`errors.Join(errs...)`** (Go 1.20) combines several errors into one whose message is each on its
  own line, and which `errors.Is`/`As` search across. Nil arguments are dropped, and joining nothing
  returns nil — so it composes cleanly in a validation loop.
- A type controls its own unwrapping by implementing `Unwrap() error` (one child) or
  `Unwrap() []error` (several). Implementing `Is(error) bool` lets a type declare its own notion of
  matching — useful when several distinct values should all count as the same condition.
- **Wrapping is an API decision.** A wrapped error exposes the inner error to callers, who may then
  depend on it. Wrap with `%w` when the caller should be able to inspect the cause; use `%v` to
  deliberately hide an implementation detail behind your own error.
- Add context **at each layer**, and do not repeat what the caller already knows. The result should
  read as a path: `load config: read /etc/app.conf: open /etc/app.conf: no such file or directory`.
*/

func m009Wrapping() {
	fmt.Println("\n--- Section 3: Wrapping — %w, Unwrap, Is and Join ---")

	// A three-layer wrap, reading as a path from outermost to innermost.
	err := m009LoadConfig("/etc/nonexistent.conf")
	fmt.Printf("  %v\n", err)

	// Walking the chain by hand.
	fmt.Println("  unwrapping step by step:")
	for e := err; e != nil; e = errors.Unwrap(e) {
		fmt.Printf("    %-24T %v\n", e, e)
	}

	// errors.Is finds the sentinel at the bottom.
	fmt.Printf("  errors.Is(err, fs.ErrNotExist) = %t\n", errors.Is(err, fs.ErrNotExist))

	// --- %w versus %v ---
	inner := errors.New("the cause")
	wrapped := fmt.Errorf("context: %w", inner)
	flattened := fmt.Errorf("context: %v", inner)
	fmt.Printf("  %%w: %v  -> errors.Is finds the cause: %t\n", wrapped, errors.Is(wrapped, inner))
	fmt.Printf("  %%v: %v  -> errors.Is finds the cause: %t  <- the cause was discarded\n",
		flattened, errors.Is(flattened, inner))

	// --- Several %w verbs (Go 1.20) ---
	multi := fmt.Errorf("two causes: %w and %w", m009ErrNotFound, m009ErrPermission)
	fmt.Printf("  two %%w verbs: %v\n", multi)
	fmt.Printf("    Is(notFound)=%t Is(permission)=%t\n",
		errors.Is(multi, m009ErrNotFound), errors.Is(multi, m009ErrPermission))

	// --- errors.Join (Go 1.20) for validation ---
	fmt.Println("  errors.Join collecting every validation failure at once:")
	if err := m009ValidateUser("", -5, "not-an-email"); err != nil {
		for _, line := range strings.Split(err.Error(), "\n") {
			fmt.Printf("    %s\n", line)
		}
		fmt.Printf("    Is(m009ErrInvalidAge)=%t (Join is searched too)\n",
			errors.Is(err, m009ErrInvalidAge))
	}
	fmt.Printf("  joining nothing returns nil: %v\n", errors.Join(nil, nil) == nil)

	// --- A type controlling its own matching ---
	notFound := m009HTTPError{Code: 404}
	fmt.Printf("  a custom Is method: m009HTTPError{404} matches m009ErrNotFound: %t\n",
		errors.Is(notFound, m009ErrNotFound))
}

func m009LoadConfig(path string) error {
	if err := m009ReadFile(path); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	return nil
}

func m009ReadFile(path string) error {
	_, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

var (
	m009ErrInvalidName  = errors.New("name must not be empty")
	m009ErrInvalidAge   = errors.New("age must be non-negative")
	m009ErrInvalidEmail = errors.New("email must contain @")
)

// m009ValidateUser reports EVERY problem at once rather than only the first.
func m009ValidateUser(name string, age int, email string) error {
	var problems []error
	if name == "" {
		problems = append(problems, m009ErrInvalidName)
	}
	if age < 0 {
		problems = append(problems, fmt.Errorf("%w: got %d", m009ErrInvalidAge, age))
	}
	if !strings.Contains(email, "@") {
		problems = append(problems, fmt.Errorf("%w: got %q", m009ErrInvalidEmail, email))
	}
	return errors.Join(problems...) // nil when problems is empty
}

// m009HTTPError implements Is, so it can declare its own notion of matching.
type m009HTTPError struct{ Code int }

func (e m009HTTPError) Error() string { return fmt.Sprintf("http status %d", e.Code) }

func (e m009HTTPError) Is(target error) bool {
	return e.Code == 404 && target == m009ErrNotFound
}

// =================================================================================================
// Section 4: Custom Error Types, errors.As and errors.AsType
// =================================================================================================

/*
## Custom Error Types, errors.As and errors.AsType

- When the caller needs **details** rather than just the fact of failure, define a type: a status
  code, a field name, a retry-after duration, a line number.
- Retrieve it with **`errors.As(err, &target)`**, which walks the chain looking for an error
  assignable to `*target`, assigns it, and reports `true`. Note that the second argument must be a
  **pointer to** the type you want.
- **Go 1.26 added `errors.AsType[T](err) (T, bool)`** — the generic version. It needs no
  pre-declared variable and no `&`, and the type parameter makes the intent obvious:

	if ve, ok := errors.AsType[*ValidationError](err); ok { ... }

  It is the form to prefer in new code. `errors.As` remains for pre-1.26 compatibility.
- **Pointer or value receiver?** If `Error()` has a pointer receiver, then only `*T` is an error, and
  you must search for `*T`. Mixing the two leads to `errors.As` mysteriously not matching. Pick one
  — pointer receivers are the more common choice — and be consistent.
- `errors.As` **panics** if the target is not a pointer to a type implementing `error`. That is a
  programming error, and the panic is deliberate.
- Do not over-engineer: most errors need no type at all. Add one when a caller will genuinely branch
  on the payload, not because it feels more structured.
*/

type m009ParseError struct {
	Line, Column int
	Token        string
	Err          error // the wrapped cause
}

func (e *m009ParseError) Error() string {
	return fmt.Sprintf("parse error at %d:%d near %q: %v", e.Line, e.Column, e.Token, e.Err)
}

// Unwrap makes the wrapped cause reachable by errors.Is/As.
func (e *m009ParseError) Unwrap() error { return e.Err }

type m009RetryableError struct {
	AfterSeconds int
	Err          error
}

func (e *m009RetryableError) Error() string {
	return fmt.Sprintf("temporary failure, retry after %ds: %v", e.AfterSeconds, e.Err)
}
func (e *m009RetryableError) Unwrap() error { return e.Err }

func m009CustomTypes() {
	fmt.Println("\n--- Section 4: Custom Error Types, errors.As and errors.AsType ---")

	err := fmt.Errorf("compiling template: %w", &m009ParseError{
		Line: 12, Column: 5, Token: "{{", Err: m009ErrNotFound,
	})
	fmt.Printf("  %v\n", err)

	// --- errors.As: the classic form ---
	var parseErr *m009ParseError
	if errors.As(err, &parseErr) {
		fmt.Printf("  errors.As found the details: line=%d column=%d token=%q\n",
			parseErr.Line, parseErr.Column, parseErr.Token)
	}

	// --- errors.AsType: the Go 1.26 generic form ---
	if pe, ok := errors.AsType[*m009ParseError](err); ok {
		fmt.Printf("  errors.AsType[*m009ParseError] (Go 1.26): line=%d, no &target needed\n", pe.Line)
	}
	if _, ok := errors.AsType[*m009RetryableError](err); !ok {
		fmt.Println("  errors.AsType[*m009RetryableError] correctly reports no match")
	}

	// Is and As compose: the chain is searched by both.
	fmt.Printf("  errors.Is(err, m009ErrNotFound) through the custom type's Unwrap: %t\n",
		errors.Is(err, m009ErrNotFound))

	// --- A realistic dispatch on error kind ---
	for _, e := range []error{
		&m009RetryableError{AfterSeconds: 30, Err: errors.New("upstream busy")},
		&m009ParseError{Line: 1, Column: 1, Token: "x", Err: errors.New("unexpected")},
		m009ValidationError{Field: "email", Value: "no-at-sign"},
		fmt.Errorf("wrapped: %w", m009ErrPermission),
	} {
		fmt.Printf("  %s\n", m009Handle(e))
	}

	// --- Pointer versus value receivers ---
	var valueErr m009ValidationError
	valueWrapped := fmt.Errorf("outer: %w", m009ValidationError{Field: "age"})
	fmt.Printf("  a VALUE-receiver error type is searched for as the value type: %t\n",
		errors.As(valueWrapped, &valueErr))
	fmt.Println("  if Error() had a pointer receiver you would search for *m009ValidationError")
}

// m009Handle shows the idiomatic dispatch: AsType for payloads, Is for sentinels, default last.
func m009Handle(err error) string {
	if re, ok := errors.AsType[*m009RetryableError](err); ok {
		return fmt.Sprintf("retryable  -> scheduling a retry in %ds", re.AfterSeconds)
	}
	if pe, ok := errors.AsType[*m009ParseError](err); ok {
		return fmt.Sprintf("parse      -> reporting position %d:%d to the user", pe.Line, pe.Column)
	}
	if ve, ok := errors.AsType[m009ValidationError](err); ok {
		return fmt.Sprintf("validation -> highlighting the %q field", ve.Field)
	}
	if errors.Is(err, m009ErrPermission) {
		return "permission -> returning 403"
	}
	return fmt.Sprintf("unknown    -> logging and returning 500 (%v)", err)
}

// =================================================================================================
// Section 5: Error Handling in Practice
// =================================================================================================

/*
## Error Handling in Practice

- **Handle an error exactly once.** Either log it or return it, never both — double handling is how
  one failure becomes six log lines. The rule of thumb: libraries return, the top of the program
  logs.
- **Add context as it goes up**, with `%w`, and make each layer's message say what *that* layer was
  trying to do. The final message should be a readable path.
- **Do not add context that the caller already has.** If the caller passed the path, repeating the
  path in every layer just makes the message noisy.
- Prefer the **early return**: check the error, return, and leave the happy path unindented
  (module 004, Section 1).
- For an operation that can partly succeed, return **both** the partial result and the error, and
  document it. `io.Reader` does exactly this.
- **Deferred cleanup that can fail** should assign to a named error result, so the failure is not
  lost (module 004, Section 6).
- **Do not ignore an error silently.** `_ = f()` says you decided; omitting the check says nothing.
- `panic`/`recover` is not error handling. Recover only at a process boundary — a request handler,
  a worker loop — and convert to an error there (module 005, Section 6).
- A **known Go weakness**: `if err != nil { return nil, err }` is repetitive, and several proposals
  to shorten it (`check`/`handle`, `try`, `?`) have all been rejected. The current position is that
  the verbosity is the price of explicitness. Live with it; do not invent a framework to hide it.
*/

func m009InPractice() {
	fmt.Println("\n--- Section 5: Error Handling in Practice ---")

	// A layered operation, showing the message reading as a path.
	if err := m009ProcessOrder("ORD-1", "abc"); err != nil {
		fmt.Printf("  %v\n", err)
		fmt.Printf("    caller can still branch: Is(m009ErrNotFound)=%t\n",
			errors.Is(err, m009ErrNotFound))
	}

	// Partial success: both a result and an error.
	parsed, err := m009ParseAll([]string{"1", "2", "x", "4"})
	fmt.Printf("  partial success: parsed=%v err=%v\n", parsed, err)

	// Deferred cleanup that can fail, reported through a named result.
	fmt.Printf("  cleanup error surfaced through a named result: %v\n", m009WriteReport())

	fmt.Println("  handle once: libraries return, the top of the program logs")
	fmt.Println("  `if err != nil { return nil, err }` is verbose on purpose - the check/try")
	fmt.Println("  proposals were all rejected, and hiding it behind a framework makes it worse")
}

func m009ProcessOrder(id, qty string) error {
	n, err := strconv.Atoi(qty)
	if err != nil {
		return fmt.Errorf("process order %s: parse quantity: %w", id, err)
	}
	if n <= 0 {
		return fmt.Errorf("process order %s: quantity must be positive, got %d", id, n)
	}
	return nil
}

// m009ParseAll returns everything it managed to parse AND the failure - a documented partial result.
func m009ParseAll(inputs []string) ([]int, error) {
	out := make([]int, 0, len(inputs))
	for i, in := range inputs {
		n, err := strconv.Atoi(in)
		if err != nil {
			return out, fmt.Errorf("element %d: %w", i, err)
		}
		out = append(out, n)
	}
	return out, nil
}

// m009WriteReport shows a deferred Close whose error is not discarded.
func m009WriteReport() (err error) {
	f, err := os.CreateTemp("", "m009-report-*")
	if err != nil {
		return fmt.Errorf("create report: %w", err)
	}
	name := f.Name()
	defer func() {
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close report: %w", cerr)
		}
		_ = os.Remove(name)
		// Closing twice fails, which is how this demo produces a visible cleanup error.
		if cerr := f.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("close report (second attempt, to show the mechanism): %w", cerr)
		}
	}()
	if _, err := f.WriteString("report body\n"); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

// Run009 runs every section of module 009 in order.
func Run009() {
	m009ErrorsAreValues()
	m009SentinelErrors()
	m009Wrapping()
	m009CustomTypes()
	m009InPractice()
}

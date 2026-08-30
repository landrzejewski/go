package basics

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

/*
# Module 017 — Command-Line Applications

This module opens the last block of the course: **building an application**, rather than learning a
language feature. Modules 001–016 taught Go; 017–021 use it to build something that starts, reads
its configuration, serves requests, talks to a database and eventually ships.

A Go program's entry point is `func main()`, and everything an operator gives it arrives through
exactly three channels: **command-line arguments**, **environment variables**, and **files**. This
module covers the first two and how to combine them into one configuration.

A note on the demos: every example here parses an **explicit argument slice** rather than
`os.Args`, because `os.Args` for this process is `[go-training 017]` — the course dispatcher's own
arguments. That is not a workaround, it is the technique: a `main` that takes its arguments as a
parameter is a `main` you can test (Section 7).
*/

// =================================================================================================
// Section 1: os.Args and the flag Package
// =================================================================================================

/*
## os.Args and the flag Package

- **`os.Args`** is a `[]string` of the raw arguments. `os.Args[0]` is the program path (as invoked,
  not necessarily absolute), so the actual arguments are `os.Args[1:]`.
- The standard library's **`flag`** package handles the parsing. It is deliberately small: it does
  **not** implement GNU-style long options. Its conventions are:
    - **one dash for every flag**, whatever its length: `-v` and `-verbose` both work, and `--v` is
      accepted as a synonym. There is no distinction between short and long forms, and **no
      combining**: `-abc` is one flag named `abc`, not `-a -b -c`.
    - `-flag value`, `-flag=value` and `-flag` (for booleans only) are the accepted spellings.
      A **boolean flag cannot** take a separate value: `-v true` sets `v` and leaves `true` as a
      positional argument.
    - parsing **stops at the first non-flag argument**, or at a bare `--`. Everything after that is
      positional and available from `Args()`. This is why `mytool -v build -x` treats `-x` as an
      argument to nothing — it comes after `build`.
- Each flag is declared before parsing, in one of two styles:
    - `v := flag.Bool("v", false, "usage")` — returns a **pointer**, valid after `Parse`
    - `flag.BoolVar(&v, "v", false, "usage")` — binds to a variable you already have
  The `Var` form is usually nicer: it fills a config struct directly.
- Built-in types: `Bool`, `String`, `Int`, `Int64`, `Uint`, `Uint64`, `Float64`, `Duration`,
  `TextVar` (Go 1.19, for anything implementing `encoding.TextUnmarshaler`), and `Func`.
- **`-h` / `-help` is automatic.** If you do not define it, `flag` prints the usage message and
  exits with status **2**. Every flag's default and description come from the declaration, so the
  help text cannot drift from the code.
- **When to reach for a third-party CLI library** (`spf13/cobra`, `urfave/cli`, `alecthomas/kong`):
  when you need nested subcommands with their own help, shell completion, or GNU-style options.
  For a tool with a handful of flags, `flag` is enough and costs nothing.
*/

// m017ServerFlags is the config a flag set fills in. Binding straight into a struct with the `Var`
// forms keeps the flag declarations and the config definition in one place.
type m017ServerFlags struct {
	Host    string
	Port    int
	Verbose bool
	Timeout time.Duration
	Tags    m017StringList // a custom flag.Value — see Section 2
}

func m017OsArgsAndFlag() {
	fmt.Println("--- Section 1: os.Args and the flag Package ---")

	// os.Args for THIS process: the dispatcher's own arguments.
	fmt.Printf("  os.Args = %q\n", os.Args)
	fmt.Println("  os.Args[0] is the program path; the real arguments are os.Args[1:]")

	// A flag set over an EXPLICIT argument slice, which is what makes this testable.
	var cfg m017ServerFlags
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // keep the course output tidy; normally you would leave this alone
	fs.StringVar(&cfg.Host, "host", "localhost", "host to bind")
	fs.IntVar(&cfg.Port, "port", 8080, "port to listen on")
	fs.BoolVar(&cfg.Verbose, "v", false, "verbose logging")
	fs.DurationVar(&cfg.Timeout, "timeout", 30*time.Second, "request timeout")

	args := []string{"-host", "example.com", "-port=9090", "-v", "-timeout", "5s", "serve", "extra"}
	if err := fs.Parse(args); err != nil {
		fmt.Println("  parse failed:", err)
		return
	}
	// flag.Bool and friends return a POINTER, valid only after Parse:
	//	var v bool = fs.Bool("v", false, "u") // ERROR: cannot use fs.Bool("v", false, "u") (value of type *bool) as bool value in variable declaration
	// and Parse always takes the argument slice - there is no zero-argument form on a FlagSet:
	//	fs.Parse() // ERROR: not enough arguments in call to fs.Parse
	fmt.Printf("  parsed %q\n", args)
	fmt.Printf("    host=%q port=%d verbose=%t timeout=%v\n",
		cfg.Host, cfg.Port, cfg.Verbose, cfg.Timeout)
	fmt.Printf("    positional arguments after parsing: %q\n", fs.Args())
	fmt.Println("    note both spellings worked: `-port=9090` and `-host example.com`")

	// --- Parsing stops at the first non-flag argument ---
	stop := flag.NewFlagSet("stop", flag.ContinueOnError)
	stop.SetOutput(io.Discard)
	after := stop.Bool("after", false, "a flag that comes too late")
	_ = stop.Parse([]string{"build", "-after"})
	fmt.Printf("  `build -after` -> after=%t, positional=%q\n", *after, stop.Args())
	fmt.Println("    parsing STOPPED at `build`, so -after was never treated as a flag")

	// A bare -- stops parsing explicitly, which is how you pass a leading dash through.
	passthrough := flag.NewFlagSet("passthrough", flag.ContinueOnError)
	passthrough.SetOutput(io.Discard)
	pv := passthrough.Bool("v", false, "verbose")
	_ = passthrough.Parse([]string{"-v", "--", "-not-a-flag", "file.txt"})
	fmt.Printf("  `-v -- -not-a-flag file.txt` -> v=%t, positional=%q\n", *pv, passthrough.Args())

	// --- A boolean flag cannot take a separate value ---
	boolFs := flag.NewFlagSet("bool", flag.ContinueOnError)
	boolFs.SetOutput(io.Discard)
	bv := boolFs.Bool("b", false, "a boolean")
	_ = boolFs.Parse([]string{"-b", "true"})
	fmt.Printf("  `-b true` -> b=%t, positional=%q  <- `true` became an ARGUMENT\n",
		*bv, boolFs.Args())
	fmt.Println("    write -b=false when you need to set a boolean flag to false")

	// --- The generated help text ---
	var help strings.Builder
	fs.SetOutput(&help)
	fs.PrintDefaults()
	fmt.Println("  fs.PrintDefaults() generates the help from the declarations:")
	for line := range strings.SplitSeq(strings.TrimRight(help.String(), "\n"), "\n") {
		fmt.Printf("    %s\n", line)
	}
	fmt.Println("    -h is automatic; undefined, it prints this and exits with status 2")
}

// =================================================================================================
// Section 2: Custom Flag Types and Subcommands
// =================================================================================================

/*
## Custom Flag Types and Subcommands

### flag.Value — teaching `flag` a new type

Any type implementing **`flag.Value`** can be a flag:

	type Value interface {
	    String() string
	    Set(string) error
	}

`fs.Var(&v, "name", "usage")` then wires it up. This is how you get a repeatable flag
(`-tag a -tag b`), a comma-separated list, an enum that rejects invalid values, or a path that must
exist. `Set` is called **once per occurrence**, so appending in `Set` gives you repeatability for
free.

Two shortcuts worth knowing:

  - **`fs.TextVar`** (Go 1.19) accepts anything implementing `encoding.TextUnmarshaler` — so a type
    that already round-trips as text (module 008, Section 6) is a flag with no extra code.
  - **`fs.Func(name, usage, fn)`** takes a `func(string) error` directly, for a one-off parse with
    no new type.

### Subcommands

`flag` has no subcommand support, but the pattern is three lines: switch on the first positional
argument, then let that branch parse the rest with **its own `FlagSet`**. This is exactly how the
`go` tool itself is structured.

### Error handling mode

`NewFlagSet` takes one of three:

  - **`ExitOnError`** — print the error and call `os.Exit(2)`. What `flag.CommandLine` uses. Fine in
    `main`, fatal anywhere you want to test, because it kills the process.
  - **`ContinueOnError`** — return the error from `Parse`. **Use this**: it is testable, and it lets
    `main` decide the exit code.
  - **`PanicOnError`** — panic. Rarely what you want.

Note that `flag.ErrHelp` is returned (not an error condition) when the user asked for `-h`; treat it
as a successful exit, not a failure.
*/

// m017StringList is a repeatable flag: -tag a -tag b yields []string{"a", "b"}.
type m017StringList []string

func (l *m017StringList) String() string { return strings.Join(*l, ",") }

func (l *m017StringList) Set(v string) error {
	if v == "" {
		return errors.New("empty value")
	}
	*l = append(*l, v) // called once per occurrence, so appending gives repeatability
	return nil
}

// m017LogLevel is an enum flag that rejects anything outside a fixed set.
type m017LogLevel string

func (l *m017LogLevel) String() string { return string(*l) }

func (l *m017LogLevel) Set(v string) error {
	switch v {
	case "debug", "info", "warn", "error":
		*l = m017LogLevel(v)
		return nil
	default:
		return fmt.Errorf("invalid log level %q (want debug, info, warn or error)", v)
	}
}

func m017CustomFlagsAndSubcommands() {
	fmt.Println("\n--- Section 2: Custom Flag Types and Subcommands ---")

	// --- A repeatable flag ---
	var tags m017StringList
	var level m017LogLevel = "info"
	fs := flag.NewFlagSet("custom", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Var(&tags, "tag", "tag to apply (repeatable)")
	fs.Var(&level, "level", "log level: debug, info, warn, error")

	if err := fs.Parse([]string{"-tag", "alpha", "-tag", "beta", "-level", "debug"}); err != nil {
		fmt.Println("  parse failed:", err)
	}
	fmt.Printf("  repeatable flag: tags=%v  enum flag: level=%q\n", tags, level)

	// The enum flag rejects a bad value at PARSE time, not somewhere deep in the program.
	bad := flag.NewFlagSet("bad", flag.ContinueOnError)
	bad.SetOutput(io.Discard)
	var badLevel m017LogLevel
	bad.Var(&badLevel, "level", "log level")
	err := bad.Parse([]string{"-level", "verbose"})
	fmt.Printf("  rejected at parse time: %v\n", err)

	// --- flag.Func, for a one-off parse with no new type ---
	pairs := map[string]string{}
	fn := flag.NewFlagSet("fn", flag.ContinueOnError)
	fn.SetOutput(io.Discard)
	fn.Func("set", "key=value (repeatable)", func(v string) error {
		k, val, ok := strings.Cut(v, "=")
		if !ok {
			return fmt.Errorf("%q is not key=value", v)
		}
		pairs[k] = val
		return nil
	})
	_ = fn.Parse([]string{"-set", "env=prod", "-set", "region=eu"})
	fmt.Printf("  flag.Func collected: %v\n", pairs)

	// --- Subcommands ---
	fmt.Println("  subcommands: switch on the first positional, then parse with its own FlagSet")
	for _, argv := range [][]string{
		{"serve", "-port", "9000"},
		{"migrate", "-steps", "3", "-dry-run"},
		{"version"},
		{"explode"},
		{},
	} {
		fmt.Printf("    %-32q -> %s\n", strings.Join(argv, " "), m017Dispatch(argv))
	}

	// --- ErrHelp is not a failure ---
	helpFs := flag.NewFlagSet("help", flag.ContinueOnError)
	helpFs.SetOutput(io.Discard)
	helpFs.Bool("x", false, "something")
	helpErr := helpFs.Parse([]string{"-h"})
	fmt.Printf("  `-h` returns flag.ErrHelp: %t — exit 0, not a failure\n",
		errors.Is(helpErr, flag.ErrHelp))
}

// m017Dispatch is the whole subcommand pattern: pick the branch, then let it parse the rest.
func m017Dispatch(argv []string) string {
	if len(argv) == 0 {
		return "usage: tool <serve|migrate|version> [flags]"
	}

	cmd, rest := argv[0], argv[1:]
	switch cmd {
	case "serve":
		fs := flag.NewFlagSet("serve", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		port := fs.Int("port", 8080, "port to listen on")
		if err := fs.Parse(rest); err != nil {
			return fmt.Sprintf("serve: %v", err)
		}
		return fmt.Sprintf("serve on port %d", *port)

	case "migrate":
		fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		steps := fs.Int("steps", 1, "how many migrations to apply")
		dry := fs.Bool("dry-run", false, "print without applying")
		if err := fs.Parse(rest); err != nil {
			return fmt.Sprintf("migrate: %v", err)
		}
		return fmt.Sprintf("migrate %d step(s), dryRun=%t", *steps, *dry)

	case "version":
		return "tool v1.0.0"

	default:
		return fmt.Sprintf("unknown subcommand %q", cmd)
	}
}

// =================================================================================================
// Section 3: Validating User Input
// =================================================================================================

/*
## Validating User Input

- Parsing and validating are different jobs. `flag` guarantees the **shape** (this is an int, that
  is a duration); only your code knows the **rules** (the port must be in range, the two dates must
  be in order, exactly one of `-file` and `-url` must be given).
- **Validate everything at once**, not one failure at a time. A tool that reports "port out of
  range", then after a fix "timeout must be positive", then "host required" wastes three round
  trips. `errors.Join` (module 009, Section 3) collects them all into one error whose message is
  one failure per line.
- Distinguish the three kinds of rule, because they need different code:
    1. **per-field** — a range, a pattern, a non-empty string
    2. **cross-field** — start before end, mutually exclusive flags, "if `-tls` then `-cert` too"
    3. **environmental** — the file exists, the port is free, the directory is writable. These can
       change between validating and using, so they are *checks*, not guarantees.
- **Was a flag actually set?** A default is indistinguishable from an explicitly-given equal value.
  `fs.Visit` walks only the flags the user set (`fs.VisitAll` walks all of them) — that is how you
  implement "this flag is required" and how configuration precedence works in Section 6.
- **Never trust a path from the user.** Use `os.Root` (module 013, Section 4) to confine file access
  rather than inspecting the string; `filepath.Clean` plus a prefix check is famously easy to defeat.
- Report failures on **stderr** and exit non-zero (Section 4). Printing an error on stdout corrupts
  whatever is consuming your output in a pipe.
*/

type m017Config struct {
	Host      string
	Port      int
	Timeout   time.Duration
	Retries   int
	OutputDir string
	Source    string // exactly one of Source/URL
	URL       string
}

// m017Validate reports EVERY problem at once. Compare with module 009 Section 3.
func m017Validate(c m017Config) error {
	var problems []error

	// 1. per-field rules
	if c.Host == "" {
		problems = append(problems, errors.New("host must not be empty"))
	}
	if c.Port < 1 || c.Port > 65535 {
		problems = append(problems, fmt.Errorf("port %d is out of range 1..65535", c.Port))
	}
	if c.Timeout <= 0 {
		problems = append(problems, fmt.Errorf("timeout must be positive, got %v", c.Timeout))
	}
	if c.Retries < 0 {
		problems = append(problems, fmt.Errorf("retries must not be negative, got %d", c.Retries))
	}

	// 2. cross-field rules
	switch {
	case c.Source == "" && c.URL == "":
		problems = append(problems, errors.New("one of -source or -url is required"))
	case c.Source != "" && c.URL != "":
		problems = append(problems, errors.New("-source and -url are mutually exclusive"))
	}

	return errors.Join(problems...) // nil when problems is empty
}

func m017InputValidation() {
	fmt.Println("\n--- Section 3: Validating User Input ---")

	good := m017Config{Host: "localhost", Port: 8080, Timeout: time.Second, Source: "in.txt"}
	fmt.Printf("  valid config -> %v\n", m017Validate(good))

	bad := m017Config{Port: 99999, Timeout: -1, Retries: -2, Source: "a", URL: "b"}
	if err := m017Validate(bad); err != nil {
		fmt.Println("  every problem reported at once, not one per run:")
		for line := range strings.SplitSeq(err.Error(), "\n") {
			fmt.Printf("    %s\n", line)
		}
	}

	// --- Which flags did the user actually set? ---
	fs := flag.NewFlagSet("visit", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.String("host", "localhost", "host")
	fs.Int("port", 8080, "port")
	fs.Bool("v", false, "verbose")
	_ = fs.Parse([]string{"-port", "8080"}) // the SAME value as the default, but set explicitly

	var set, all []string
	fs.Visit(func(f *flag.Flag) { set = append(set, f.Name) })
	fs.VisitAll(func(f *flag.Flag) { all = append(all, f.Name) })
	fmt.Printf("  fs.Visit (explicitly set): %v\n", set)
	fmt.Printf("  fs.VisitAll (every flag):  %v\n", all)
	fmt.Println("  -port was set to exactly its default, and Visit still reports it - which is")
	fmt.Println("  how `required` flags and configuration precedence (Section 6) are implemented")

	// A required-flag check, in three lines.
	fmt.Printf("  required check: %v\n", m017RequireFlags(fs, "host", "port"))
}

// m017RequireFlags reports which of the named flags the user did not set.
func m017RequireFlags(fs *flag.FlagSet, names ...string) error {
	provided := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { provided[f.Name] = true })

	var missing []error
	for _, n := range names {
		if !provided[n] {
			missing = append(missing, fmt.Errorf("-%s is required", n))
		}
	}
	return errors.Join(missing...)
}

// =================================================================================================
// Section 4: Exit Codes, stdout vs stderr, and os.Exit
// =================================================================================================

/*
## Exit Codes, stdout vs stderr, and os.Exit

- A process communicates success through its **exit status**: `0` means success, anything else is a
  failure. Shell scripts, CI systems, `make` and `&&` all depend on it, so getting it right is not
  optional.
- Go's conventions:
    - **0** — success
    - **1** — general failure (the usual choice for "something went wrong")
    - **2** — usage error; what `flag` uses when parsing fails or `-h` is undefined
    - `os.Exit(status)` sets it explicitly; returning normally from `main` is `0`; an unrecovered
      **panic exits with 2** and prints a stack trace to stderr
- **`os.Exit` does not run deferred functions.** Not one — no `defer file.Close()`, no
  `defer cleanup()`, no `recover`. This is the single most important thing to know about it.
- The consequence is a firm rule: **call `os.Exit` in exactly one place, at the very bottom of
  `main`, and never in a library.** The idiom is to keep a `run() error` that does the work with
  `defer` intact, and let `main` be four lines:

	func main() {
	    if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
	        fmt.Fprintln(os.Stderr, "myapp:", err)
	        os.Exit(1)
	    }
	}

- **`log.Fatal` calls `os.Exit(1)`** and therefore has the same problem. It is fine on the last line
  of `main` and wrong everywhere else. `log.Panic` at least unwinds.
- **stdout is for output, stderr is for diagnostics.** Errors, progress and logs go to stderr so a
  pipe (`mytool | jq`) receives only the data. Prefix messages with the program name, as every Unix
  tool does: `myapp: cannot open config: ...`.
*/

func m017ExitCodesAndStreams() {
	fmt.Println("\n--- Section 4: Exit Codes, stdout vs stderr, and os.Exit ---")

	fmt.Println("  conventions: 0 success | 1 general failure | 2 usage error")
	fmt.Println("  an unrecovered panic exits with 2 and prints a stack trace to stderr")

	// --- os.Exit skips every defer ---
	fmt.Println("  os.Exit runs NO deferred functions:")
	fmt.Println("    " + m017DemoDeferWithReturn())
	fmt.Println("    with os.Exit in its place, that cleanup line would never print -")
	fmt.Println("    no file closed, no lock released, no recover")

	// The shape that keeps defer working.
	fmt.Println("  so main stays four lines and the work lives in run() error:")
	fmt.Println("    func main() {")
	fmt.Println("        if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {")
	fmt.Println("            fmt.Fprintln(os.Stderr, \"myapp:\", err)")
	fmt.Println("            os.Exit(1)")
	fmt.Println("        }")
	fmt.Println("    }")

	// --- Choosing the status from the error ---
	for _, err := range []error{
		nil,
		errors.New("disk full"),
		fmt.Errorf("bad usage: %w", flag.ErrHelp),
		m017UsageError{errors.New("-port is required")},
	} {
		fmt.Printf("  exit status for %-34v -> %d\n", m017Describe(err), m017ExitStatus(err))
	}

	// --- stdout versus stderr ---
	fmt.Println("  stdout carries DATA, stderr carries DIAGNOSTICS:")
	fmt.Println("    `mytool | jq` must not receive your log lines")
	fmt.Println("    prefix messages with the program name: `myapp: cannot open config: ...`")
}

func m017DemoDeferWithReturn() (result string) {
	defer func() { result += " -> and this deferred cleanup ran" }()
	return "returned normally"
}

// m017UsageError marks an error as a usage problem, which maps to exit status 2.
type m017UsageError struct{ Err error }

func (e m017UsageError) Error() string { return e.Err.Error() }
func (e m017UsageError) Unwrap() error { return e.Err }

// m017ExitStatus turns an error into a process exit status.
func m017ExitStatus(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, flag.ErrHelp):
		return 0 // the user asked for help and got it: that is success
	case errors.As(err, new(m017UsageError)):
		return 2
	default:
		return 1
	}
}

func m017Describe(err error) string {
	if err == nil {
		return "nil"
	}
	return strconv.Quote(err.Error())
}

// =================================================================================================
// Section 5: Environment Variables
// =================================================================================================

/*
## Environment Variables

- **`os.Getenv(k)`** returns `""` for an unset variable, which is indistinguishable from a variable
  set to the empty string. **`os.LookupEnv(k)`** returns `(value, ok)` and is what you want whenever
  "unset" and "empty" mean different things — which, for configuration, is nearly always.
- `os.Setenv`, `os.Unsetenv`, `os.Environ()` (every variable as `KEY=value`) and `os.ExpandEnv`
  (substitute `$VAR` in a string) complete the set.
- The environment is **process-wide and inherited by children**, which makes it convenient and
  makes it a poor place for anything sensitive: it appears in `ps e` output on some systems, in
  crash dumps, and in the environment of every subprocess you spawn. **Secrets belong in a file
  or a secret manager**, not `-password` on the command line and preferably not in the environment.
- Conventions: `SCREAMING_SNAKE_CASE`, prefixed with the application name (`MYAPP_PORT`) so it
  cannot collide. This is the **12-factor** approach and it is what container orchestrators expect.
- Everything arrives as a **string**, so every non-string setting needs `strconv` and every
  conversion can fail. Report the variable's name in the error — `MYAPP_PORT: invalid syntax` is
  actionable, `invalid syntax` is not.
- In tests, use **`t.Setenv`** (module 014): it restores the previous value automatically and
  deliberately marks the test as non-parallel, because the environment is process-wide state.
*/

func m017EnvironmentVariables() {
	fmt.Println("\n--- Section 5: Environment Variables ---")

	// Getenv cannot tell "unset" from "set to empty".
	const unset = "M017_DEFINITELY_NOT_SET"
	fmt.Printf("  os.Getenv(%q) = %q  <- but is it unset, or set to \"\"?\n", unset, os.Getenv(unset))
	if v, ok := os.LookupEnv(unset); !ok {
		fmt.Printf("  os.LookupEnv gives the answer: value=%q ok=%t\n", v, ok)
	}

	// Set one for the demo, then read it back both ways.
	_ = os.Setenv("M017_EMPTY", "")
	defer os.Unsetenv("M017_EMPTY")
	v, ok := os.LookupEnv("M017_EMPTY")
	fmt.Printf("  a variable set to the empty string: value=%q ok=%t  <- ok is TRUE\n", v, ok)

	// Typed lookups, each reporting the variable name on failure.
	_ = os.Setenv("M017_PORT", "9090")
	_ = os.Setenv("M017_DEBUG", "true")
	_ = os.Setenv("M017_TIMEOUT", "15s")
	_ = os.Setenv("M017_BROKEN", "not-a-number")
	defer func() {
		for _, k := range []string{"M017_PORT", "M017_DEBUG", "M017_TIMEOUT", "M017_BROKEN"} {
			_ = os.Unsetenv(k)
		}
	}()

	port, err := m017EnvInt("M017_PORT", 8080)
	fmt.Printf("  M017_PORT    -> %d (err=%v)\n", port, err)
	debug, err := m017EnvBool("M017_DEBUG", false)
	fmt.Printf("  M017_DEBUG   -> %t (err=%v)\n", debug, err)
	timeout, err := m017EnvDuration("M017_TIMEOUT", time.Minute)
	fmt.Printf("  M017_TIMEOUT -> %v (err=%v)\n", timeout, err)
	broken, err := m017EnvInt("M017_BROKEN", 1)
	fmt.Printf("  M017_BROKEN  -> %d (err=%v)\n", broken, err)
	fmt.Println("    ^ the error names the VARIABLE, which is what makes it actionable")

	// An unset variable falls back to the default with no error.
	fallback, err := m017EnvInt("M017_ABSENT", 42)
	fmt.Printf("  M017_ABSENT  -> %d (err=%v) - unset is not an error, it is a default\n",
		fallback, err)

	// os.ExpandEnv and the size of the environment.
	fmt.Printf("  os.ExpandEnv: %q\n", os.ExpandEnv("port is $M017_PORT"))
	fmt.Printf("  this process has %d environment variables\n", len(os.Environ()))

	fmt.Println("  conventions: MYAPP_PORT (prefixed, SCREAMING_SNAKE_CASE) - the 12-factor style")
	fmt.Println("  secrets do NOT belong here: the environment is inherited by every subprocess")
}

func m017EnvInt(key string, fallback int) (int, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return fallback, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}

func m017EnvBool(key string, fallback bool) (bool, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback, nil
	}
	b, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback, fmt.Errorf("%s: %w", key, err)
	}
	return b, nil
}

func m017EnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw, ok := os.LookupEnv(key)
	if !ok || raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return fallback, fmt.Errorf("%s: %w", key, err)
	}
	return d, nil
}

// =================================================================================================
// Section 6: Application Configuration
// =================================================================================================

/*
## Application Configuration

Real programs take configuration from several places at once, and the value that wins must be
predictable. The **conventional precedence**, lowest to highest, is:

	1. compiled-in defaults      always present, so the program runs with no configuration at all
	2. a configuration file      the deployment's settings, version-controlled
	3. environment variables     the container's or the operator's overrides
	4. command-line flags        this one invocation

Each layer overrides the one below it. The rule of thumb is **the more specific and the more
immediate the source, the higher it wins** — a flag typed just now beats a file written last month.

Implementing it cleanly needs one thing from Section 3: knowing **whether a flag was actually
set**. Applying flags with `fs.Visit` rather than reading every flag's value is what stops a
default from silently overriding the environment.

Other points worth settling early:

  - Load into **one struct**, validate it **once** (Section 3), and pass it explicitly. A global
    `Config` variable is convenient for a week and untestable forever.
  - Keep the **defaults in code**, not in a sample file the user might not have.
  - Make the program **print its effective configuration** on request (`-config-dump`) with secrets
    redacted. It turns "why is it connecting to the wrong host" into a five-second question.
  - `encoding/json` is enough for a config file, and needs `DisallowUnknownFields` (module 013,
    Section 5) so a typo is an error rather than a silently ignored key. TOML and YAML need a
    dependency; that is the main argument for JSON.
  - `viper` and `koanf` do all of this for you. For a handful of settings, this is 40 lines and no
    dependency.
*/

// m017AppConfig is loaded from four layers, then validated once.
type m017AppConfig struct {
	Host     string        `json:"host"`
	Port     int           `json:"port"`
	Timeout  time.Duration `json:"-"` // durations need custom handling in JSON (module 013 §6)
	LogLevel string        `json:"logLevel"`
	Password string        `json:"password"`
}

// String redacts the secret, so the struct is safe to print.
func (c m017AppConfig) String() string {
	password := "(unset)"
	if c.Password != "" {
		password = "***redacted***"
	}
	return fmt.Sprintf("host=%s port=%d timeout=%v logLevel=%s password=%s",
		c.Host, c.Port, c.Timeout, c.LogLevel, password)
}

// m017DefaultConfig is layer 1: the program runs with no configuration at all.
func m017DefaultConfig() m017AppConfig {
	return m017AppConfig{
		Host:     "localhost",
		Port:     8080,
		Timeout:  30 * time.Second,
		LogLevel: "info",
	}
}

// m017LoadConfig applies all four layers in order, lowest precedence first.
func m017LoadConfig(fileJSON string, env map[string]string, args []string) (m017AppConfig, error) {
	cfg := m017DefaultConfig() // 1. defaults

	// 2. configuration file
	if fileJSON != "" {
		dec := json.NewDecoder(strings.NewReader(fileJSON))
		dec.DisallowUnknownFields() // a typo is an error, not a silently ignored key
		if err := dec.Decode(&cfg); err != nil {
			return cfg, fmt.Errorf("config file: %w", err)
		}
	}

	// 3. environment variables
	if v, ok := env["M017APP_HOST"]; ok && v != "" {
		cfg.Host = v
	}
	if v, ok := env["M017APP_PORT"]; ok && v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return cfg, fmt.Errorf("M017APP_PORT: %w", err)
		}
		cfg.Port = n
	}
	if v, ok := env["M017APP_LOG_LEVEL"]; ok && v != "" {
		cfg.LogLevel = v
	}

	// 4. command-line flags — applied with Visit, so an unset flag's DEFAULT never
	//    overrides what the file or the environment provided.
	fs := flag.NewFlagSet("app", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	host := fs.String("host", cfg.Host, "host to bind")
	port := fs.Int("port", cfg.Port, "port to listen on")
	logLevel := fs.String("log-level", cfg.LogLevel, "log level")
	timeout := fs.Duration("timeout", cfg.Timeout, "request timeout")
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}
	fs.Visit(func(f *flag.Flag) { // ONLY the flags the user actually set
		switch f.Name {
		case "host":
			cfg.Host = *host
		case "port":
			cfg.Port = *port
		case "log-level":
			cfg.LogLevel = *logLevel
		case "timeout":
			cfg.Timeout = *timeout
		}
	})

	return cfg, nil
}

func m017Configuration() {
	fmt.Println("\n--- Section 6: Application Configuration ---")

	const configFile = `{"host": "file-host", "port": 7070, "logLevel": "warn"}`
	env := map[string]string{"M017APP_HOST": "env-host", "M017APP_PORT": "9090"}
	args := []string{"-host", "flag-host"}

	fmt.Println("  precedence, lowest to highest: defaults -> file -> environment -> flags")

	// Each layer added in turn, so the overriding is visible.
	steps := []struct {
		label string
		file  string
		env   map[string]string
		args  []string
	}{
		{"defaults only            ", "", nil, nil},
		{"+ config file            ", configFile, nil, nil},
		{"+ environment            ", configFile, env, nil},
		{"+ flags (final)          ", configFile, env, args},
	}
	for _, s := range steps {
		cfg, err := m017LoadConfig(s.file, s.env, s.args)
		if err != nil {
			fmt.Printf("  %s -> error: %v\n", s.label, err)
			continue
		}
		fmt.Printf("  %s -> %s\n", s.label, cfg)
	}
	fmt.Println("  host came from the flag, port from the environment, logLevel from the file,")
	fmt.Println("  and timeout from the compiled-in default - each layer overrode the one below")

	// The trap that fs.Visit avoids.
	fmt.Println()
	fmt.Println("  why fs.Visit and not just reading every flag value:")
	fmt.Println("    -port is declared with the ALREADY-RESOLVED value as its default, so reading")
	fmt.Println("    it unconditionally would be harmless here - but declaring it with a fixed")
	fmt.Println("    literal default and reading it unconditionally would silently overwrite the")
	fmt.Println("    environment with 8080 on every run where -port was not given")

	// An unknown key in the file is an error, not a silent no-op.
	if _, err := m017LoadConfig(`{"hostt": "typo"}`, nil, nil); err != nil {
		fmt.Printf("  DisallowUnknownFields catches a typo: %v\n", err)
	}

	// Redaction.
	withSecret := m017DefaultConfig()
	withSecret.Password = "hunter2"
	fmt.Printf("  a config-dump is safe to print because String() redacts: %s\n", withSecret)
}

// =================================================================================================
// Section 7: A Testable main
// =================================================================================================

/*
## A Testable main

`func main()` takes no arguments, returns nothing, reads global state (`os.Args`, the environment,
`os.Stdin`) and writes to global state (`os.Stdout`, `os.Exit`). Every one of those makes it
untestable. The fix is mechanical and worth doing in every program you write:

	func main() {
	    if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
	        fmt.Fprintln(os.Stderr, "myapp:", err)
	        os.Exit(m017ExitStatus(err))
	    }
	}

	func run(args []string, stdout, stderr io.Writer) error { ... }

`main` now does three things — wire the real world in, print the error, choose the exit status —
and **`run` is an ordinary function you can call from a test**: pass an argument slice, pass
`bytes.Buffer`s, assert on the output and the returned error. No subprocess, no golden files, no
capturing `os.Stdout`.

The same idea extends to everything else `main` touches:

  - the **argument slice** is a parameter, so a test passes its own
  - **stdout and stderr** are `io.Writer`s, so a test passes buffers (module 008, Section 4)
  - **stdin** is an `io.Reader`, so a test passes `strings.NewReader`
  - the **environment** is a `map[string]string` parameter, or read once in `main` and passed in
  - the **clock** is a `func() time.Time` field, so a test passes a fixed one
  - **`os.Exit` is called in exactly one place** — `main` — so nothing else can kill the test binary

This repository's own `main.go` follows the shape: it parses `os.Args[1:]`, dispatches, and does
nothing else. The CLI exercises in `notes.md` (`echo`, `cat`, `find`, `grep` — solved in
`examples/`) are the natural place to practise it.
*/

// m017Run is the whole application, as an ordinary testable function.
func m017Run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("greet", flag.ContinueOnError)
	fs.SetOutput(stderr)
	upper := fs.Bool("upper", false, "upper-case the greeting")
	name := fs.String("name", "", "name to greet; if empty, read from stdin")

	if err := fs.Parse(args); err != nil {
		return m017UsageError{err}
	}

	who := *name
	if who == "" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		who = strings.TrimSpace(string(data))
	}
	if who == "" {
		return m017UsageError{errors.New("-name is required, or provide a name on stdin")}
	}

	greeting := "Hello, " + who + "!"
	if *upper {
		greeting = strings.ToUpper(greeting)
	}
	fmt.Fprintln(stdout, greeting)
	return nil
}

func m017TestableMain() {
	fmt.Println("\n--- Section 7: A Testable main ---")

	// Exactly what a test would do: pass arguments, pass buffers, assert.
	cases := []struct {
		args  []string
		stdin string
	}{
		{[]string{"-name", "Ada"}, ""},
		{[]string{"-name", "Ada", "-upper"}, ""},
		{nil, "Grace\n"},
		{[]string{"-nope"}, ""},
		{nil, ""},
	}

	for _, c := range cases {
		var stdout, stderr strings.Builder
		err := m017Run(c.args, strings.NewReader(c.stdin), &stdout, &stderr)

		label := fmt.Sprintf("args=%v stdin=%q", c.args, c.stdin)
		switch {
		case err != nil:
			fmt.Printf("  %-34s -> exit %d, error: %v\n", label, m017ExitStatus(err), err)
		default:
			fmt.Printf("  %-34s -> exit 0, stdout: %q\n", label, strings.TrimSpace(stdout.String()))
		}
	}

	fmt.Println()
	fmt.Println("  no subprocess, no os.Stdout capture, no golden files - m017Run is just a")
	fmt.Println("  function, so a table-driven test (module 014 §2) covers the whole program")
	fmt.Println("  main() keeps only three jobs: wire the real world in, print the error to")
	fmt.Println("  stderr, and call os.Exit exactly once")
}

// Run017 runs every section of module 017 in order.
func Run017() {
	m017OsArgsAndFlag()
	m017CustomFlagsAndSubcommands()
	m017InputValidation()
	m017ExitCodesAndStreams()
	m017EnvironmentVariables()
	m017Configuration()
	m017TestableMain()
}

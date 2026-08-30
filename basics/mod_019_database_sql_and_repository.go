package basics

import (
	"cmp"
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq" // blank import: registers the "postgres" driver (module 001a §2)
)

/*
# Module 019 — database/sql and the Repository Pattern

`database/sql` is not an ORM and not a query builder. It is a thin, driver-agnostic layer providing
a **connection pool**, **statement preparation**, **transactions** and **row scanning** — and
nothing else. You write SQL. That is the whole design, and it is why the package has stayed
essentially unchanged for a decade.

The driver is chosen by a **blank import** (`_ "github.com/lib/pq"`), whose `init` calls
`sql.Register("postgres", ...)`. That is the canonical example of why blank imports exist.

This module needs the PostgreSQL from `docker-compose.yml`:

	docker compose up -d

**If the database is unreachable the module says so and skips to Section 7**, which runs entirely
in memory — so `go run . all` still succeeds on a machine with no Docker.
*/

const (
	m019DefaultDSN  = "postgres://admin:admin@localhost:5432/users?sslmode=disable"
	m019PingTimeout = 2 * time.Second
)

// m019DSN returns the connection string, letting the environment override the compose default.
// This is module 017's configuration precedence in miniature: a compiled-in default, overridable
// by an environment variable — handy when 5432 is already taken by another project.
func m019DSN() string {
	if dsn, ok := os.LookupEnv("M019_DSN"); ok && dsn != "" {
		return dsn
	}
	return m019DefaultDSN
}

// =================================================================================================
// Section 1: sql.DB Is a Pool, Not a Connection
// =================================================================================================

/*
## sql.DB Is a Pool, Not a Connection

- **`sql.Open` does not connect.** It looks up the registered driver and returns a `*sql.DB` — a
  *pool* that opens connections lazily, on first use. With `lib/pq` even the DSN is only parsed
  when the first connection is opened, so `sql.Open` succeeding tells
  you almost nothing, and a program that only calls `Open` at startup discovers a wrong password on
  its first request instead of at boot.
- **`db.PingContext(ctx)` is what actually connects.** Call it once at startup, with a timeout, and
  fail fast if it does not answer.
- A `*sql.DB` is **safe for concurrent use** and is meant to be **long-lived**: create one per
  database for the lifetime of the program and pass it around. Opening one per request is the
  classic performance bug — you lose the pool entirely.
- `db.Close()` is for shutdown, not for the end of a query. It closes the pool. In a server it
  belongs in the shutdown sequence *after* the HTTP server has drained (module 018, Section 6).
- Every call has a **`Context` variant** — `QueryContext`, `ExecContext`, `PingContext`,
  `BeginTx` — and you should use it exclusively. The non-context forms use `context.Background()`,
  so a client that disconnects leaves the query running to completion against your database.
- `db.Stats()` exposes the pool's live state: open connections, in use, idle, and how much time has
  been spent waiting for one. `WaitCount` climbing is the signal that `MaxOpenConns` is too low.
*/

// m019Connect opens the pool and verifies it. Returns nil when the database is unavailable.
func m019Connect() (*sql.DB, error) {
	db, err := sql.Open("postgres", m019DSN())
	if err != nil {
		return nil, fmt.Errorf("open: %w", err) // only a bad DSN or unknown driver gets here
	}

	m019Tune(db)

	ctx, cancel := context.WithTimeout(context.Background(), m019PingTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return db, nil
}

func m019PoolBasics(db *sql.DB) {
	fmt.Println("--- Section 1: sql.DB Is a Pool, Not a Connection ---")

	fmt.Println("  sql.Open looks up the driver and returns a POOL; it does not connect")
	fmt.Println("  db.PingContext is what connects - call it once at startup, with a timeout")

	// sql.Open succeeds even for a database that does not exist, which is the point.
	bogus, err := sql.Open("postgres", "postgres://nobody@127.0.0.1:1/none?sslmode=disable")
	fmt.Printf("  sql.Open on a host that is not listening: err=%v  <- it still succeeded\n", err)
	pingCtx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	fmt.Printf("  ...and PingContext is where it actually fails: %v\n",
		m019Shorten(bogus.PingContext(pingCtx)))
	bogus.Close()

	// An unknown driver is the other Open-time failure.
	_, err = sql.Open("mysql", "whatever")
	fmt.Printf("  sql.Open with an unregistered driver: %v\n", m019Shorten(err))
	fmt.Println("  the driver is registered by a BLANK IMPORT: _ \"github.com/lib/pq\"")

	if db == nil {
		return
	}
	s := db.Stats()
	fmt.Printf("  live pool stats: open=%d inUse=%d idle=%d waitCount=%d\n",
		s.OpenConnections, s.InUse, s.Idle, s.WaitCount)
	fmt.Println("  a rising WaitCount means MaxOpenConns is too low for the load")
}

// =================================================================================================
// Section 2: Tuning the Pool
// =================================================================================================

/*
## Tuning the Pool

Four settings, and the defaults are wrong for a server.

	db.SetMaxOpenConns(25)                  // default: UNLIMITED
	db.SetMaxIdleConns(25)                  // default: 2
	db.SetConnMaxLifetime(5 * time.Minute)  // default: forever
	db.SetConnMaxIdleTime(2 * time.Minute)  // default: forever

- **`SetMaxOpenConns` defaults to unlimited**, which means a traffic spike can open more connections
  than PostgreSQL's `max_connections` (100 by default) and every one of them fails. Always set it.
  A sensible starting point is well under the server's limit, divided by the number of instances.
- **`SetMaxIdleConns` defaults to 2**, so a busy server closes and reopens connections constantly.
  Set it **equal to `MaxOpenConns`** unless you have a reason not to; an idle connection is cheap
  and a TCP+TLS handshake is not.
- **`SetConnMaxLifetime` defaults to forever.** A finite lifetime is what lets a load balancer
  rebalance, a failover promote a new primary, and a rolling database upgrade actually complete.
  Minutes, not hours.
- **`SetConnMaxIdleTime`** (Go 1.15) reaps connections the pool is not using, which matters when
  traffic is bursty.
- More connections is not faster. Past the database's ability to run queries in parallel, extra
  connections add contention. Measure with `db.Stats()` and raise `MaxOpenConns` only when
  `WaitCount` and `WaitDuration` say you are queueing.
*/

func m019Tune(db *sql.DB) {
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25) // equal to MaxOpenConns: idle connections are cheap
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(2 * time.Minute)
}

func m019PoolTuning(db *sql.DB) {
	fmt.Println("\n--- Section 2: Tuning the Pool ---")

	fmt.Println("  the defaults are wrong for a server:")
	fmt.Println("    MaxOpenConns     unlimited -> a spike exhausts PostgreSQL's max_connections")
	fmt.Println("    MaxIdleConns     2         -> constant reconnection under load")
	fmt.Println("    ConnMaxLifetime  forever   -> failover and rolling upgrades never complete")
	fmt.Println("    ConnMaxIdleTime  forever   -> idle connections held after a burst")
	fmt.Println("  this module sets 25 / 25 / 5m / 2m")

	if db == nil {
		return
	}

	// Run a few queries concurrently so the pool actually has to work.
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			var n int
			_ = db.QueryRowContext(ctx, "select 1").Scan(&n)
		})
	}
	wg.Wait()

	s := db.Stats()
	fmt.Printf("  after 8 concurrent queries: maxOpen=%d open=%d idle=%d waitCount=%d waitDuration=%v\n",
		s.MaxOpenConnections, s.OpenConnections, s.Idle, s.WaitCount, s.WaitDuration)
	fmt.Println("  more connections is not faster - raise MaxOpenConns only when WaitCount climbs")
}

// =================================================================================================
// Section 3: Querying and Scanning
// =================================================================================================

/*
## Querying and Scanning

Three methods, and picking the wrong one is the most common `database/sql` bug.

  - **`ExecContext`** — for `INSERT`, `UPDATE`, `DELETE`, DDL. Returns a `sql.Result`
    (`RowsAffected`, and `LastInsertId` on drivers that support it — **not** PostgreSQL, which needs
    `INSERT ... RETURNING id` with `QueryRow` instead).
  - **`QueryRowContext`** — for exactly one row. Returns a `*sql.Row` whose `Scan` returns
    **`sql.ErrNoRows`** when nothing matched. `Row` closes itself, so there is nothing to clean up.
  - **`QueryContext`** — for many rows. Returns `*sql.Rows`, which **holds a connection until it is
    closed**. This is the one that bites.

The `Rows` contract has four parts and skipping any of them is a bug:

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil { return err }
	defer rows.Close()             // 1. ALWAYS — even on the happy path
	for rows.Next() {              // 2. Next advances and reports whether a row is there
	    if err := rows.Scan(&a, &b); err != nil { return err }
	}
	return rows.Err()              // 3. Next returning false may mean an ERROR, not just "done"

Forgetting `rows.Close()` holds a pooled connection until the query's context is cancelled or the
process exits (`database/sql` sets no finalizer on `Rows`), and a handler that does it on every
request exhausts the pool within minutes. Forgetting
`rows.Err()` silently turns a mid-iteration network failure into "no more rows" — a truncated
result set that looks like success.

Never run a query **inside** a `rows.Next()` loop over the same `*sql.DB`: the outer `Rows` is
holding a connection, so with `MaxOpenConns(1)` you deadlock and with more you just serialise.
Collect the ids first, then query once with `= ANY($1)`.
*/

type m019User struct {
	ID    int64
	Name  string
	Email string
}

func m019QueryingAndScanning(ctx context.Context, db *sql.DB) {
	fmt.Println("\n--- Section 3: Querying and Scanning ---")

	if db == nil {
		fmt.Println("  (skipped: no database)")
		return
	}

	if err := m019Reset(ctx, db); err != nil {
		fmt.Println("  setup failed:", err)
		return
	}

	// ExecContext for writes; RowsAffected tells you what happened.
	res, err := db.ExecContext(ctx,
		`insert into m019_users (name, email) values ($1,$2), ($3,$4), ($5,$6)`,
		"Ada", "ada@example.com", "Alan", "alan@example.com", "Grace", "grace@example.com")
	if err != nil {
		fmt.Println("  insert failed:", err)
		return
	}
	affected, _ := res.RowsAffected()
	fmt.Printf("  ExecContext insert: rowsAffected=%d\n", affected)

	// PostgreSQL has no LastInsertId - use INSERT ... RETURNING with QueryRow.
	var newID int64
	err = db.QueryRowContext(ctx,
		`insert into m019_users (name, email) values ($1,$2) returning id`,
		"Barbara", "barbara@example.com").Scan(&newID)
	fmt.Printf("  INSERT ... RETURNING id -> %d (PostgreSQL has no LastInsertId)\n", newID)
	if _, err := res.LastInsertId(); err != nil {
		fmt.Printf("  and calling LastInsertId confirms it: %v\n", m019Shorten(err))
	}

	// QueryRowContext for exactly one row, and the ErrNoRows sentinel.
	var u m019User
	err = db.QueryRowContext(ctx,
		`select id, name, email from m019_users where name = $1`, "Ada").Scan(&u.ID, &u.Name, &u.Email)
	fmt.Printf("  QueryRowContext found: %+v (err=%v)\n", u, err)

	err = db.QueryRowContext(ctx,
		`select id, name, email from m019_users where name = $1`, "Nobody").Scan(&u.ID, &u.Name, &u.Email)
	fmt.Printf("  a missing row returns the sentinel: errors.Is(err, sql.ErrNoRows)=%t\n",
		errors.Is(err, sql.ErrNoRows))

	// QueryContext for many rows - with all four parts of the contract.
	rows, err := db.QueryContext(ctx, `select id, name, email from m019_users order by id`)
	if err != nil {
		fmt.Println("  query failed:", err)
		return
	}
	defer rows.Close() // 1

	var users []m019User
	for rows.Next() { // 2
		var u m019User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			fmt.Println("  scan failed:", err)
			return
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil { // 3
		fmt.Println("  iteration failed:", err)
		return
	}
	fmt.Printf("  QueryContext returned %d rows:\n", len(users))
	for _, u := range users {
		fmt.Printf("    %d %-8s %s\n", u.ID, u.Name, u.Email)
	}

	// Column metadata, when you do not know the shape ahead of time.
	if cols, err := m019Columns(ctx, db); err == nil {
		fmt.Printf("  rows.Columns(): %v\n", cols)
	}

	fmt.Println("  forgetting rows.Close() leaks a pooled connection; forgetting rows.Err()")
	fmt.Println("  turns a mid-iteration network failure into a silently truncated result")
}

func m019Columns(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx, `select * from m019_users limit 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return rows.Columns()
}

// =================================================================================================
// Section 4: Parameters and SQL Injection
// =================================================================================================

/*
## Parameters and SQL Injection

**Never build SQL by concatenating user input.** Not with `fmt.Sprintf`, not with `+`, not "just
this once because it is only an integer".

	// Catastrophic:
	q := "select * from users where name = '" + name + "'"
	// With name = `x' or '1'='1`, that returns every row.
	// With name = `x'; drop table users; --`, it does what it says.

	// Correct — the value never becomes part of the statement:
	db.QueryContext(ctx, "select * from users where name = $1", name)

A **placeholder** sends the query and the values to the server separately. The value is never
parsed as SQL, so no content can change the statement's meaning. It is not escaping — it is a
different transport.

The placeholder syntax is **driver-specific**, which is one of the few places `database/sql` is not
portable: `$1, $2` for PostgreSQL (`lib/pq`, `pgx`), `?` for MySQL and SQLite, `:name` for Oracle.

Two things placeholders **cannot** parameterise, because they are part of the statement's structure,
not its data:

  - **table and column names** — `select * from $1` is invalid
  - **`ORDER BY` direction and keywords** — `order by $1` will not work

For those, validate against an **allow-list** of known-good identifiers. Never interpolate the raw
string, even from an "internal" caller.

`IN` clauses need care too: `where id in ($1)` binds *one* value. In PostgreSQL use
`where id = any($1)` with `pq.Array`; elsewhere, build the right number of placeholders
programmatically — placeholders, not values.
*/

func m019ParametersAndInjection(ctx context.Context, db *sql.DB) {
	fmt.Println("\n--- Section 4: Parameters and SQL Injection ---")

	fmt.Println("  a placeholder sends query and values SEPARATELY - the value is never parsed")
	fmt.Println("  as SQL, so no content can change the statement's meaning")
	fmt.Println("  syntax is driver-specific: $1 (PostgreSQL), ? (MySQL/SQLite), :name (Oracle)")

	if db == nil {
		fmt.Println("  (live demonstration skipped: no database)")
	} else {
		// The classic injection payload, passed as a PARAMETER: it matches nothing.
		payload := `x' or '1'='1`
		var count int
		err := db.QueryRowContext(ctx,
			`select count(*) from m019_users where name = $1`, payload).Scan(&count)
		fmt.Printf("  parameterised with %q -> %d rows (err=%v)\n", payload, count, err)

		var total int
		_ = db.QueryRowContext(ctx, `select count(*) from m019_users`).Scan(&total)
		fmt.Printf("  the table actually holds %d rows, so the payload matched nothing\n", total)
		fmt.Println("  concatenated into the SQL, that same string would have returned all of them")

		// An IN clause needs = any($1), not in ($1).
		rows, err := db.QueryContext(ctx,
			`select name from m019_users where id = any($1) order by id`, m019Int64Array{1, 2})
		if err != nil {
			fmt.Println("  query with an array parameter failed:", err)
		} else {
			defer rows.Close()
			var names []string
			for rows.Next() {
				var n string
				if err := rows.Scan(&n); err != nil {
					fmt.Println("  scan failed:", err)
					break
				}
				names = append(names, n)
			}
			if err := rows.Err(); err != nil {
				fmt.Println("  iteration failed:", err)
			} else {
				fmt.Printf("  `where id = any($1)` binds a whole list: %v\n", names)
			}
		}
	}

	// --- What a placeholder cannot do ---
	fmt.Println("  placeholders cannot parameterise IDENTIFIERS or keywords:")
	fmt.Println("    select * from $1     -- invalid")
	fmt.Println("    order by $1          -- binds a value, never a column")
	for _, candidate := range []string{"name", "email", "id; drop table m019_users"} {
		col, err := m019SafeColumn(candidate)
		fmt.Printf("    allow-list %-28q -> %q (err=%v)\n", candidate, col, err)
	}
	fmt.Println("  an allow-list is the only safe way to vary a column or direction")
}

// m019SafeColumn validates an identifier against a fixed set. Never interpolate anything else.
func m019SafeColumn(name string) (string, error) {
	allowed := []string{"id", "name", "email"}
	if slices.Contains(allowed, name) {
		return name, nil
	}
	return "", fmt.Errorf("column %q is not sortable", name)
}

// m019Int64Array implements driver.Valuer so a slice can be bound to `= any($1)`.
// (github.com/lib/pq offers pq.Array for this; hand-rolling it shows what it does.)
type m019Int64Array []int64

// driver.Value is a DEFINED type (`type Value any`), not an alias, so the method must name it
// exactly - `Value() (any, error)` would not satisfy the interface. The blank assignment below
// makes the compiler check that.
var _ driver.Valuer = m019Int64Array(nil)

func (a m019Int64Array) Value() (driver.Value, error) {
	parts := make([]string, len(a))
	for i, v := range a {
		parts[i] = fmt.Sprint(v)
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

// =================================================================================================
// Section 5: NULL, and Mapping Errors
// =================================================================================================

/*
## NULL, and Mapping Errors

SQL's `NULL` has no Go equivalent, and scanning it into a `string` or an `int` fails with
`converting NULL to string is unsupported`. There are three ways out:

  - **`sql.NullString`, `sql.NullInt64`, `sql.NullTime`, …** — a struct with a value and a `Valid`
    bool. Explicit, and awkward to use: every read is `if v.Valid`.
  - **`sql.Null[T]`** (**Go 1.22**) — the generic version, so `sql.Null[string]` and
    `sql.Null[time.Time]` replace the whole family.
  - **A pointer** — `*string` scans `NULL` as `nil`. The most natural to *use*, and it matches how
    `encoding/json` distinguishes absent from empty (module 013, Section 5).
  - **`COALESCE(col, '')` in the query** — pushes the decision to the database. Best when the zero
    value genuinely is the right default.

Choose per column, and prefer making the column `NOT NULL DEFAULT ...` when you control the schema.
A nullable column is a decision every reader has to handle forever.

### Errors worth naming

  - **`sql.ErrNoRows`** — from `QueryRow(...).Scan`. It is a *sentinel* (module 009, Section 2), so
    match it with `errors.Is`, and translate it at the repository boundary into your own domain
    error rather than leaking `database/sql` into your business logic.
  - **`sql.ErrTxDone`** — using a transaction after `Commit` or `Rollback`.
  - **`sql.ErrConnDone`** — using a `*sql.Conn` after it was returned to the pool.
  - Driver-specific errors — a unique-constraint violation is `*pq.Error` with `Code == "23505"`.
    Match on it with `errors.As` to turn a duplicate insert into a clean 409, but keep that
    knowledge inside the repository.
*/

func m019NullsAndErrors(ctx context.Context, db *sql.DB) {
	fmt.Println("\n--- Section 5: NULL, and Mapping Errors ---")

	fmt.Println("  scanning NULL into a plain string fails: converting NULL to string is unsupported")
	fmt.Println("  four ways out: sql.NullString | sql.Null[T] (Go 1.22) | *string | COALESCE")

	if db == nil {
		fmt.Println("  (live demonstration skipped: no database)")
	} else {
		if _, err := db.ExecContext(ctx,
			`insert into m019_users (name, email, nickname) values ($1,$2,$3)`,
			"Nullable", "null@example.com", nil); err != nil {
			fmt.Println("  insert failed:", err)
			return
		}

		const q = `select nickname from m019_users where name = $1`

		// 1. A plain string fails.
		var plain string
		err := db.QueryRowContext(ctx, q, "Nullable").Scan(&plain)
		fmt.Printf("  into a string:        %v\n", m019Shorten(err))

		// 2. sql.NullString.
		var ns sql.NullString
		_ = db.QueryRowContext(ctx, q, "Nullable").Scan(&ns)
		fmt.Printf("  into sql.NullString:  value=%q valid=%t\n", ns.String, ns.Valid)

		// 3. sql.Null[T] - Go 1.22, the generic replacement for the whole family.
		var generic sql.Null[string]
		_ = db.QueryRowContext(ctx, q, "Nullable").Scan(&generic)
		fmt.Printf("  into sql.Null[string]: value=%q valid=%t  <- Go 1.22\n", generic.V, generic.Valid)

		// 4. A pointer.
		var ptr *string
		_ = db.QueryRowContext(ctx, q, "Nullable").Scan(&ptr)
		fmt.Printf("  into *string:         nil=%t\n", ptr == nil)

		// 5. COALESCE, deciding in the query.
		var coalesced string
		_ = db.QueryRowContext(ctx,
			`select coalesce(nickname, '(none)') from m019_users where name = $1`,
			"Nullable").Scan(&coalesced)
		fmt.Printf("  with COALESCE:        %q\n", coalesced)

		// --- ErrTxDone ---
		tx, err := db.BeginTx(ctx, nil)
		if err == nil {
			_ = tx.Rollback()
			_, err = tx.ExecContext(ctx, `select 1`)
			fmt.Printf("  using a finished transaction: errors.Is(err, sql.ErrTxDone)=%t\n",
				errors.Is(err, sql.ErrTxDone))
		}

		// --- A unique violation, translated at the boundary ---
		_, err = db.ExecContext(ctx,
			`insert into m019_users (name, email) values ($1,$2)`, "Dup", "ada@example.com")
		fmt.Printf("  duplicate email -> %v\n", m019Shorten(m019TranslateError(err)))
	}

	fmt.Println("  translate sql.ErrNoRows into a DOMAIN error at the repository boundary,")
	fmt.Println("  so database/sql never leaks into business logic (Section 7)")
}

// m019ErrDuplicate is this package's domain error for a unique-constraint violation.
var m019ErrDuplicate = errors.New("already exists")

// m019TranslateError maps driver errors to domain errors. Driver knowledge stops here.
func m019TranslateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return m009ErrNotFound
	}
	// lib/pq reports a unique violation as SQLSTATE 23505. Matching on the string keeps this
	// module free of a pq import; real code uses errors.As with *pq.Error.
	if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key") {
		return m019ErrDuplicate
	}
	return err
}

// =================================================================================================
// Section 6: Transactions
// =================================================================================================

/*
## Transactions

	tx, err := db.BeginTx(ctx, nil)
	if err != nil { return err }
	defer tx.Rollback()           // no-op after a successful Commit
	// ... tx.ExecContext / tx.QueryContext ...
	return tx.Commit()

- **`defer tx.Rollback()` immediately after `BeginTx`** is the idiom. After a successful `Commit`
  the rollback returns `sql.ErrTxDone` and changes nothing, so it is safe — and it guarantees that
  every early return, and every panic, releases the transaction. Without it, an early `return err`
  holds the connection for as long as the transaction lives.
- A `*sql.Tx` **holds exactly one connection** for its whole life. Two consequences: keep
  transactions short, and **never** use the parent `db` inside one — those queries go to a different
  connection and are not part of the transaction.
- **`BeginTx` takes the context**, and cancelling it rolls the transaction back automatically.
- `&sql.TxOptions{Isolation: sql.LevelSerializable, ReadOnly: true}` sets the isolation level.
  The default is the driver's, which for PostgreSQL is `READ COMMITTED`.
- At `SERIALIZABLE`, PostgreSQL may abort a transaction with a **serialization failure** (SQLSTATE
  `40001`) that is expected and **retryable**. Code that uses it needs a retry loop; that is the
  cost of the guarantee.
- Do not hold a transaction open across an HTTP call or any other slow external work. You are
  holding a connection and, usually, row locks.
*/

func m019Transactions(ctx context.Context, db *sql.DB) {
	fmt.Println("\n--- Section 6: Transactions ---")

	if db == nil {
		fmt.Println("  (skipped: no database)")
		return
	}

	// A transaction lives on *sql.Tx, never on the pool - the pool has no Commit at all:
	//	db.Commit() // ERROR: db.Commit undefined (type *sql.DB has no field or method Commit)

	before := m019Count(ctx, db)

	// A transaction that commits.
	if err := m019Transfer(ctx, db, "Committed A", "Committed B", true); err != nil {
		fmt.Println("  commit path failed:", err)
	}
	afterCommit := m019Count(ctx, db)

	// A transaction that rolls back - the deferred Rollback does the work.
	err := m019Transfer(ctx, db, "Rolled A", "Rolled B", false)
	afterRollback := m019Count(ctx, db)

	fmt.Printf("  rows before=%d afterCommit=%d afterRollback=%d\n", before, afterCommit, afterRollback)
	fmt.Printf("  the rollback path returned: %v\n", m019Shorten(err))
	fmt.Println("  both inserts of the failed transaction vanished - that is atomicity")

	// Read-only and isolation level.
	roTx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelReadCommitted})
	if err == nil {
		var n int
		_ = roTx.QueryRowContext(ctx, `select count(*) from m019_users`).Scan(&n)
		_, writeErr := roTx.ExecContext(ctx, `insert into m019_users (name, email) values ('x','x@x')`)
		fmt.Printf("  a read-only transaction can read (%d rows) but not write: %v\n",
			n, m019Shorten(writeErr))
		_ = roTx.Rollback()
	}

	fmt.Println("  `defer tx.Rollback()` is safe after Commit: it returns sql.ErrTxDone and")
	fmt.Println("  changes nothing, while guaranteeing every early return releases the connection")
	fmt.Println("  never use the parent db inside a transaction - that is a different connection")
}

// m019Transfer inserts two rows atomically, failing deliberately when commit is false.
func m019Transfer(ctx context.Context, db *sql.DB, a, b string, commit bool) (err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback() // the whole safety net, in one line

	if _, err = tx.ExecContext(ctx,
		`insert into m019_users (name, email) values ($1,$2)`, a, a+"@example.com"); err != nil {
		return fmt.Errorf("insert %s: %w", a, err)
	}
	if _, err = tx.ExecContext(ctx,
		`insert into m019_users (name, email) values ($1,$2)`, b, b+"@example.com"); err != nil {
		return fmt.Errorf("insert %s: %w", b, err)
	}

	if !commit {
		return errors.New("deliberate failure before commit: both inserts are discarded")
	}
	return tx.Commit()
}

func m019Count(ctx context.Context, db *sql.DB) int {
	var n int
	_ = db.QueryRowContext(ctx, `select count(*) from m019_users`).Scan(&n)
	return n
}

// =================================================================================================
// Section 7: The Repository Pattern
// =================================================================================================

/*
## The Repository Pattern

A **repository** is an interface, owned by the code that *uses* it, describing persistence in the
language of the domain rather than of SQL:

	type UserRepository interface {
	    Create(ctx context.Context, u *User) error
	    ByID(ctx context.Context, id int64) (*User, error)
	    List(ctx context.Context) ([]User, error)
	    Delete(ctx context.Context, id int64) error
	}

Why it is worth the indirection:

  - the **domain never imports `database/sql`**; `sql.ErrNoRows` is translated into a domain error
    at the boundary (Section 5), so business logic depends on meaning, not on a driver
  - **tests need no database.** An in-memory implementation is thirty lines, runs in microseconds,
    and cannot flake. Contrast a testcontainer, which is the right tool for testing the *SQL*
  - the storage engine becomes **replaceable** — module 020 implements this same interface with GORM
  - it names the operations the application actually performs, which is a design discipline in
    itself: an interface that grows a `FindByAnyFieldWithSorting` method is telling you something

Where it goes wrong:

  - a **generic `Repository[T]`** with `Save`/`Find`/`Delete` for every entity abstracts away the
    thing that matters. Different aggregates need different operations.
  - **leaking query objects** — a method taking a `WhereClause` struct is a query builder wearing a
    repository's clothes, and it re-couples the caller to the database.
  - **hiding transactions.** Something has to compose several repository calls atomically. Either
    pass an explicit transaction handle, or expose a `WithTx(ctx, func(Repository) error)` method.
    Pretending transactions do not exist is the most common failure of this pattern.

Go's idiom of **defining the interface where it is consumed** (module 008) means the repository
interface belongs in the package that uses it, and the PostgreSQL implementation in its own package
that imports nothing of the domain but the entity type.
*/

// m019UserRepository is the interface the application depends on. Note: no SQL, no driver.
type m019UserRepository interface {
	Create(ctx context.Context, u *m019User) error
	ByID(ctx context.Context, id int64) (*m019User, error)
	List(ctx context.Context) ([]m019User, error)
	Delete(ctx context.Context, id int64) error
}

// --- The PostgreSQL implementation ---

type m019PostgresRepo struct{ db *sql.DB }

func (r *m019PostgresRepo) Create(ctx context.Context, u *m019User) error {
	err := r.db.QueryRowContext(ctx,
		`insert into m019_users (name, email) values ($1,$2) returning id`,
		u.Name, u.Email).Scan(&u.ID)
	return m019TranslateError(err) // driver knowledge stops here
}

func (r *m019PostgresRepo) ByID(ctx context.Context, id int64) (*m019User, error) {
	var u m019User
	err := r.db.QueryRowContext(ctx,
		`select id, name, email from m019_users where id = $1`, id).Scan(&u.ID, &u.Name, &u.Email)
	if err != nil {
		return nil, m019TranslateError(err) // sql.ErrNoRows becomes m009ErrNotFound
	}
	return &u, nil
}

func (r *m019PostgresRepo) List(ctx context.Context) ([]m019User, error) {
	rows, err := r.db.QueryContext(ctx, `select id, name, email from m019_users order by id`)
	if err != nil {
		return nil, m019TranslateError(err)
	}
	defer rows.Close()

	var out []m019User
	for rows.Next() {
		var u m019User
		if err := rows.Scan(&u.ID, &u.Name, &u.Email); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *m019PostgresRepo) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `delete from m019_users where id = $1`, id)
	if err != nil {
		return m019TranslateError(err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return m009ErrNotFound // deleting nothing is a domain-level "not found"
	}
	return nil
}

// --- The in-memory implementation: the whole test double, in thirty lines ---

type m019MemoryRepo struct {
	mu     sync.RWMutex
	nextID int64
	users  map[int64]m019User
}

func m019NewMemoryRepo() *m019MemoryRepo {
	return &m019MemoryRepo{users: map[int64]m019User{}}
}

func (r *m019MemoryRepo) Create(ctx context.Context, u *m019User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, existing := range r.users {
		if existing.Email == u.Email {
			return m019ErrDuplicate // the same domain error the SQL version returns
		}
	}
	r.nextID++
	u.ID = r.nextID
	r.users[u.ID] = *u
	return nil
}

func (r *m019MemoryRepo) ByID(ctx context.Context, id int64) (*m019User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return nil, m009ErrNotFound
	}
	return &u, nil
}

func (r *m019MemoryRepo) List(ctx context.Context) ([]m019User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]m019User, 0, len(r.users))
	for _, u := range r.users {
		out = append(out, u)
	}
	slices.SortFunc(out, func(a, b m019User) int { return cmp.Compare(a.ID, b.ID) })
	return out, nil
}

func (r *m019MemoryRepo) Delete(ctx context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.users[id]; !ok {
		return m009ErrNotFound
	}
	delete(r.users, id)
	return nil
}

// Compile-time proof that both satisfy the interface (module 008, Section 1).
var (
	_ m019UserRepository = (*m019PostgresRepo)(nil)
	_ m019UserRepository = (*m019MemoryRepo)(nil)
)

// m019ExerciseRepository is the CONTRACT, run identically against any implementation.
func m019ExerciseRepository(ctx context.Context, repo m019UserRepository, label string) {
	fmt.Printf("  --- %s ---\n", label)

	u := &m019User{Name: "Repo Ada", Email: "repo-ada@example.com"}
	if err := repo.Create(ctx, u); err != nil {
		fmt.Printf("    Create: %v\n", err)
		return
	}
	fmt.Printf("    Create   -> id=%d\n", u.ID)

	// The duplicate is the same DOMAIN error from both implementations.
	dup := &m019User{Name: "Repo Dup", Email: "repo-ada@example.com"}
	fmt.Printf("    Create duplicate -> errors.Is(err, m019ErrDuplicate)=%t\n",
		errors.Is(repo.Create(ctx, dup), m019ErrDuplicate))

	found, err := repo.ByID(ctx, u.ID)
	fmt.Printf("    ByID(%d)  -> %+v (err=%v)\n", u.ID, m019Deref(found), err)

	_, err = repo.ByID(ctx, 999999)
	fmt.Printf("    ByID(999999) -> errors.Is(err, m009ErrNotFound)=%t  <- NOT sql.ErrNoRows\n",
		errors.Is(err, m009ErrNotFound))

	list, err := repo.List(ctx)
	fmt.Printf("    List     -> %d user(s) (err=%v)\n", len(list), err)

	fmt.Printf("    Delete(%d) -> %v\n", u.ID, repo.Delete(ctx, u.ID))
	fmt.Printf("    Delete(%d) again -> errors.Is(err, m009ErrNotFound)=%t\n",
		u.ID, errors.Is(repo.Delete(ctx, u.ID), m009ErrNotFound))
}

func m019Repository(ctx context.Context, db *sql.DB) {
	fmt.Println("\n--- Section 7: The Repository Pattern ---")

	fmt.Println("  one interface, two implementations, one identical exercise:")

	// This half always runs, database or not - which is the entire point.
	m019ExerciseRepository(ctx, m019NewMemoryRepo(), "in-memory (no database needed)")

	if db == nil {
		fmt.Println("  --- PostgreSQL: skipped, no database ---")
	} else {
		if err := m019Reset(ctx, db); err == nil {
			m019ExerciseRepository(ctx, &m019PostgresRepo{db: db}, "PostgreSQL")
		}
	}

	fmt.Println()
	fmt.Println("  both returned the SAME domain errors, so the caller never learns which one")
	fmt.Println("  it is talking to - and the unit tests never need Docker")
	fmt.Println("  module 020 implements this same interface a third time, with GORM")
	fmt.Println("  what this pattern must NOT hide is transactions: expose WithTx, or pass a")
	fmt.Println("  transaction handle explicitly - see Section 6")
}

// =================================================================================================
// Helpers
// =================================================================================================

// m019Reset drops and recreates the demo table, so the module is idempotent.
func m019Reset(ctx context.Context, db *sql.DB) error {
	const schema = `
		drop table if exists m019_users;
		create table m019_users (
			id       bigserial primary key,
			name     varchar(100) not null,
			email    varchar(100) not null unique,
			nickname varchar(100)
		);`
	if _, err := db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}
	return nil
}

// m019Shorten trims a driver error to its first line, so the output stays readable.
func m019Shorten(err error) string {
	if err == nil {
		return "<nil>"
	}
	line, _, _ := strings.Cut(err.Error(), "\n")
	if len(line) > 96 {
		line = line[:96] + "..."
	}
	return line
}

func m019Deref(u *m019User) m019User {
	if u == nil {
		return m019User{}
	}
	return *u
}

// Run019 runs every section of module 019 in order.
func Run019() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := m019Connect()
	if err != nil {
		fmt.Println("NOTE: PostgreSQL is not reachable, so the live sections are skipped.")
		fmt.Printf("      %v\n", m019Shorten(err))
		fmt.Println("      Start it with:  docker compose up -d")
		fmt.Printf("      DSN: %s\n", m019DSN())
		fmt.Println("      (or set M019_DSN to point somewhere else)")
		fmt.Println()
	} else {
		defer db.Close() // the pool is closed at shutdown, after everything that uses it
		fmt.Fprintln(os.Stdout, "PostgreSQL is reachable; running every section against it.")
		fmt.Println()
	}

	m019PoolBasics(db)
	m019PoolTuning(db)
	m019QueryingAndScanning(ctx, db)
	m019ParametersAndInjection(ctx, db)
	m019NullsAndErrors(ctx, db)
	m019Transactions(ctx, db)
	m019Repository(ctx, db)
}

package basics

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

/*
# Module 020 — GORM

Module 019 wrote SQL by hand. **GORM** is the other end of the spectrum: an object-relational
mapper that generates the SQL from Go structs. It is by far the most widely used database library
in the Go ecosystem, and it is genuinely useful — but it is also the library people most often
regret, usually because they reached for it before deciding what they wanted from it.

This module builds the **same repository interface as module 019** on GORM, so the two are directly
comparable: same operations, same domain errors, same output.

It needs the PostgreSQL from `docker-compose.yml`:

	docker compose up -d

As in 019, an unreachable database means the live sections are skipped and `go run . all` still
succeeds. Set `M019_DSN` to point elsewhere.
*/

// =================================================================================================
// Section 1: ORM or database/sql?
// =================================================================================================

/*
## ORM or database/sql?

What GORM gives you:

  - **no scanning code** — a struct in, a struct out, with no `rows.Scan(&a, &b, &c)` to keep in
    sync with the `SELECT` list
  - **schema migration from the struct** — `AutoMigrate` creates and alters tables
  - **associations** — load a user and their orders in one call, with the join written for you
  - **hooks** — `BeforeCreate`, `AfterFind`, and so on, for cross-cutting concerns
  - **soft deletes, timestamps, optimistic locking** — conventions you would otherwise hand-roll
  - **portability** — the same code against PostgreSQL, MySQL, SQLite and SQL Server

What it costs:

  - **the SQL becomes implicit.** A method chain that looks harmless can emit a query with a
    Cartesian join. You must be willing to read the generated SQL — `db.Debug()` prints it.
  - **the N+1 problem is one forgotten `Preload` away** (Section 5), and it will not fail, it will
    just be slow in production
  - **reflection everywhere** means errors move from compile time to run time: a typo in
    `Where("nmae = ?", x)` compiles perfectly
  - **another layer to learn**, with its own conventions, its own gotchas, and its own upgrade path
  - **`ErrRecordNotFound` is returned by `First` but not by `Find`**, which surprises everyone once

**When to use which**

Reach for **`database/sql`** when the SQL is the interesting part: reporting, analytics, anything
with window functions or CTEs, or a service whose performance profile you must control precisely.

Reach for **GORM** when the persistence is boring CRUD over a normalised schema, when you have many
entities that all behave the same way, and when development speed matters more than the last 20% of
query control.

A pragmatic middle ground is common and sensible: **GORM for CRUD, raw SQL for the three queries
that matter**. GORM supports `db.Raw(...).Scan(...)` precisely so you do not have to choose.
`sqlc` (generate type-safe Go from SQL) and `sqlx` (a thin `database/sql` extension) occupy the
same middle ground from the other direction.
*/

func m020ORMOrSQL() {
	fmt.Println("--- Section 1: ORM or database/sql? ---")

	fmt.Println("  GORM gives you:   no scan code, migrations from structs, associations, hooks,")
	fmt.Println("                    soft deletes, timestamps, cross-database portability")
	fmt.Println("  GORM costs you:   implicit SQL, the N+1 trap, run-time errors instead of")
	fmt.Println("                    compile-time ones, and another layer to learn")
	fmt.Println()
	fmt.Println("  database/sql when the SQL is the interesting part - reporting, analytics,")
	fmt.Println("    window functions, CTEs, or precise control over the query plan")
	fmt.Println("  GORM when persistence is boring CRUD over many similar entities")
	fmt.Println("  the common middle ground: GORM for CRUD, db.Raw for the three queries that matter")
	fmt.Println()
	fmt.Println("  compare side by side:")
	fmt.Println("    database/sql:  err := db.QueryRowContext(ctx,")
	fmt.Println("                       `select id, name, email from users where id = $1`, id).")
	fmt.Println("                       Scan(&u.ID, &u.Name, &u.Email)")
	fmt.Println("    GORM:          err := db.WithContext(ctx).First(&u, id).Error")
	fmt.Println("  the second is shorter; the first is the one you can predict the cost of")
}

// =================================================================================================
// Section 2: Models, Tags and Conventions
// =================================================================================================

/*
## Models, Tags and Conventions

GORM is **convention over configuration**, and knowing the conventions is most of knowing GORM:

  - the **table name** is the snake_case *plural* of the struct name: `Product` → `products`.
    Override with a `TableName() string` method, which is what this module does to keep its tables
    namespaced.
  - the **column name** is the snake_case field name: `CreatedAt` → `created_at`
  - the field named **`ID`** is the primary key, and an integer `ID` is auto-increment
  - `CreatedAt` and `UpdatedAt`, if present, are **maintained automatically**
  - `DeletedAt gorm.DeletedAt` turns on **soft deletes** (Section 4)

**`gorm.Model`** is a shorthand struct embedding `ID uint`, `CreatedAt`, `UpdatedAt` and
`DeletedAt`. Embedding it is idiomatic GORM — but note it commits you to soft deletes and to a
`uint` key, so many projects declare the four fields explicitly instead.

The `gorm:"..."` **struct tag** configures the rest, semicolon-separated:

	type Product struct {
	    ID        uint    `gorm:"primaryKey"`
	    Code      string  `gorm:"size:32;uniqueIndex;not null"`
	    Name      string  `gorm:"size:200;not null;index"`
	    Price     int64   `gorm:"not null;default:0;check:price >= 0"`
	    Slug      string  `gorm:"->"`                      // read-only
	    Secret    string  `gorm:"-"`                       // ignored entirely
	    CreatedAt time.Time
	}

Common options: `primaryKey`, `autoIncrement`, `size:N`, `type:jsonb`, `not null`, `default:X`,
`uniqueIndex`, `index`, `check:...`, `column:name`, `embedded`, `->` (read-only), `<-` (write-only),
`-` (ignore).

Tags are **strings**, so a typo is silent (module 007, Section 5) — `gorm:"uniqeIndex"` creates no
index and reports nothing.
*/

// m020User is the GORM model matching module 019's m019User, with explicit timestamp fields
// rather than gorm.Model, so nothing is hidden.
type m020User struct {
	ID        int64          `gorm:"primaryKey;autoIncrement"`
	Name      string         `gorm:"size:100;not null"`
	Email     string         `gorm:"size:100;not null;uniqueIndex"`
	Nickname  *string        `gorm:"size:100"` // a pointer, so NULL is representable (019 §5)
	CreatedAt time.Time      // maintained automatically
	UpdatedAt time.Time      // maintained automatically
	DeletedAt gorm.DeletedAt `gorm:"index"` // presence of this field enables SOFT DELETE
	Orders    []m020Order    `gorm:"foreignKey:UserID"`
	Tags      []m020Tag      `gorm:"many2many:m020_user_tags"` // the inverse of m020Tag.Users
	internal  string         // unexported: invisible to GORM, as to every reflective library
}

// TableName overrides the `m020_users` GORM would derive, keeping the course's tables namespaced.
func (m020User) TableName() string { return "m020_users" }

type m020Order struct {
	ID        int64 `gorm:"primaryKey;autoIncrement"`
	UserID    int64 `gorm:"not null;index"`
	Item      string
	Amount    int64 `gorm:"not null;default:0;check:amount >= 0"`
	CreatedAt time.Time
}

func (m020Order) TableName() string { return "m020_orders" }

// m020Tag and the join table demonstrate many2many.
type m020Tag struct {
	ID    int64      `gorm:"primaryKey;autoIncrement"`
	Label string     `gorm:"size:50;not null;uniqueIndex"`
	Users []m020User `gorm:"many2many:m020_user_tags"`
}

func (m020Tag) TableName() string { return "m020_tags" }

func m020ModelsAndTags() {
	fmt.Println("\n--- Section 2: Models, Tags and Conventions ---")

	fmt.Println("  conventions: Product -> products | CreatedAt -> created_at | ID is the key")
	fmt.Println("  CreatedAt/UpdatedAt are maintained automatically; DeletedAt enables soft delete")
	fmt.Println("  gorm.Model embeds all four - convenient, but it commits you to soft deletes")
	fmt.Println("  and a uint key, so this module declares them explicitly instead")
	fmt.Println()
	fmt.Println("  this module's models:")
	fmt.Println("    m020User  -> table m020_users  (TableName() overrides the derived name)")
	fmt.Println("    m020Order -> table m020_orders (belongs to a user)")
	fmt.Println("    m020Tag   -> table m020_tags   (many2many with users)")
	fmt.Println()
	fmt.Println("  tag options: primaryKey autoIncrement size:N type:jsonb not null default:X")
	fmt.Println("               uniqueIndex index check:... column:name embedded")
	fmt.Println("               `->` read-only   `<-` write-only   `-` ignore entirely")
	fmt.Println("  the tag is a STRING: `gorm:\"uniqeIndex\"` creates no index and says nothing")
	fmt.Println("  an unexported field is invisible to GORM, as to every reflective library")
}

// =================================================================================================
// Section 3: Connecting and AutoMigrate
// =================================================================================================

/*
## Connecting and AutoMigrate

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})

- `gorm.Open` returns a **`*gorm.DB`**, which is *not* a connection — it is a statement builder
  wrapping a `*sql.DB`. **`db.DB()`** reaches the underlying pool, which is where you apply the
  tuning from module 019, Section 2. GORM does not do it for you.
- **`gorm.Config`** matters more than it looks:
    - `Logger` — the default logs slow queries and errors to stdout. Replace it with something
      routed through `slog`, or silence it.
    - `PrepareStmt: true` — cache prepared statements; a straightforward win for repeated queries.
    - `SkipDefaultTransaction: true` — GORM wraps every single `Create`/`Update`/`Delete` in its
      own transaction. Turning that off is measurably faster when you do not need it.
    - `NamingStrategy` — override the pluralisation and casing rules.
- **`db.WithContext(ctx)`** is how a context reaches the query. It is easy to forget, and without
  it a cancelled request keeps querying — the same failure as the non-context `database/sql` calls.

### AutoMigrate, and its limits

`db.AutoMigrate(&User{}, &Order{})` creates tables, adds missing columns, and creates missing
indexes and constraints. What it deliberately **will not do**:

  - **drop or rename** a column — an unused column stays forever
  - **change a column's type** in a way that could lose data
  - **anything with data in it**: backfills, splits, merges

So `AutoMigrate` is excellent for development and for the first deploy, and **insufficient as a
production migration strategy**. Real schema changes need versioned, reviewable, reversible
migrations — `golang-migrate`, `goose`, `atlas` or `dbmate`. The usual arrangement is
`AutoMigrate` locally, versioned migrations in CI and production.
*/

func m020Connect(ctx context.Context) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(m019DSN()), &gorm.Config{
		Logger:                 logger.Default.LogMode(logger.Silent), // quiet, for reproducible output
		PrepareStmt:            true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		return nil, err
	}

	// GORM does not tune the pool - reach through to the *sql.DB and do it yourself.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	m019Tune(sqlDB) // the very same function module 019 uses

	pingCtx, cancel := context.WithTimeout(ctx, m019PingTimeout)
	defer cancel()
	if err := sqlDB.PingContext(pingCtx); err != nil {
		return nil, err
	}
	return db, nil
}

func m020ConnectingAndMigrating(ctx context.Context, db *gorm.DB) {
	fmt.Println("\n--- Section 3: Connecting and AutoMigrate ---")

	fmt.Println("  gorm.Open returns a statement builder, not a connection")
	fmt.Println("  db.DB() reaches the underlying *sql.DB - tune the pool there, GORM will not")
	fmt.Println("  gorm.Config worth setting: Logger, PrepareStmt, SkipDefaultTransaction")
	fmt.Println("  db.WithContext(ctx) is how the context reaches the query - easy to forget")

	if db == nil {
		fmt.Println("  (skipped: no database)")
		return
	}

	// gorm.Open returns (*gorm.DB, error) like every other constructor that can fail:
	//	db := gorm.Open(postgres.Open(m019DSN())) // ERROR: assignment mismatch: 1 variable but gorm.Open returns 2 values

	// Start from a clean slate so the module is idempotent. Schema changes run on a session with
	// PrepareStmt OFF: a statement cached before DropTable/AutoMigrate would otherwise make
	// Postgres fail with "cached plan must not change result type" when it is re-run.
	migrate := db.Session(&gorm.Session{PrepareStmt: false, Context: ctx})
	_ = migrate.Migrator().DropTable("m020_user_tags", &m020Order{}, &m020Tag{}, &m020User{})

	if err := migrate.AutoMigrate(&m020User{}, &m020Order{}, &m020Tag{}); err != nil {
		fmt.Println("  AutoMigrate failed:", err)
		return
	}
	fmt.Println("  AutoMigrate created m020_users, m020_orders, m020_tags and the join table")

	// Inspect what it built.
	migrator := db.Migrator()
	cols, _ := migrator.ColumnTypes(&m020User{})
	names := make([]string, 0, len(cols))
	for _, c := range cols {
		names = append(names, c.Name())
	}
	fmt.Printf("  m020_users columns: %v\n", names)
	fmt.Printf("  has the unique index on email: %t\n", migrator.HasIndex(&m020User{}, "Email"))

	sqlDB, _ := db.DB()
	s := sqlDB.Stats()
	fmt.Printf("  underlying pool: maxOpen=%d open=%d idle=%d\n",
		s.MaxOpenConnections, s.OpenConnections, s.Idle)

	fmt.Println("  AutoMigrate will NOT drop or rename a column, change a type destructively,")
	fmt.Println("  or backfill data - so it is a development tool, not a migration strategy.")
	fmt.Println("  Production needs versioned migrations: golang-migrate, goose, atlas, dbmate")
}

// =================================================================================================
// Section 4: CRUD and Soft Deletes
// =================================================================================================

/*
## CRUD and Soft Deletes

	db.Create(&user)                       // INSERT; fills user.ID, CreatedAt, UpdatedAt
	db.First(&user, 1)                     // SELECT ... WHERE id = 1 ORDER BY id LIMIT 1
	db.First(&user, "email = ?", email)    // with a condition
	db.Find(&users)                        // SELECT ... (all rows)
	db.Save(&user)                         // UPDATE every column (or INSERT if the key is zero)
	db.Model(&user).Updates(map[string]any{"name": "x"})  // UPDATE only these
	db.Delete(&user)                       // soft delete if DeletedAt is present, else DELETE

Every call returns a `*gorm.DB` carrying **`.Error`** and **`.RowsAffected`**. Nothing is returned
as a normal Go `(value, error)` pair, so the error is easy to drop — check `.Error` every time.

### The traps, in the order people hit them

 1. **`First` returns `gorm.ErrRecordNotFound`; `Find` does not.** `Find` on an empty result is a
    success with `RowsAffected == 0` and an untouched slice. Two different idioms for "was anything
    there", and mixing them up is the most common GORM bug.
 2. **`Updates` with a struct skips zero values.** Setting a field to `0`, `""` or `false` does
    nothing, because GORM cannot distinguish "set to zero" from "not set" — exactly the zero-value
    ambiguity from module 001a, Section 4. Use a **`map[string]any`**, or `Select` the columns
    explicitly.
 3. **`Save` writes every column**, including ones you did not intend to change, which silently
    clobbers a concurrent update. Prefer `Updates`.
 4. **A `Delete` or `Update` with no condition is refused** (`ErrMissingWhereClause`) — a guard
    against `DELETE FROM users` with no `WHERE`. `db.Where("1=1").Delete(...)` overrides it, which
    should feel appropriately uncomfortable.

### Soft deletes

With a `DeletedAt gorm.DeletedAt` field, `Delete` sets the timestamp instead of removing the row,
and **every subsequent query silently adds `WHERE deleted_at IS NULL`**. That is convenient and it
is a trap:

  - `Unscoped()` sees the soft-deleted rows, and `Unscoped().Delete(...)` really removes them
  - **a `uniqueIndex` still applies to soft-deleted rows**, so re-creating a "deleted" user with the
    same email fails with a duplicate-key error. This surprises everyone. The fix is a partial
    index (`WHERE deleted_at IS NULL`), which `AutoMigrate` will not create for you.
*/

func m020CRUD(ctx context.Context, db *gorm.DB) {
	fmt.Println("\n--- Section 4: CRUD and Soft Deletes ---")

	if db == nil {
		fmt.Println("  (skipped: no database)")
		return
	}
	tx := db.WithContext(ctx)

	// Create fills ID and the timestamps.
	u := m020User{Name: "Ada", Email: "ada@example.com"}
	if err := tx.Create(&u).Error; err != nil {
		fmt.Println("  create failed:", err)
		return
	}
	fmt.Printf("  Create      -> id=%d createdAt set=%t\n", u.ID, !u.CreatedAt.IsZero())

	if err := tx.Create(&m020User{Name: "Alan", Email: "alan@example.com"}).Error; err != nil {
		fmt.Println("  create failed:", err)
	}

	// First vs Find on a MISSING row - the trap.
	var missing m020User
	firstErr := tx.First(&missing, "email = ?", "nobody@example.com").Error
	fmt.Printf("  First on nothing -> errors.Is(err, gorm.ErrRecordNotFound)=%t\n",
		errors.Is(firstErr, gorm.ErrRecordNotFound))

	var none []m020User
	res := tx.Find(&none, "email = ?", "nobody@example.com")
	fmt.Printf("  Find  on nothing -> err=%v rowsAffected=%d len=%d  <- NOT an error\n",
		res.Error, res.RowsAffected, len(none))

	// Updates with a struct silently skips zero values.
	var target m020User
	_ = tx.First(&target, u.ID).Error
	_ = tx.Model(&target).Updates(m020User{Name: "Ada Lovelace", Nickname: nil}).Error
	var afterStruct m020User
	_ = tx.First(&afterStruct, u.ID).Error
	fmt.Printf("  Updates(struct) set the name to %q\n", afterStruct.Name)

	// The same thing with a zero value: ignored.
	_ = tx.Model(&afterStruct).Updates(m020User{Name: ""}).Error
	var afterZero m020User
	_ = tx.First(&afterZero, u.ID).Error
	fmt.Printf("  Updates(struct{Name:\"\"}) -> name is still %q  <- the zero value was SKIPPED\n",
		afterZero.Name)

	// A map says what it means.
	_ = tx.Model(&afterZero).Updates(map[string]any{"name": ""}).Error
	var afterMap m020User
	_ = tx.First(&afterMap, u.ID).Error
	fmt.Printf("  Updates(map{\"name\": \"\"}) -> name is now %q  <- use a map for zero values\n",
		afterMap.Name)
	_ = tx.Model(&afterMap).Updates(map[string]any{"name": "Ada"}).Error

	// A delete with no condition is refused.
	blockErr := tx.Delete(&m020User{}).Error
	fmt.Printf("  Delete with no WHERE -> %v\n", m019Shorten(blockErr))

	// --- Soft delete ---
	_ = tx.Delete(&m020User{}, u.ID).Error
	var visible int64
	_ = tx.Model(&m020User{}).Where("id = ?", u.ID).Count(&visible).Error
	var withDeleted int64
	_ = tx.Unscoped().Model(&m020User{}).Where("id = ?", u.ID).Count(&withDeleted).Error
	fmt.Printf("  after Delete: normal query sees %d, Unscoped sees %d (the row is still there)\n",
		visible, withDeleted)

	// The uniqueIndex trap: the soft-deleted row still occupies the email.
	reuse := tx.Create(&m020User{Name: "Ada Again", Email: "ada@example.com"}).Error
	fmt.Printf("  re-creating with the same email -> %v\n", m019Shorten(reuse))
	fmt.Println("  a uniqueIndex applies to SOFT-DELETED rows too - the fix is a partial index")
	fmt.Println("  (WHERE deleted_at IS NULL), which AutoMigrate will not create for you")

	// Unscoped().Delete really removes it.
	_ = tx.Unscoped().Delete(&m020User{}, u.ID).Error
	_ = tx.Unscoped().Model(&m020User{}).Where("id = ?", u.ID).Count(&withDeleted).Error
	fmt.Printf("  Unscoped().Delete really removed it: rows with that id = %d\n", withDeleted)
}

// =================================================================================================
// Section 5: Querying, Preload and the N+1 Problem
// =================================================================================================

/*
## Querying, Preload and the N+1 Problem

The query builder chains, and each call returns a new `*gorm.DB`:

	db.Where("price > ?", 100).Where("stock > ?", 0).Order("price desc").Limit(10).Find(&items)
	db.Where(map[string]any{"status": "active"}).Find(&items)
	db.Select("id", "name").Find(&items)
	db.Joins("JOIN orders ON orders.user_id = users.id").Where("orders.amount > ?", 500).Find(&users)
	db.Raw("select * from users where id = ?", id).Scan(&u)   // when the chain fights you

**A reused `*gorm.DB` accumulates conditions.** Once a chain method such as `Where` has been called
on a `*gorm.DB`, further calls on that same value keep adding to it — a classic source of "why is
this query filtering by something I removed". Only a fresh value is safe to build on: the root
`db`, `db.WithContext(ctx)`, or `db.Session(&gorm.Session{})` each start a new, independent chain.

### The N+1 problem

Loading 100 users and then their orders in a loop issues **101 queries**. It works, it passes tests
with three rows, and it collapses in production. GORM's answer is **`Preload`**:

	db.Find(&users)                    // 1 query; users[i].Orders is empty
	for _, u := range users {
	    db.Where("user_id = ?", u.ID).Find(&u.Orders)   // N more queries — the bug
	}

	db.Preload("Orders").Find(&users)  // 2 queries total: users, then all orders WHERE user_id IN (...)

  - `Preload` issues **one extra query per association**, using `IN`. This is what you want by
    default.
  - `Joins("Orders")` uses a single `LEFT JOIN` instead — better for a `belongs to`, wasteful for a
    `has many`, because every parent row is repeated per child.
  - Nested: `Preload("Orders.Items")`. Conditional: `Preload("Orders", "amount > ?", 100)`.
  - `Preload(clause.Associations)` loads every association one level deep.

**There is no compile-time protection against N+1.** The only defence is reading the generated SQL —
`db.Debug()` prints every statement — and watching query counts in a load test.
*/

func m020QueryingAndPreload(ctx context.Context, db *gorm.DB) {
	fmt.Println("\n--- Section 5: Querying, Preload and the N+1 Problem ---")

	if db == nil {
		fmt.Println("  (skipped: no database)")
		return
	}
	tx := db.WithContext(ctx)

	// Seed a few users, each with orders.
	seed := []m020User{
		{Name: "Buyer One", Email: "one@example.com"},
		{Name: "Buyer Two", Email: "two@example.com"},
		{Name: "Buyer Three", Email: "three@example.com"},
	}
	for i := range seed {
		if err := tx.Create(&seed[i]).Error; err != nil {
			continue
		}
		for j := range 3 {
			_ = tx.Create(&m020Order{
				UserID: seed[i].ID,
				Item:   fmt.Sprintf("item-%d-%d", i+1, j+1),
				Amount: int64((i + 1) * (j + 1) * 100),
			}).Error
		}
	}

	// Chained conditions.
	var expensive []m020Order
	_ = tx.Where("amount > ?", 400).Order("amount desc").Limit(3).Find(&expensive).Error
	fmt.Printf("  Where+Order+Limit -> %d orders, top amount=%d\n",
		len(expensive), m020TopAmount(expensive))

	// Select only what you need.
	var slim []m020User
	_ = tx.Select("id", "name").Where("email like ?", "%@example.com").Find(&slim).Error
	fmt.Printf("  Select(\"id\",\"name\") -> %d users, first email is %q (not loaded)\n",
		len(slim), m020FirstEmail(slim))

	// --- N+1, counted ---
	// Callbacks are registered on the *gorm.DB's shared callback table, which is GLOBAL to every
	// session derived from that connection - this counter stays installed after the demo.
	var counted int
	countingDB := tx.Session(&gorm.Session{}).Callback().Query().After("gorm:query")
	if err := countingDB.Register("m020:count", func(d *gorm.DB) { counted++ }); err != nil {
		fmt.Println("  registering the query callback failed:", err)
	}

	var users []m020User
	counted = 0
	_ = tx.Find(&users).Error
	for i := range users {
		_ = tx.Where("user_id = ?", users[i].ID).Find(&users[i].Orders).Error
	}
	naive := counted

	counted = 0
	var preloaded []m020User
	_ = tx.Preload("Orders").Find(&preloaded).Error
	withPreload := counted

	_ = tx.Callback().Query().Remove("m020:count")

	fmt.Printf("  loading %d users and their orders:\n", len(users))
	fmt.Printf("    naive loop  -> %d queries  (1 + N)\n", naive)
	fmt.Printf("    Preload     -> %d queries  (users, then all orders WHERE user_id IN (...))\n",
		withPreload)
	fmt.Printf("    both loaded the same data: %d and %d orders in total\n",
		m020TotalOrders(users), m020TotalOrders(preloaded))

	// Joins, for comparison.
	var joined []m020User
	_ = tx.Joins("JOIN m020_orders ON m020_orders.user_id = m020_users.id").
		Where("m020_orders.amount > ?", 500).
		Distinct().Find(&joined).Error
	fmt.Printf("  Joins + Distinct -> %d users with an order over 500\n", len(joined))

	// Raw, for when the chain fights you.
	type report struct {
		Name  string
		Total int64
	}
	var rows []report
	_ = tx.Raw(`select u.name, coalesce(sum(o.amount),0) as total
	            from m020_users u left join m020_orders o on o.user_id = u.id
	            where u.deleted_at is null
	            group by u.name order by total desc limit 3`).Scan(&rows).Error
	fmt.Println("  db.Raw for an aggregate the builder would fight you over:")
	for _, r := range rows {
		fmt.Printf("    %-12s total=%d\n", r.Name, r.Total)
	}

	fmt.Println("  nothing warns you about N+1 at compile time - read the SQL with db.Debug()")
}

func m020TotalOrders(users []m020User) int {
	total := 0
	for _, u := range users {
		total += len(u.Orders)
	}
	return total
}

func m020TopAmount(orders []m020Order) int64 {
	if len(orders) == 0 {
		return 0
	}
	return orders[0].Amount
}

func m020FirstEmail(users []m020User) string {
	if len(users) == 0 {
		return ""
	}
	return users[0].Email
}

// =================================================================================================
// Section 6: Associations
// =================================================================================================

/*
## Associations

GORM infers relationships from field types and naming conventions:

	// belongs to: Order has a UserID, so it belongs to one User
	type Order struct { UserID int64; User User }

	// has many: User has a slice of Orders keyed by Order.UserID
	type User struct { Orders []Order `gorm:"foreignKey:UserID"` }

	// has one: the same, singular
	type User struct { Profile Profile }

	// many to many: through a join table GORM creates and manages
	type User struct { Tags []Tag `gorm:"many2many:user_tags"` }

- The **foreign key is inferred** as `<OwnerType>ID`. `foreignKey` and `references` override it.
- **Creating a parent creates its children** by default (`FullSaveAssociations` extends this to
  updates). That is convenient and occasionally more writing than you intended — `Omit("Orders")`
  turns it off, and `db.Session(&gorm.Session{SkipHooks: true})` goes further.
- `db.Model(&user).Association("Tags").Append(&tag)` / `.Delete(...)` / `.Replace(...)` / `.Clear()`
  manipulate a relationship without loading the whole graph.
- **`constraint:OnDelete:CASCADE`** in the tag makes `AutoMigrate` emit a real foreign key with the
  cascade. Without it there is no referential integrity at the database level — GORM's associations
  are a Go-side convention, and the database will happily hold orphans.
- Deep object graphs are where ORMs earn their reputation for surprising queries. Load what you
  need, one level at a time, and read the SQL.
*/

func m020Associations(ctx context.Context, db *gorm.DB) {
	fmt.Println("\n--- Section 6: Associations ---")

	if db == nil {
		fmt.Println("  (skipped: no database)")
		return
	}
	tx := db.WithContext(ctx)

	// Creating a parent creates its children in the same operation.
	author := m020User{
		Name:  "Composer",
		Email: "composer@example.com",
		Orders: []m020Order{
			{Item: "nested-a", Amount: 10},
			{Item: "nested-b", Amount: 20},
		},
	}
	if err := tx.Create(&author).Error; err != nil {
		fmt.Println("  create failed:", err)
		return
	}
	fmt.Printf("  Create with nested Orders -> user id=%d, order ids=%v\n",
		author.ID, m020OrderIDs(author.Orders))
	fmt.Println("  the children were inserted automatically - Omit(\"Orders\") prevents that")

	// has many, loaded back.
	var loaded m020User
	_ = tx.Preload("Orders").First(&loaded, author.ID).Error
	fmt.Printf("  Preload(\"Orders\") -> %d orders\n", len(loaded.Orders))

	// many2many through the join table.
	tags := []m020Tag{{Label: "vip"}, {Label: "beta"}}
	for i := range tags {
		_ = tx.Where(m020Tag{Label: tags[i].Label}).FirstOrCreate(&tags[i]).Error
	}
	if err := tx.Model(&loaded).Association("Tags").Append(&tags); err != nil {
		fmt.Println("  association append failed:", err)
	}
	count := tx.Model(&loaded).Association("Tags").Count()
	fmt.Printf("  Association(\"Tags\").Append -> %d tags on the user\n", count)

	var tagged m020Tag
	_ = tx.Preload("Users").First(&tagged, "label = ?", "vip").Error
	fmt.Printf("  and from the other side: tag %q has %d user(s)\n", tagged.Label, len(tagged.Users))

	// Replace and Clear.
	firstTag := tags[:1]
	_ = tx.Model(&loaded).Association("Tags").Replace(&firstTag)
	fmt.Printf("  Association.Replace -> %d tag(s)\n", tx.Model(&loaded).Association("Tags").Count())
	_ = tx.Model(&loaded).Association("Tags").Clear()
	fmt.Printf("  Association.Clear   -> %d tag(s)\n", tx.Model(&loaded).Association("Tags").Count())

	fmt.Println("  without constraint:OnDelete:CASCADE there is NO database-level foreign key,")
	fmt.Println("  so the association is a Go-side convention and orphans are possible")
}

func m020OrderIDs(orders []m020Order) []int64 {
	out := make([]int64, len(orders))
	for i, o := range orders {
		out[i] = o.ID
	}
	return out
}

// =================================================================================================
// Section 7: Transactions and Hooks
// =================================================================================================

/*
## Transactions and Hooks

### Transactions

	// Managed: commits on nil, rolls back on any error or panic.
	err := db.Transaction(func(tx *gorm.DB) error {
	    if err := tx.Create(&a).Error; err != nil { return err }
	    return tx.Create(&b).Error
	})

	// Manual, mirroring module 019 §6:
	tx := db.Begin()
	defer func() { if r := recover(); r != nil { tx.Rollback() } }()
	...
	tx.Commit()

**Use the managed form.** It cannot leak a transaction, it rolls back on a panic, and there is no
`defer tx.Rollback()` to forget. Nested calls become **savepoints** automatically.

The one rule that matters: **inside the closure, use `tx`, never `db`.** A `db` call goes to a
different connection and is not part of the transaction — the same trap as module 019, Section 6,
and easier to fall into here because both variables are in scope.

`SkipDefaultTransaction: true` (Section 3) turns off the implicit single-statement transaction that
GORM otherwise wraps around every write.

### Hooks

`BeforeSave`, `BeforeCreate`, `AfterCreate`, `BeforeUpdate`, `AfterFind`, `BeforeDelete`, and their
`After` counterparts, are methods on the model:

	func (u *User) BeforeCreate(tx *gorm.DB) error { ... }

Returning an error **aborts the operation and rolls back the transaction**. Hooks are genuinely
useful for normalisation (lower-casing an email), derived fields (a slug), and audit columns.

They are also easy to regret: a hook runs on **every** save, including bulk operations and
migrations, it is invisible at the call site, and it makes the model depend on `*gorm.DB`. Business
logic in a hook is business logic nobody can find. Keep them small, pure and about the row itself.
*/

// BeforeCreate normalises the email. Small, pure, and about this row only.
func (u *m020User) BeforeCreate(tx *gorm.DB) error {
	u.Email = strings.ToLower(strings.TrimSpace(u.Email))
	if u.Name == "" {
		return errors.New("m020User: name must not be empty") // aborts and rolls back
	}
	return nil
}

func m020TransactionsAndHooks(ctx context.Context, db *gorm.DB) {
	fmt.Println("\n--- Section 7: Transactions and Hooks ---")

	if db == nil {
		fmt.Println("  (skipped: no database)")
		return
	}
	tx := db.WithContext(ctx)

	before := m020Count(ctx, db)

	// Managed transaction that commits.
	err := tx.Transaction(func(t *gorm.DB) error {
		if err := t.Create(&m020User{Name: "Tx A", Email: "tx-a@example.com"}).Error; err != nil {
			return err
		}
		return t.Create(&m020User{Name: "Tx B", Email: "tx-b@example.com"}).Error
	})
	afterCommit := m020Count(ctx, db)
	fmt.Printf("  committed transaction: err=%v rows %d -> %d\n", err, before, afterCommit)

	// Managed transaction that fails: everything is discarded.
	err = tx.Transaction(func(t *gorm.DB) error {
		if err := t.Create(&m020User{Name: "Tx C", Email: "tx-c@example.com"}).Error; err != nil {
			return err
		}
		return errors.New("deliberate failure after the first insert")
	})
	afterRollback := m020Count(ctx, db)
	fmt.Printf("  failed transaction:    err=%v\n", m019Shorten(err))
	fmt.Printf("  rows %d -> %d — the insert was rolled back automatically\n",
		afterCommit, afterRollback)

	// A panic inside the closure also rolls back.
	fmt.Printf("  a panic inside Transaction: %v\n", m005CatchPanic(func() {
		_ = tx.Transaction(func(t *gorm.DB) error {
			_ = t.Create(&m020User{Name: "Tx D", Email: "tx-d@example.com"}).Error
			panic("boom")
		})
	}))
	fmt.Printf("  rows after the panic: %d — still rolled back\n", m020Count(ctx, db))

	// --- Hooks ---
	messy := m020User{Name: "Hooked", Email: "  MiXeD@Example.COM  "}
	if err := tx.Create(&messy).Error; err == nil {
		fmt.Printf("  BeforeCreate normalised the email to %q\n", messy.Email)
	}

	invalid := m020User{Name: "", Email: "no-name@example.com"}
	fmt.Printf("  a hook returning an error aborts the write: %v\n",
		m019Shorten(tx.Create(&invalid).Error))

	fmt.Println("  use the MANAGED form: it cannot leak a transaction and rolls back on panic")
	fmt.Println("  inside the closure use `tx`, never `db` - `db` is a different connection")
	fmt.Println("  keep hooks small and about the row; business logic in a hook is unfindable")
}

func m020Count(ctx context.Context, db *gorm.DB) int64 {
	var n int64
	_ = db.WithContext(ctx).Model(&m020User{}).Count(&n).Error
	return n
}

// =================================================================================================
// Section 8: The Same Repository, on GORM
// =================================================================================================

/*
## The Same Repository, on GORM

Module 019 defined `m019UserRepository` and implemented it twice — once on PostgreSQL through
`database/sql`, once in memory. Here is a **third implementation on GORM**, satisfying the identical
interface and returning the identical domain errors.

That is the payoff of the pattern (module 019, Section 7): the storage engine is a swappable
detail. The application code, its tests and its error handling do not change at all, and the
`m019ExerciseRepository` function from module 019 runs against this implementation unmodified.

Note what the boundary is doing: `gorm.ErrRecordNotFound` and the driver's unique-violation error
are both translated into this package's own domain errors, so **`gorm` never escapes this file** —
exactly as `database/sql` never escaped module 019's repository.

Compare the three implementations side by side and the trade-off from Section 1 is concrete:

	database/sql   ~15 lines per method, explicit SQL, explicit scanning
	GORM           ~4 lines per method, generated SQL, reflection
	in-memory      ~6 lines per method, a map and a mutex
*/

type m020GormRepo struct{ db *gorm.DB }

func (r *m020GormRepo) Create(ctx context.Context, u *m019User) error {
	model := m020User{Name: u.Name, Email: u.Email}
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return m020TranslateError(err)
	}
	u.ID = model.ID
	return nil
}

func (r *m020GormRepo) ByID(ctx context.Context, id int64) (*m019User, error) {
	var model m020User
	if err := r.db.WithContext(ctx).First(&model, id).Error; err != nil {
		return nil, m020TranslateError(err)
	}
	return &m019User{ID: model.ID, Name: model.Name, Email: model.Email}, nil
}

func (r *m020GormRepo) List(ctx context.Context) ([]m019User, error) {
	var models []m020User
	if err := r.db.WithContext(ctx).Order("id").Find(&models).Error; err != nil {
		return nil, m020TranslateError(err)
	}
	out := make([]m019User, len(models))
	for i, m := range models {
		out[i] = m019User{ID: m.ID, Name: m.Name, Email: m.Email}
	}
	return out, nil
}

func (r *m020GormRepo) Delete(ctx context.Context, id int64) error {
	res := r.db.WithContext(ctx).Delete(&m020User{}, id)
	if res.Error != nil {
		return m020TranslateError(res.Error)
	}
	if res.RowsAffected == 0 {
		return m009ErrNotFound // Delete is not an error when nothing matched - Find's trap again
	}
	return nil
}

// Compile-time proof, exactly as in module 019.
var _ m019UserRepository = (*m020GormRepo)(nil)

// m020TranslateError keeps GORM from escaping this file.
func m020TranslateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return m009ErrNotFound
	}
	// String matching is a dependency-free stand-in. The real form is
	// `var pgErr *pgconn.PgError; errors.As(err, &pgErr) && pgErr.Code == "23505"`.
	if strings.Contains(err.Error(), "23505") || strings.Contains(err.Error(), "duplicate key") {
		return m019ErrDuplicate
	}
	return err
}

func m020Repository(ctx context.Context, db *gorm.DB) {
	fmt.Println("\n--- Section 8: The Same Repository, on GORM ---")

	if db == nil {
		fmt.Println("  (skipped: no database)")
		fmt.Println("  see module 019 §7 for the in-memory implementation, which needs no database")
		return
	}

	// A clean table, so the output matches module 019's exactly.
	_ = db.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate: true}).
		Unscoped().Delete(&m020Order{}).Error
	_ = db.WithContext(ctx).Session(&gorm.Session{AllowGlobalUpdate: true}).
		Unscoped().Delete(&m020User{}).Error

	// The SAME exercise function from module 019, unmodified.
	m019ExerciseRepository(ctx, &m020GormRepo{db: db}, "GORM (module 019's interface, third impl)")

	fmt.Println()
	fmt.Println("  that is module 019's m019ExerciseRepository, called without a single change")
	fmt.Println("  gorm.ErrRecordNotFound became m009ErrNotFound at the boundary, so `gorm`")
	fmt.Println("  never escapes this file - just as database/sql never escaped module 019's")
	fmt.Println()
	fmt.Println("  lines per method:  database/sql ~15 | GORM ~4 | in-memory ~6")
	fmt.Println("  the ORM is shorter; the hand-written SQL is the one you can predict")
}

// Run020 runs every section of module 020 in order.
func Run020() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db, err := m020Connect(ctx)
	if err != nil {
		fmt.Println("NOTE: PostgreSQL is not reachable, so the live sections are skipped.")
		fmt.Printf("      %v\n", m019Shorten(err))
		fmt.Println("      Start it with:  docker compose up -d")
		fmt.Printf("      DSN: %s\n", m019DSN())
		fmt.Println("      (or set M019_DSN to point somewhere else)")
		fmt.Println()
	} else {
		if sqlDB, err := db.DB(); err == nil {
			defer sqlDB.Close()
		}
		fmt.Println("PostgreSQL is reachable; running every section against it.")
		fmt.Println()
	}

	m020ORMOrSQL()
	m020ModelsAndTags()
	m020ConnectingAndMigrating(ctx, db)
	m020CRUD(ctx, db)
	m020QueryingAndPreload(ctx, db)
	m020Associations(ctx, db)
	m020TransactionsAndHooks(ctx, db)
	m020Repository(ctx, db)
}

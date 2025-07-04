# GORM with PostgreSQL

## Table of Contents
1. [Introduction](#introduction)
2. [Installation](#installation)
3. [PostgreSQL Connection](#postgresql-connection)
4. [Model Definition with PostgreSQL Types](#model-definition-with-postgresql-types)
5. [CRUD Operations](#crud-operations)
6. [PostgreSQL-Specific Features](#postgresql-specific-features)
7. [Querying](#querying)
8. [Associations](#associations)
9. [Migrations](#migrations)
10. [Indexes and Constraints](#indexes-and-constraints)
11. [Transactions](#transactions)
12. [Performance Optimization](#performance-optimization)
13. [Best Practices](#best-practices)

## Introduction

GORM is a powerful ORM library for Go that provides excellent support for PostgreSQL. This tutorial covers PostgreSQL-specific features and best practices when using GORM with PostgreSQL.

## Installation

Install GORM and the PostgreSQL driver:

```bash
go get -u gorm.io/gorm
go get -u gorm.io/driver/postgres
```

## PostgreSQL Connection

### Basic Connection

```go
package main

import (
    "log"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
    "gorm.io/gorm/logger"
)

func main() {
    // Connection string
    dsn := "host=localhost user=postgres password=yourpassword dbname=mydb port=5432 sslmode=disable TimeZone=Asia/Shanghai"
    
    // Open connection
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
        Logger: logger.Default.LogMode(logger.Info),
    })
    
    if err != nil {
        log.Fatal("Failed to connect to database:", err)
    }
    
    // Get generic database object sql.DB to configure connection pool
    sqlDB, err := db.DB()
    if err != nil {
        log.Fatal("Failed to get database:", err)
    }
    
    // SetMaxIdleConns sets the maximum number of connections in the idle connection pool
    sqlDB.SetMaxIdleConns(10)
    
    // SetMaxOpenConns sets the maximum number of open connections to the database
    sqlDB.SetMaxOpenConns(100)
    
    // SetConnMaxLifetime sets the maximum amount of time a connection may be reused
    sqlDB.SetConnMaxLifetime(time.Hour)
}
```

### Advanced Configuration

```go
// Using DSN with more options
dsn := "host=localhost user=postgres password=yourpassword dbname=mydb port=5432 sslmode=prefer connect_timeout=10 TimeZone=UTC"

// Custom PostgreSQL config
db, err := gorm.Open(postgres.New(postgres.Config{
    DSN:                  dsn,
    PreferSimpleProtocol: true, // disables implicit prepared statement usage
}), &gorm.Config{
    NamingStrategy: schema.NamingStrategy{
        TablePrefix:   "app_", // table name prefix
        SingularTable: false,  // use singular table name
    },
    NowFunc: func() time.Time {
        return time.Now().UTC()
    },
})
```

## Model Definition with PostgreSQL Types

### Basic Model with PostgreSQL Types

```go
import (
    "time"
    "github.com/lib/pq"
    "gorm.io/datatypes"
    "gorm.io/gorm"
)

type User struct {
    ID            uint           `gorm:"primaryKey"`
    UUID          string         `gorm:"type:uuid;default:gen_random_uuid()"`
    Username      string         `gorm:"type:varchar(100);uniqueIndex;not null"`
    Email         string         `gorm:"type:varchar(255);uniqueIndex;not null"`
    Age           int            `gorm:"type:integer;check:age > 0"`
    Balance       float64        `gorm:"type:decimal(10,2);default:0"`
    Tags          pq.StringArray `gorm:"type:text[]"`
    Metadata      datatypes.JSON `gorm:"type:jsonb"`
    Status        string         `gorm:"type:varchar(20);default:'active'"`
    IpAddress     string         `gorm:"type:inet"`
    MacAddress    string         `gorm:"type:macaddr"`
    Website       string         `gorm:"type:text"`
    Biography     string         `gorm:"type:text"`
    ProfilePic    []byte         `gorm:"type:bytea"`
    LastLoginAt   *time.Time     `gorm:"type:timestamptz"`
    CreatedAt     time.Time      `gorm:"type:timestamptz;default:CURRENT_TIMESTAMP"`
    UpdatedAt     time.Time      `gorm:"type:timestamptz;default:CURRENT_TIMESTAMP"`
    DeletedAt     gorm.DeletedAt `gorm:"type:timestamptz;index"`
}

// Example with PostgreSQL-specific constraints
type Product struct {
    ID          uint                   `gorm:"primaryKey"`
    Name        string                 `gorm:"type:varchar(255);not null"`
    SKU         string                 `gorm:"type:varchar(100);uniqueIndex"`
    Price       float64                `gorm:"type:decimal(10,2);check:price >= 0"`
    Stock       int                    `gorm:"type:integer;default:0;check:stock >= 0"`
    Categories  pq.StringArray         `gorm:"type:text[]"`
    Attributes  map[string]interface{} `gorm:"type:jsonb"`
    SearchVector string                `gorm:"type:tsvector"`
}
```

### PostgreSQL Enum Types

```go
// First, create the enum type in PostgreSQL
db.Exec("CREATE TYPE user_role AS ENUM ('admin', 'user', 'guest')")

type User struct {
    ID   uint   `gorm:"primaryKey"`
    Name string
    Role string `gorm:"type:user_role;default:'user'"`
}
```

## CRUD Operations

### Create with PostgreSQL Features

```go
// Create with JSONB data
user := User{
    Username: "john_doe",
    Email:    "john@example.com",
    Tags:     pq.StringArray{"developer", "golang", "postgres"},
    Metadata: datatypes.JSON([]byte(`{"city": "New York", "interests": ["coding", "reading"]}`)),
}
db.Create(&user)

// Bulk insert with COPY (more efficient for large datasets)
users := []User{
    {Username: "alice", Email: "alice@example.com"},
    {Username: "bob", Email: "bob@example.com"},
}
db.CreateInBatches(users, 1000)

// Insert with ON CONFLICT
db.Clauses(clause.OnConflict{
    Columns:   []clause.Column{{Name: "email"}},
    DoUpdates: clause.AssignmentColumns([]string{"username", "updated_at"}),
}).Create(&user)

// Using RETURNING clause
var newUser User
db.Clauses(clause.Returning{}).Create(&User{
    Username: "jane",
    Email:    "jane@example.com",
}).Scan(&newUser)
```

### Read with PostgreSQL Features

```go
// Query JSONB fields
var users []User
db.Where("metadata->>'city' = ?", "New York").Find(&users)
db.Where("metadata @> ?", `{"interests": ["coding"]}`).Find(&users)

// Query array fields
db.Where("? = ANY(tags)", "developer").Find(&users)
db.Where("tags && ?", pq.StringArray{"golang", "postgres"}).Find(&users)

// Full-text search
db.Where("to_tsvector('english', name || ' ' || biography) @@ plainto_tsquery('english', ?)", "golang developer").Find(&users)

// Using FOR UPDATE (row-level locking)
tx := db.Begin()
var user User
tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, 1)
// ... do something with user
tx.Commit()

// With CTE (Common Table Expressions)
db.Raw(`
    WITH active_users AS (
        SELECT * FROM users WHERE status = 'active'
    )
    SELECT * FROM active_users WHERE age > ?
`, 25).Scan(&users)
```

### Update with PostgreSQL Features

```go
// Update JSONB field
db.Model(&user).Update("metadata", gorm.Expr("metadata || ?", `{"verified": true}`))

// Append to array
db.Model(&user).Update("tags", gorm.Expr("array_append(tags, ?)", "newbie"))

// Remove from array
db.Model(&user).Update("tags", gorm.Expr("array_remove(tags, ?)", "newbie"))

// Increment with conflict handling
db.Model(&Product{}).
    Where("id = ?", 1).
    Update("stock", gorm.Expr("GREATEST(stock - ?, 0)", 5))

// Bulk update with CASE
db.Exec(`
    UPDATE products 
    SET price = CASE 
        WHEN category = 'electronics' THEN price * 1.1
        WHEN category = 'books' THEN price * 1.05
        ELSE price 
    END
    WHERE updated_at < NOW() - INTERVAL '30 days'
`)
```

## PostgreSQL-Specific Features

### Working with JSONB

```go
// Complex JSONB queries
type UserPreferences struct {
    Theme        string   `json:"theme"`
    Language     string   `json:"language"`
    Notifications bool    `json:"notifications"`
    Interests    []string `json:"interests"`
}

// Query nested JSONB
db.Where("metadata->>'theme' = ?", "dark").Find(&users)
db.Where("metadata->'notifications' = ?", "true").Find(&users)
db.Where("jsonb_array_length(metadata->'interests') > ?", 3).Find(&users)

// Update nested JSONB
db.Model(&user).Update("metadata", gorm.Expr("jsonb_set(metadata, '{theme}', ?)", `"light"`))

// JSONB aggregation
var result struct {
    TotalUsers int
    Themes     datatypes.JSON
}
db.Raw(`
    SELECT COUNT(*) as total_users, 
           jsonb_agg(DISTINCT metadata->>'theme') as themes
    FROM users
    WHERE metadata IS NOT NULL
`).Scan(&result)
```

### Full-Text Search

```go
// Create text search index
db.Exec("CREATE INDEX idx_products_search ON products USING GIN(to_tsvector('english', name || ' ' || description))")

// Search model
type Article struct {
    ID           uint   `gorm:"primaryKey"`
    Title        string `gorm:"type:text"`
    Content      string `gorm:"type:text"`
    SearchVector string `gorm:"type:tsvector"`
}

// Update search vector on save
func (a *Article) BeforeSave(tx *gorm.DB) error {
    tx.Statement.SetColumn("search_vector", 
        gorm.Expr("to_tsvector('english', ? || ' ' || ?)", a.Title, a.Content))
    return nil
}

// Perform full-text search
var articles []Article
db.Where("search_vector @@ plainto_tsquery('english', ?)", "golang tutorial").
    Order("ts_rank(search_vector, plainto_tsquery('english', ?)) DESC", "golang tutorial").
    Find(&articles)
```

### Window Functions

```go
type UserRank struct {
    ID       uint
    Username string
    Score    int
    Rank     int
}

var rankings []UserRank
db.Raw(`
    SELECT id, username, score,
           RANK() OVER (ORDER BY score DESC) as rank
    FROM users
    WHERE status = 'active'
`).Scan(&rankings)

// With partition
db.Raw(`
    SELECT id, username, department, salary,
           RANK() OVER (PARTITION BY department ORDER BY salary DESC) as dept_rank
    FROM employees
`).Scan(&results)
```

## Querying

### Advanced PostgreSQL Queries

```go
// Using DISTINCT ON
var latestUserActivities []Activity
db.Raw(`
    SELECT DISTINCT ON (user_id) *
    FROM activities
    ORDER BY user_id, created_at DESC
`).Scan(&latestUserActivities)

// Recursive CTE
var hierarchicalData []Category
db.Raw(`
    WITH RECURSIVE category_tree AS (
        SELECT id, name, parent_id, 0 as level
        FROM categories
        WHERE parent_id IS NULL
        
        UNION ALL
        
        SELECT c.id, c.name, c.parent_id, ct.level + 1
        FROM categories c
        JOIN category_tree ct ON c.parent_id = ct.id
    )
    SELECT * FROM category_tree
    ORDER BY level, name
`).Scan(&hierarchicalData)

// Using LATERAL joins
db.Raw(`
    SELECT u.*, recent_orders.*
    FROM users u
    LEFT JOIN LATERAL (
        SELECT * FROM orders
        WHERE user_id = u.id
        ORDER BY created_at DESC
        LIMIT 3
    ) recent_orders ON true
`).Scan(&results)
```

## Associations

### PostgreSQL-Optimized Associations

```go
// One-to-Many with foreign key constraints
type Author struct {
    ID    uint   `gorm:"primaryKey"`
    Name  string
    Books []Book `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL;"`
}

type Book struct {
    ID       uint    `gorm:"primaryKey"`
    Title    string
    AuthorID *uint   `gorm:"index"`
    Author   *Author
}

// Many-to-Many with additional fields
type User struct {
    ID      uint      `gorm:"primaryKey"`
    Name    string
    Courses []Course  `gorm:"many2many:enrollments;"`
}

type Course struct {
    ID    uint   `gorm:"primaryKey"`
    Name  string
    Users []User `gorm:"many2many:enrollments;"`
}

// Custom join table
type Enrollment struct {
    UserID     uint      `gorm:"primaryKey"`
    CourseID   uint      `gorm:"primaryKey"`
    EnrolledAt time.Time `gorm:"type:timestamptz;default:CURRENT_TIMESTAMP"`
    Grade      *float64  `gorm:"type:decimal(3,2)"`
    Status     string    `gorm:"type:varchar(20);default:'active'"`
}

// Polymorphic associations
type Comment struct {
    ID            uint   `gorm:"primaryKey"`
    Content       string
    CommentableID uint
    CommentableType string
}

// Preloading with conditions
db.Preload("Books", "published = ?", true).
    Preload("Books.Publisher").
    Find(&authors)

// Nested preloading with sorting
db.Preload("Comments", func(db *gorm.DB) *gorm.DB {
    return db.Order("created_at DESC").Limit(10)
}).Find(&posts)
```

## Migrations

### PostgreSQL Migration Best Practices

```go
// Auto migration with additional configurations
func RunMigrations(db *gorm.DB) error {
    // Create extensions
    db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"")
    db.Exec("CREATE EXTENSION IF NOT EXISTS \"btree_gist\"")
    
    // Create custom types
    db.Exec("CREATE TYPE order_status AS ENUM ('pending', 'processing', 'completed', 'cancelled')")
    
    // Run auto migrations
    err := db.AutoMigrate(
        &User{},
        &Product{},
        &Order{},
    )
    if err != nil {
        return err
    }
    
    // Create custom indexes
    db.Exec("CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_email_lower ON users (LOWER(email))")
    db.Exec("CREATE INDEX IF NOT EXISTS idx_products_categories ON products USING GIN(categories)")
    db.Exec("CREATE INDEX IF NOT EXISTS idx_users_metadata ON users USING GIN(metadata)")
    
    // Create triggers
    db.Exec(`
        CREATE OR REPLACE FUNCTION update_updated_at_column()
        RETURNS TRIGGER AS $$
        BEGIN
            NEW.updated_at = CURRENT_TIMESTAMP;
            RETURN NEW;
        END;
        $$ language 'plpgsql';
    `)
    
    db.Exec(`
        CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
        FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    `)
    
    return nil
}

// Manual migration example
type Migration struct {
    ID        uint      `gorm:"primaryKey"`
    Name      string    `gorm:"uniqueIndex"`
    AppliedAt time.Time `gorm:"type:timestamptz;default:CURRENT_TIMESTAMP"`
}

func ApplyMigration(db *gorm.DB, name string, up func(*gorm.DB) error) error {
    var migration Migration
    if err := db.Where("name = ?", name).First(&migration).Error; err == nil {
        // Migration already applied
        return nil
    }
    
    tx := db.Begin()
    if err := up(tx); err != nil {
        tx.Rollback()
        return err
    }
    
    if err := tx.Create(&Migration{Name: name}).Error; err != nil {
        tx.Rollback()
        return err
    }
    
    return tx.Commit().Error
}
```

## Indexes and Constraints

### Creating Efficient Indexes

```go
type User struct {
    ID        uint   `gorm:"primaryKey"`
    Email     string `gorm:"uniqueIndex:idx_email;type:varchar(255)"`
    Username  string `gorm:"uniqueIndex:idx_username;type:varchar(100)"`
    FirstName string `gorm:"index:idx_name"`
    LastName  string `gorm:"index:idx_name"`
    Status    string `gorm:"index:idx_status,where:status='active'"`
    CreatedAt time.Time `gorm:"index:idx_created_at"`
}

// Composite indexes
type Order struct {
    ID         uint      `gorm:"primaryKey"`
    UserID     uint      `gorm:"index:idx_user_status"`
    Status     string    `gorm:"index:idx_user_status"`
    TotalAmount float64  `gorm:"type:decimal(10,2)"`
    CreatedAt  time.Time `gorm:"index:idx_created"`
}

// Partial indexes
db.Exec(`
    CREATE INDEX idx_orders_pending 
    ON orders(created_at) 
    WHERE status = 'pending'
`)

// Expression indexes
db.Exec(`
    CREATE INDEX idx_users_email_lower 
    ON users(LOWER(email))
`)

// GiST index for range queries
db.Exec(`
    CREATE INDEX idx_events_period 
    ON events USING GIST (tstzrange(start_time, end_time))
`)
```

### Constraints

```go
// Check constraints
type Product struct {
    ID    uint    `gorm:"primaryKey"`
    Name  string
    Price float64 `gorm:"check:price > 0"`
    Stock int     `gorm:"check:stock >= 0"`
}

// Foreign key constraints
type Order struct {
    ID     uint `gorm:"primaryKey"`
    UserID uint `gorm:"not null;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT;"`
    User   User
}

// Unique constraints with multiple columns
db.Exec(`
    ALTER TABLE user_roles 
    ADD CONSTRAINT unique_user_role 
    UNIQUE (user_id, role_id)
`)

// Exclusion constraints
db.Exec(`
    ALTER TABLE room_bookings
    ADD CONSTRAINT no_overlapping_bookings
    EXCLUDE USING GIST (
        room_id WITH =,
        tstzrange(start_time, end_time) WITH &&
    )
`)
```

## Transactions

### Transaction Management

```go
// Basic transaction
err := db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&order).Error; err != nil {
        return err
    }
    
    if err := tx.Model(&Product{}).Where("id = ?", productID).
        Update("stock", gorm.Expr("stock - ?", quantity)).Error; err != nil {
        return err
    }
    
    return nil
})

// Manual transaction control
tx := db.Begin()
defer func() {
    if r := recover(); r != nil {
        tx.Rollback()
    }
}()

if err := tx.Error; err != nil {
    return err
}

// Perform operations
if err := tx.Create(&user).Error; err != nil {
    tx.Rollback()
    return err
}

// Savepoint
tx.SavePoint("sp1")
// ... some operations
if err != nil {
    tx.RollbackTo("sp1")
}

return tx.Commit().Error

// Isolation levels
tx := db.Begin(&sql.TxOptions{
    Isolation: sql.LevelSerializable,
    ReadOnly:  false,
})
```

### Concurrent Operations

```go
// Advisory locks
var locked bool
db.Raw("SELECT pg_try_advisory_lock(?)", 12345).Scan(&locked)
if locked {
    defer db.Exec("SELECT pg_advisory_unlock(?)", 12345)
    // Perform exclusive operations
}

// Skip locked rows
var orders []Order
db.Clauses(clause.Locking{
    Strength: "UPDATE",
    Options:  "SKIP LOCKED",
}).Where("status = ?", "pending").Limit(10).Find(&orders)
```

## Performance Optimization

### Query Optimization

```go
// Use EXPLAIN to analyze queries
var result []map[string]interface{}
db.Raw("EXPLAIN (ANALYZE, BUFFERS) SELECT * FROM users WHERE email = ?", "john@example.com").Scan(&result)

// Batch operations
db.Where("status = ?", "inactive").Delete(&User{})

// Use prepared statements
stmtMgr, err := db.DB()
stmt, err := stmtMgr.Prepare("SELECT * FROM users WHERE email = $1")
defer stmt.Close()

// Connection pooling
sqlDB, err := db.DB()
sqlDB.SetMaxIdleConns(10)
sqlDB.SetMaxOpenConns(100)
sqlDB.SetConnMaxLifetime(time.Hour)
sqlDB.SetConnMaxIdleTime(10 * time.Minute)

// Use SELECT only required columns
db.Select("id", "name", "email").Find(&users)

// Disable auto preloading
db.Preload(clause.Associations).Find(&users)

// Use raw SQL for complex queries
db.Raw(`
    SELECT u.*, 
           COUNT(DISTINCT o.id) as order_count,
           COALESCE(SUM(o.total), 0) as total_spent
    FROM users u
    LEFT JOIN orders o ON o.user_id = u.id AND o.status = 'completed'
    WHERE u.created_at > NOW() - INTERVAL '30 days'
    GROUP BY u.id
    HAVING COUNT(DISTINCT o.id) > 5
`).Scan(&userStats)
```

### Caching Strategies

```go
// Query result caching
var users []User
cacheKey := "active_users"
if cached, found := cache.Get(cacheKey); found {
    users = cached.([]User)
} else {
    db.Where("status = ?", "active").Find(&users)
    cache.Set(cacheKey, users, 5*time.Minute)
}

// Prepared statement caching
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
    PrepareStmt: true,
})
```

## Best Practices

### 1. Error Handling

```go
// Always check for errors
if err := db.Where("email = ?", email).First(&user).Error; err != nil {
    if errors.Is(err, gorm.ErrRecordNotFound) {
        // Handle not found
        return nil, fmt.Errorf("user not found")
    }
    // Handle other errors
    return nil, fmt.Errorf("database error: %w", err)
}

// Use Error field
result := db.Model(&user).Update("last_login_at", time.Now())
if result.Error != nil {
    return result.Error
}
if result.RowsAffected == 0 {
    return fmt.Errorf("no rows updated")
}
```

### 2. Context Support

```go
// Use context for cancellation and timeouts
ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
defer cancel()

var users []User
if err := db.WithContext(ctx).Find(&users).Error; err != nil {
    return err
}
```

### 3. Database-Specific Features

```go
// Use PostgreSQL-specific functions
db.Where("email ILIKE ?", "%@example.com%").Find(&users)

// Use native PostgreSQL types
type Event struct {
    ID        uint      `gorm:"primaryKey"`
    Name      string
    TimeRange string    `gorm:"type:tstzrange"`
    Location  string    `gorm:"type:point"`
    Tags      pq.StringArray `gorm:"type:text[]"`
}

// Leverage PostgreSQL operators
db.Where("? <@ time_range", time.Now()).Find(&events)
db.Where("location <-> point(?,?) < ?", lat, lng, radius).Find(&events)
```

### 4. Security Best Practices

```go
// Always use parameterized queries
email := r.FormValue("email")
db.Where("email = ?", email).First(&user) // Safe

// Never do this:
db.Where(fmt.Sprintf("email = '%s'", email)).First(&user) // SQL injection vulnerability!

// Validate input
if err := validator.ValidateEmail(email); err != nil {
    return err
}

// Use allowlists for dynamic columns
allowedColumns := map[string]bool{
    "name": true,
    "email": true,
    "age": true,
}
if allowedColumns[column] {
    db.Order(column + " DESC").Find(&users)
}
```

### 5. Testing with PostgreSQL

```go
// Use transactions for test isolation
func TestUserCreation(t *testing.T) {
    tx := db.Begin()
    defer tx.Rollback()
    
    user := User{Name: "Test User", Email: "test@example.com"}
    err := tx.Create(&user).Error
    assert.NoError(t, err)
    assert.NotZero(t, user.ID)
    
    var found User
    err = tx.First(&found, user.ID).Error
    assert.NoError(t, err)
    assert.Equal(t, user.Name, found.Name)
}

// Use test containers
container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
    ContainerRequest: testcontainers.ContainerRequest{
        Image:        "postgres:15",
        ExposedPorts: []string{"5432/tcp"},
        Env: map[string]string{
            "POSTGRES_PASSWORD": "test",
            "POSTGRES_DB":       "testdb",
        },
    },
})
```

### 6. Monitoring and Logging

```go
// Custom logger
type SqlLogger struct {
    SlowThreshold time.Duration
}

func (l *SqlLogger) LogMode(level logger.LogLevel) logger.Interface {
    return l
}

func (l *SqlLogger) Info(ctx context.Context, msg string, args ...interface{}) {
    log.Printf("INFO: "+msg, args...)
}

func (l *SqlLogger) Warn(ctx context.Context, msg string, args ...interface{}) {
    log.Printf("WARN: "+msg, args...)
}

func (l *SqlLogger) Error(ctx context.Context, msg string, args ...interface{}) {
    log.Printf("ERROR: "+msg, args...)
}

func (l *SqlLogger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
    elapsed := time.Since(begin)
    sql, rows := fc()
    
    if err != nil {
        log.Printf("ERROR: %s [%s] rows:%d error:%s", sql, elapsed, rows, err)
    } else if elapsed > l.SlowThreshold {
        log.Printf("SLOW: %s [%s] rows:%d", sql, elapsed, rows)
    } else {
        log.Printf("TRACE: %s [%s] rows:%d", sql, elapsed, rows)
    }
}

// Use the custom logger
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
    Logger: &SqlLogger{SlowThreshold: 200 * time.Millisecond},
})
```

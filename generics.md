# Go Generics

## Introduction

Go generics, introduced in Go 1.18, allow you to write flexible, reusable code that works 
with multiple types while maintaining type safety. 

## Basic Syntax

The basic syntax for generics uses square brackets `[]` to declare type parameters:

```go
func FunctionName[T TypeConstraint](param T) T {
    // function body
}

type TypeName[T TypeConstraint] struct {
    // struct fields
}
```

## Type Parameters

Type parameters are placeholders for types that will be specified when the generic function or type is used:

```go
// Single type parameter
func Print[T any](value T) {
    fmt.Println(value)
}

// Multiple type parameters
func Swap[T, U any](a T, b U) (U, T) {
    return b, a
}
```

## Type Constraints

Type constraints specify what types can be used for a type parameter. The most basic constraint is `any` (alias for `interface{}`):

```go
func Equal[T comparable](a, b T) bool {
    return a == b
}
```

## Generic Functions

### Simple Generic Function

```go
package main

import "fmt"

// Generic function that works with any type
func Contains[T comparable](slice []T, value T) bool {
    for _, v := range slice {
        if v == value {
            return true
        }
    }
    return false
}

func main() {
    // Works with integers
    nums := []int{1, 2, 3, 4, 5}
    fmt.Println(Contains(nums, 3))  // true
    
    // Works with strings
    words := []string{"hello", "world", "go"}
    fmt.Println(Contains(words, "go"))  // true
}
```

### Generic Map Function

```go
func Map[T, U any](slice []T, transform func(T) U) []U {
    result := make([]U, len(slice))
    for i, v := range slice {
        result[i] = transform(v)
    }
    return result
}

// Usage
func main() {
    numbers := []int{1, 2, 3, 4}
    doubled := Map(numbers, func(n int) int { return n * 2 })
    fmt.Println(doubled)  // [2 4 6 8]
    
    strings := Map(numbers, func(n int) string { 
        return fmt.Sprintf("num_%d", n) 
    })
    fmt.Println(strings)  // [num_1 num_2 num_3 num_4]
}
```

## Generic Types

### Generic Struct

```go
type Stack[T any] struct {
    items []T
}

func (s *Stack[T]) Push(item T) {
    s.items = append(s.items, item)
}

func (s *Stack[T]) Pop() (T, bool) {
    var zero T
    if len(s.items) == 0 {
        return zero, false
    }
    item := s.items[len(s.items)-1]
    s.items = s.items[:len(s.items)-1]
    return item, true
}

// Usage
func main() {
    intStack := &Stack[int]{}
    intStack.Push(10)
    intStack.Push(20)
    
    stringStack := &Stack[string]{}
    stringStack.Push("hello")
    stringStack.Push("world")
}
```

### Generic Interface

```go
type Container[T any] interface {
    Add(T)
    Get(index int) T
    Size() int
}

type SliceContainer[T any] struct {
    items []T
}

func (s *SliceContainer[T]) Add(item T) {
    s.items = append(s.items, item)
}

func (s *SliceContainer[T]) Get(index int) T {
    return s.items[index]
}

func (s *SliceContainer[T]) Size() int {
    return len(s.items)
}
```

## Type Inference

Go can often infer type parameters from the arguments:

```go
func Min[T constraints.Ordered](a, b T) T {
    if a < b {
        return a
    }
    return b
}

func main() {
    // Type inference - no need to specify [int]
    result1 := Min(5, 3)  // inferred as Min[int](5, 3)
    
    // Explicit type specification (rarely needed)
    result2 := Min[float64](3.14, 2.71)
}
```

## Built-in Constraints

Go provides several built-in constraints in the `constraints` package:

```go
import "golang.org/x/exp/constraints"

// constraints.Ordered includes all ordered types
func Sort[T constraints.Ordered](slice []T) {
    sort.Slice(slice, func(i, j int) bool {
        return slice[i] < slice[j]
    })
}

// constraints.Integer includes all integer types
func Sum[T constraints.Integer | constraints.Float](nums []T) T {
    var total T
    for _, n := range nums {
        total += n
    }
    return total
}
```

## Custom Constraints

### Interface Constraints

```go
type Stringer interface {
    String() string
}

func PrintAll[T Stringer](items []T) {
    for _, item := range items {
        fmt.Println(item.String())
    }
}
```

### Type Set Constraints

```go
type Number interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64 |
    ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 |
    ~float32 | ~float64
}

func Add[T Number](a, b T) T {
    return a + b
}

// Works with custom types based on these types
type MyInt int

func main() {
    var a, b MyInt = 10, 20
    result := Add(a, b)  // Works because of ~int
    fmt.Println(result)  // 30
}
```

## Advanced examples

### Generic Channels

```go
func Merge[T any](ch1, ch2 <-chan T) <-chan T {
    out := make(chan T)
    go func() {
        defer close(out)
        for {
            select {
            case v, ok := <-ch1:
                if !ok {
                    ch1 = nil
                    continue
                }
                out <- v
            case v, ok := <-ch2:
                if !ok {
                    ch2 = nil
                    continue
                }
                out <- v
            }
            if ch1 == nil && ch2 == nil {
                break
            }
        }
    }()
    return out
}
```

### Generic Result Type

```go
type Result[T any] struct {
    value T
    err   error
}

func Ok[T any](value T) Result[T] {
    return Result[T]{value: value}
}

func Err[T any](err error) Result[T] {
    var zero T
    return Result[T]{value: zero, err: err}
}

func (r Result[T]) IsOk() bool {
    return r.err == nil
}

func (r Result[T]) Unwrap() (T, error) {
    return r.value, r.err
}

// Usage
func Divide[T constraints.Float](a, b T) Result[T] {
    if b == 0 {
        return Err[T](errors.New("division by zero"))
    }
    return Ok(a / b)
}
```

### Generic Cache

```go
type Cache[K comparable, V any] struct {
    mu    sync.RWMutex
    items map[K]V
}

func NewCache[K comparable, V any]() *Cache[K, V] {
    return &Cache[K, V]{
        items: make(map[K]V),
    }
}

func (c *Cache[K, V]) Set(key K, value V) {
    c.mu.Lock()
    defer c.mu.Unlock()
    c.items[key] = value
}

func (c *Cache[K, V]) Get(key K) (V, bool) {
    c.mu.RLock()
    defer c.mu.RUnlock()
    value, found := c.items[key]
    return value, found
}
```

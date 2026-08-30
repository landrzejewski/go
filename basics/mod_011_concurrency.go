package basics

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

/*
# Module 011 — Concurrency

Go's concurrency model is **CSP**: independent processes communicating over channels. The slogan is

	Do not communicate by sharing memory; share memory by communicating.

It is good advice but not a law — the standard library is full of mutexes, and a mutex is often the
right, simpler answer. The real rule is: use a **channel** to transfer ownership of data or to
signal, and a **mutex** to protect a small piece of shared state.

**Concurrency is not parallelism.** Concurrency is structuring a program as independently executing
pieces; parallelism is running them simultaneously. Go gives you the first, and `GOMAXPROCS` decides
how much of the second you get.

This module is the language-level tour. The `concurrency/` package in this repository holds the
worked exercises: `barbers.go` (the sleeping barber), `barrier.go`, `semaphore.go`,
`producer_consumer_*.go` and `find_files.go` (a three-stage pipeline).
*/

// =================================================================================================
// Section 1: Goroutines
// =================================================================================================

/*
## Goroutines

- `go f()` starts `f` in a new **goroutine** and returns immediately. The arguments are evaluated
  **at the `go` statement**, in the current goroutine — the same rule as `defer`.
- A goroutine is not an OS thread. It starts with about **8 KB of stack** that grows and shrinks on
  demand, and the runtime multiplexes many goroutines onto few threads (the *M:N* scheduler).
  Hundreds of thousands of goroutines is ordinary; hundreds of thousands of threads is not.
- The scheduler is **preemptive** since Go 1.14: a goroutine can be interrupted even in a tight loop
  with no function calls. Before that, such a loop could hang the whole program.
- **`main` does not wait.** When `main` returns, the process exits and every remaining goroutine is
  killed, mid-statement, with no cleanup and no deferred functions. You must synchronise explicitly.
- **A panic in any goroutine kills the entire program**, no matter where the `recover` is. Each
  goroutine that might panic needs its own deferred `recover`.
- **Goroutine leaks are the characteristic Go bug**: a goroutine blocked forever on a channel is
  never collected, and it holds everything it references. Every goroutine you start needs a defined
  way to stop — a closed channel, a cancelled context, or a bounded amount of work.
- `runtime.NumGoroutine()` is the quickest leak check; `GODEBUG=gctrace=1` and the `pprof`
  goroutine profile are the real tools.
*/

func m011Goroutines() {
	fmt.Println("--- Section 1: Goroutines ---")

	fmt.Printf("  goroutines at start: %d\n", runtime.NumGoroutine())

	// Arguments are evaluated at the `go` statement, not when the goroutine runs.
	var wg sync.WaitGroup
	for i := range 3 {
		wg.Add(1)
		go func(captured int) {
			defer wg.Done()
			_ = captured
		}(i * 10)
	}
	wg.Wait()
	fmt.Println("  arguments are evaluated at the `go` statement, like defer")

	// Since Go 1.22 the loop variable is per-iteration, so this no longer needs the parameter.
	results := make([]int, 3)
	var wg2 sync.WaitGroup
	for i := range 3 {
		wg2.Add(1)
		go func() {
			defer wg2.Done()
			results[i] = i * i // safe: each goroutine writes a distinct element
		}()
	}
	wg2.Wait()
	fmt.Printf("  Go 1.22 per-iteration loop variables: %v\n", results)

	// --- Go 1.25: WaitGroup.Go replaces Add/go/defer Done ---
	var wg3 sync.WaitGroup
	total := make([]int, 4)
	for i := range 4 {
		wg3.Go(func() { total[i] = i + 1 }) // Add(1) + go + defer Done(), in one call
	}
	wg3.Wait()
	fmt.Printf("  sync.WaitGroup.Go (Go 1.25): %v\n", total)

	// --- Cheapness ---
	const many = 10_000
	start := time.Now()
	var wg4 sync.WaitGroup
	var counter atomic.Int64
	for range many {
		wg4.Go(func() { counter.Add(1) })
	}
	wg4.Wait()
	fmt.Printf("  %d goroutines started and finished in %v (peak count %d)\n",
		many, time.Since(start).Round(time.Microsecond), counter.Load())

	// --- Each goroutine needs its own recover ---
	var wg5 sync.WaitGroup
	wg5.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("  a goroutine recovered from its own panic: %v\n", r)
			}
		}()
		panic("without this recover, the whole program would die")
	})
	wg5.Wait()

	fmt.Printf("  goroutines at end: %d (a leak shows up here first)\n", runtime.NumGoroutine())
}

// =================================================================================================
// Section 2: Channels
// =================================================================================================

/*
## Channels

- `make(chan T)` is **unbuffered**; `make(chan T, n)` has a buffer of `n`.
- An **unbuffered** channel is a *rendezvous*: the send blocks until a receiver is ready, and the
  receive blocks until a sender is. It synchronises the two goroutines as well as passing a value.
- A **buffered** channel decouples them: sends block only when the buffer is full, receives only
  when it is empty. Buffering is for smoothing bursts, not for "making it faster" — an unbounded
  buffer just moves the failure to memory exhaustion.
- **Direction** is part of the type: `chan<- T` is send-only, `<-chan T` is receive-only. Use them
  in function signatures; the compiler then enforces the intended role.
- `close(ch)` says *no more values will be sent*.
    - Receiving from a closed channel yields the zero value **immediately** and forever.
    - The **comma-ok** receive `v, ok := <-ch` reports `ok == false` once the channel is closed and
      drained.
    - `for v := range ch` reads until the channel is closed. If nobody closes it, this deadlocks.
- The rules that cause panics:
    - **sending on a closed channel panics**
    - **closing a closed channel panics**
    - **closing a nil channel panics**
  Only receiving is always safe. The convention that follows is: **the sender closes**, never the
  receiver, and with multiple senders nobody closes — use a `sync.WaitGroup` and close afterwards.
- **A nil channel blocks forever** on both send and receive. That sounds useless and is in fact a
  key idiom: setting a channel variable to `nil` disables its `case` in a `select` (Section 3).
- Deadlock: if *every* goroutine is blocked, the runtime detects it and aborts with
  `fatal error: all goroutines are asleep - deadlock!`. This is **not recoverable**. If even one
  goroutine is still runnable, the runtime cannot tell, and the program simply hangs.
*/

func m011Channels() {
	fmt.Println("\n--- Section 2: Channels ---")

	// --- Unbuffered: a rendezvous ---
	done := make(chan string)
	go func() { done <- "the send blocked until this receive was ready" }()
	fmt.Printf("  unbuffered: %q\n", <-done)

	// --- Buffered: decoupled up to the capacity ---
	buf := make(chan int, 3)
	buf <- 1
	buf <- 2
	fmt.Printf("  buffered: len=%d cap=%d (two sends did not block)\n", len(buf), cap(buf))
	fmt.Printf("  receiving: %d %d\n", <-buf, <-buf)

	// --- Direction ---
	numbers := make(chan int, 3)
	m011Produce(numbers, 3) // takes chan<- int
	fmt.Printf("  directional channels: consumed %v\n", m011Consume(numbers))

	// --- close, comma-ok and range ---
	ch := make(chan int, 3)
	for i := range 3 {
		ch <- i
	}
	close(ch)
	fmt.Print("  range over a closed channel drains it: ")
	for v := range ch {
		fmt.Print(v, " ")
	}
	fmt.Println()
	v, ok := <-ch
	fmt.Printf("  after draining: v=%d ok=%t (a closed channel yields zero values forever)\n", v, ok)

	// --- The panic rules ---
	closed := make(chan int)
	close(closed)
	fmt.Printf("  send on a closed channel:  %v\n", m005CatchPanic(func() { closed <- 1 }))
	fmt.Printf("  close a closed channel:    %v\n", m005CatchPanic(func() { close(closed) }))
	fmt.Printf("  close a nil channel:       %v\n", m005CatchPanic(func() { var c chan int; close(c) }))
	fmt.Println("  only RECEIVING is always safe - so the sender closes, and with several")
	fmt.Println("  senders nobody closes: use a WaitGroup and close after Wait()")

	// --- The multiple-sender pattern ---
	out := make(chan int)
	var wg sync.WaitGroup
	for i := range 3 {
		wg.Go(func() { out <- i })
	}
	go func() { wg.Wait(); close(out) }() // exactly one closer, after every sender is done
	sum := 0
	for v := range out {
		sum += v
	}
	fmt.Printf("  three senders, one closer after Wait(): sum=%d\n", sum)

	// --- A nil channel blocks forever ---
	fmt.Println("  a nil channel blocks forever on both send and receive -")
	fmt.Println("  which is exactly what makes the nil-out idiom in Section 3 work")
}

func m011Produce(out chan<- int, n int) { // send-only: the compiler forbids receiving
	for i := range n {
		out <- i
	}
	close(out)
}

func m011Consume(in <-chan int) []int { // receive-only: the compiler forbids sending and closing
	var got []int
	for v := range in {
		got = append(got, v)
	}
	return got
}

// =================================================================================================
// Section 3: select
// =================================================================================================

/*
## select

- `select` waits on **several channel operations at once** and proceeds with the first that becomes
  ready. If several are ready it picks one **uniformly at random** — deliberately, so that no case
  can starve another.
- A **`default` case** makes the whole `select` non-blocking: if nothing is ready, `default` runs
  immediately. This is how you write a non-blocking send or receive.
- **An empty `select{}`** blocks forever. With no other runnable goroutine it triggers the deadlock
  detector; with a running server it is a way to park `main`.
- **Timeouts** are `select` plus `time.After`. Note that `time.After` allocates a timer that is not
  collected until it fires — in a hot loop use `time.NewTimer` and `Stop` it. Since Go 1.23 the
  unfired timer is collectable, which removed the worst of that leak.
- **The nil-channel idiom**: setting a channel variable to `nil` disables its case permanently,
  because a nil channel never becomes ready. This is the standard way to merge several inputs and
  drop each one as it closes — without it you would spin on the already-closed channel.
- `for { select { ... } }` with a `case <-ctx.Done(): return` is the canonical shape of a worker
  loop (Section 5).
*/

func m011Select() {
	fmt.Println("\n--- Section 3: select ---")

	// --- Waiting on whichever is first ---
	fast := make(chan string)
	slow := make(chan string)
	go func() { time.Sleep(5 * time.Millisecond); fast <- "fast" }()
	go func() { time.Sleep(50 * time.Millisecond); slow <- "slow" }()

	select {
	case v := <-fast:
		fmt.Printf("  select took the first ready case: %q\n", v)
	case v := <-slow:
		fmt.Printf("  select took: %q\n", v)
	}

	// --- default makes it non-blocking ---
	empty := make(chan int)
	select {
	case v := <-empty:
		fmt.Println("  received", v)
	default:
		fmt.Println("  default: nothing was ready, so we did not block")
	}

	full := make(chan int, 1)
	full <- 1
	select {
	case full <- 2:
		fmt.Println("  sent")
	default:
		fmt.Println("  default: the buffer was full, so the send was skipped rather than blocking")
	}

	// --- Timeout ---
	silent := make(chan int)
	select {
	case v := <-silent:
		fmt.Println("  received", v)
	case <-time.After(20 * time.Millisecond):
		fmt.Println("  timed out after 20ms via select + time.After")
	}

	// --- Random choice among ready cases ---
	a, b := make(chan int, 10), make(chan int, 10)
	for range 10 {
		a <- 1
		b <- 2
	}
	counts := map[int]int{}
	for range 10 {
		select {
		case v := <-a:
			counts[v]++
		case v := <-b:
			counts[v]++
		}
	}
	fmt.Printf("  both ready 10 times -> roughly random split: %v\n", counts)

	// --- The nil-channel idiom, as a fan-in ---
	merged := m011Merge(m011Numbers(1, 3), m011Numbers(10, 2))
	var got []int
	for v := range merged {
		got = append(got, v)
	}
	fmt.Printf("  fan-in via the nil-out idiom: %v (order varies)\n", got)
}

func m011Numbers(start, count int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for i := range count {
			out <- start + i
		}
	}()
	return out
}

// m011Merge fans two channels into one. The loop condition MUST be in the header: with it at the
// bottom, `continue` after a close would skip the check and the next select would wait on two nil
// channels - which blocks forever, since there is no ready case and no default.
func m011Merge[T any](ch1, ch2 <-chan T) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for ch1 != nil || ch2 != nil {
			select {
			case v, ok := <-ch1:
				if !ok {
					ch1 = nil // disables this case in every later select
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
		}
	}()
	return out
}

// =================================================================================================
// Section 4: sync and sync/atomic
// =================================================================================================

/*
## sync and sync/atomic

- **`sync.Mutex`** — mutual exclusion. Its zero value is an unlocked mutex, so it needs no
  constructor. It is **not reentrant**: locking it twice in the same goroutine deadlocks.
- **`sync.RWMutex`** — many readers or one writer. It is only worth it when reads dominate *and*
  are slow; for short critical sections a plain `Mutex` is faster, because `RWMutex` has more
  bookkeeping.
- The idiom is always `mu.Lock(); defer mu.Unlock()`, and the mutex is declared **immediately above
  the fields it protects**, with a comment saying so.
- A `sync.Mutex` **must not be copied** after first use. `go vet`'s `copylocks` check catches it,
  which is why any struct containing one is passed by pointer (module 007).
- **`sync.WaitGroup`** — wait for a set of goroutines. `Add` before starting, `Done` when finished,
  `Wait` to block. **Go 1.25's `wg.Go(f)`** does `Add(1)`, `go`, and `defer Done()` in one call and
  should be preferred: it makes the classic "Add inside the goroutine" race impossible.
- **`sync.Once`** — run something exactly once. **`sync.OnceValue[T]` / `OnceValues`** (Go 1.21) are
  the generic versions and are usually nicer (module 010, Section 9).
- **`sync.Cond`** — wait for a condition. Rarely needed; a channel usually expresses the same thing
  more clearly. This repo's `concurrency/barrier.go` and `semaphore.go` use it deliberately, to show
  the classic form.
- **`sync.Pool`** — a free list of temporary objects to reduce allocation pressure. Its contents can
  vanish at any GC. Only use it after profiling shows allocation is the bottleneck.
- **`sync.Map`** — a concurrent map for two specific workloads: a key written once and read many
  times, or disjoint key sets per goroutine. For anything else a plain map plus a `RWMutex` is
  faster and clearer.
- **`sync/atomic`** — lock-free operations on a single word. Prefer the **typed** wrappers added in
  Go 1.19 (`atomic.Int64`, `atomic.Bool`, `atomic.Pointer[T]`, `atomic.Value`) over the older
  free functions: they cannot be accidentally accessed non-atomically, and their zero value is
  ready to use.
- **The race detector** (`go test -race`, `go run -race`) is the only reliable way to find data
  races. It has real overhead (roughly 2-20×) but it finds bugs that no amount of reading will.
  Run your tests with it in CI.
*/

// The idiomatic shape: the mutex sits directly above what it guards.
type m011Registry struct {
	mu     sync.RWMutex
	counts map[string]int // guarded by mu
}

func m011NewRegistry() *m011Registry {
	return &m011Registry{counts: make(map[string]int)}
}

func (r *m011Registry) Inc(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.counts[key]++
}

func (r *m011Registry) Get(key string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.counts[key]
}

func m011SyncPrimitives() {
	fmt.Println("\n--- Section 4: sync and sync/atomic ---")

	// --- Mutex-guarded map ---
	reg := m011NewRegistry()
	var wg sync.WaitGroup
	for range 100 {
		wg.Go(func() { reg.Inc("hits") })
	}
	wg.Wait()
	fmt.Printf("  RWMutex-guarded map after 100 concurrent Inc: %d\n", reg.Get("hits"))

	// Without the mutex this would be `fatal error: concurrent map writes` - which is NOT
	// a recoverable panic; it aborts the process.
	fmt.Println("  an unguarded concurrent map write is a FATAL error, not a panic:")
	fmt.Println("    fatal error: concurrent map writes  (recover cannot catch it)")

	// --- Typed atomics (Go 1.19) ---
	var hits atomic.Int64
	var ready atomic.Bool
	var wg2 sync.WaitGroup
	for range 1000 {
		wg2.Go(func() { hits.Add(1) })
	}
	wg2.Wait()
	ready.Store(true)
	fmt.Printf("  atomic.Int64 after 1000 concurrent Add: %d (ready=%t)\n", hits.Load(), ready.Load())

	// CompareAndSwap: the building block of every lock-free algorithm.
	var state atomic.Int32
	swapped := state.CompareAndSwap(0, 5)
	failed := state.CompareAndSwap(0, 9)
	fmt.Printf("  CompareAndSwap(0,5)=%t then CompareAndSwap(0,9)=%t, value=%d\n",
		swapped, failed, state.Load())

	// atomic.Pointer[T] - a generic, type-safe atomic pointer.
	var cfg atomic.Pointer[m011Registry]
	cfg.Store(reg)
	fmt.Printf("  atomic.Pointer[T] (generic, Go 1.19): loaded a registry with %d hits\n",
		cfg.Load().Get("hits"))

	// --- sync.Once and OnceValue ---
	var once sync.Once
	for range 3 {
		once.Do(func() { fmt.Println("  sync.Once: this line runs exactly once") })
	}

	// --- The non-reentrancy trap ---
	fmt.Println("  a sync.Mutex is NOT reentrant: locking it twice in one goroutine deadlocks")
	fmt.Println("  run everything under `go test -race` - it is the only reliable race finder")
}

// =================================================================================================
// Section 5: context
// =================================================================================================

/*
## context

- `context.Context` carries a **cancellation signal**, a **deadline** and **request-scoped values**
  across API boundaries and goroutines. It is how a Go program says "stop, nobody needs this any
  more".
- The conventions are strict and worth following exactly:
    - a `Context` is the **first parameter**, named `ctx`, of any function that can block
    - **never store one in a struct** (with rare, documented exceptions); pass it explicitly
    - **never pass a nil Context** — use `context.TODO()` if you genuinely do not have one yet
    - a `Context` is **immutable**: each `With...` returns a *derived* child
- The constructors:
    - `context.Background()` — the root, at the top of `main` or a request
    - `context.TODO()` — a placeholder marking a decision not yet made
    - `context.WithCancel(parent)` — returns a `cancel` function
    - `context.WithTimeout(parent, d)` / `WithDeadline(parent, t)`
    - `context.WithValue(parent, key, val)`
    - `context.WithCancelCause` (Go 1.20) — cancel with an explanatory error, read back with
      `context.Cause(ctx)`
    - `context.AfterFunc` (Go 1.21) — run a function when the context is done
    - `context.WithoutCancel` (Go 1.21) — keep the values but drop the cancellation
- **Always `defer cancel()`.** Not calling it leaks the context's internal goroutine and timer until
  the parent is cancelled. `go vet`'s `lostcancel` check catches the common cases.
- Cancellation **propagates down** the tree: cancelling a parent cancels every descendant. It never
  propagates up.
- `ctx.Done()` returns a channel closed on cancellation; `ctx.Err()` says why —
  `context.Canceled` or `context.DeadlineExceeded`.
- **`WithValue` is for request-scoped data only** — a trace ID, an authenticated user — never for
  optional parameters. Use an **unexported key type** so no other package can collide with or read
  your key. A `string` key is a bug waiting to happen.
*/

// The correct context key: an unexported defined type, so no other package can collide.
type m011ctxKey string

const m011RequestIDKey m011ctxKey = "requestID"

func m011Context() {
	fmt.Println("\n--- Section 5: context ---")

	// --- Timeout ---
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel() // always, even when the timeout will fire on its own

	if err := m011SlowWork(ctx, 200*time.Millisecond); err != nil {
		fmt.Printf("  work cancelled: %v (ctx.Err()=%v)\n", err, ctx.Err())
	}

	// The same work, inside the budget.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel2()
	if err := m011SlowWork(ctx2, 10*time.Millisecond); err == nil {
		fmt.Println("  work finished inside its deadline")
	}

	// --- Explicit cancellation propagating to children ---
	parent, cancelParent := context.WithCancel(context.Background())
	child, cancelChild := context.WithCancel(parent)
	defer cancelChild()
	cancelParent()
	<-child.Done() // the child was cancelled by the parent
	fmt.Printf("  cancelling a parent cancelled its child: %v\n", child.Err())

	// --- WithCancelCause (Go 1.20): cancel with a reason ---
	ctx3, cancelCause := context.WithCancelCause(context.Background())
	cancelCause(errors.New("upstream returned 503"))
	fmt.Printf("  WithCancelCause: Err()=%v Cause()=%v\n", ctx3.Err(), context.Cause(ctx3))

	// --- AfterFunc (Go 1.21) ---
	ctx4, cancel4 := context.WithCancel(context.Background())
	fired := make(chan struct{})
	context.AfterFunc(ctx4, func() { close(fired) })
	cancel4()
	<-fired
	fmt.Println("  context.AfterFunc (Go 1.21) ran on cancellation")

	// --- WithoutCancel (Go 1.21): keep the values, drop the cancellation ---
	valued := context.WithValue(context.Background(), m011RequestIDKey, "req-42")
	cancelled, cancel5 := context.WithCancel(valued)
	cancel5()
	detached := context.WithoutCancel(cancelled)
	fmt.Printf("  WithoutCancel keeps values (%v) but not cancellation (Err=%v)\n",
		detached.Value(m011RequestIDKey), detached.Err())

	// --- Values, with a properly typed key ---
	fmt.Printf("  request-scoped value: %v\n", m011HandleRequest(valued))
	fmt.Println("  the key type is unexported, so no other package can collide with it")
	fmt.Println("  WithValue is for request-scoped data only - never for optional parameters")
}

// m011SlowWork is the canonical cancellable worker: select on the work and on ctx.Done().
func m011SlowWork(ctx context.Context, duration time.Duration) error {
	select {
	case <-time.After(duration):
		return nil
	case <-ctx.Done():
		return fmt.Errorf("m011SlowWork: %w", ctx.Err())
	}
}

func m011HandleRequest(ctx context.Context) string {
	id, ok := ctx.Value(m011RequestIDKey).(string)
	if !ok {
		return "no request id"
	}
	return "handling " + id
}

// =================================================================================================
// Section 6: Patterns — worker pools, pipelines, fan-in and fan-out
// =================================================================================================

/*
## Patterns — worker pools, pipelines, fan-in and fan-out

- **Worker pool** — N goroutines reading one jobs channel and writing one results channel. Use it to
  bound concurrency when the work is CPU-bound (`runtime.NumCPU()` workers) or when a downstream
  service imposes a rate limit. Do **not** start an unbounded goroutine per item.
- **Pipeline** — a chain of stages, each a function taking a receive-only channel and returning
  another. Each stage owns its output channel and closes it when done. This repo's
  `concurrency/find_files.go` is a three-stage example: walk, filter by extension, filter by
  content.
- **Fan-out** — several goroutines reading the same channel. **Fan-in** — merging several channels
  into one, which is the `m011Merge` from Section 3.
- **Semaphore** — a buffered channel used as a counting semaphore: acquire by sending, release by
  receiving. `concurrency/semaphore.go` shows the `sync.Cond` version of the same idea.
- **Every pattern needs a cancellation story.** A stage that blocks sending into a full downstream
  channel after the consumer has gone away is a leak. Give every stage a `ctx` and a
  `case <-ctx.Done(): return`.
- `golang.org/x/sync/errgroup` is the community-standard wrapper: it runs a group of goroutines,
  cancels the shared context when one fails, and returns the first error. It is not in the standard
  library but is used almost universally.
*/

func m011Patterns() {
	fmt.Println("\n--- Section 6: Patterns ---")

	// --- Worker pool ---
	jobs := make(chan int, 20)
	results := make(chan int, 20)
	workers := min(4, runtime.NumCPU())

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for j := range jobs { // every worker reads the SAME channel: fan-out
				results <- j * j
			}
		})
	}
	for i := 1; i <= 9; i++ {
		jobs <- i
	}
	close(jobs)    // no more work; each worker's range then ends
	wg.Wait()      // wait for all of them
	close(results) // only now is it safe to close the output

	sum := 0
	for r := range results {
		sum += r
	}
	fmt.Printf("  worker pool: %d workers squared 1..9, sum=%d\n", workers, sum)

	// --- Pipeline ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	final := m011Square(ctx, m011Filter(ctx, m011Generate(ctx, 1, 10), func(n int) bool {
		return n%2 == 0
	}))
	var out []int
	for v := range final {
		out = append(out, v)
	}
	fmt.Printf("  pipeline generate -> filter even -> square: %v\n", out)

	// --- Semaphore as a buffered channel ---
	const limit = 3
	sem := make(chan struct{}, limit)
	var peak, current atomic.Int32
	var wg2 sync.WaitGroup
	for range 20 {
		wg2.Go(func() {
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release
			n := current.Add(1)
			for {
				p := peak.Load()
				if n <= p || peak.CompareAndSwap(p, n) {
					break
				}
			}
			time.Sleep(time.Millisecond)
			current.Add(-1)
		})
	}
	wg2.Wait()
	fmt.Printf("  buffered channel as a semaphore: limit=%d, observed peak=%d\n", limit, peak.Load())

	// --- Early cancellation of a pipeline ---
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2() // `go vet`'s lostcancel check insists on this, even though we cancel below
	src := m011Generate(ctx2, 1, 1_000_000)
	taken := 0
	for range src {
		taken++
		if taken == 3 {
			cancel2() // the generator sees ctx.Done() and returns instead of leaking
			break
		}
	}
	time.Sleep(5 * time.Millisecond)
	fmt.Printf("  cancelled a 1,000,000-item generator after %d items; goroutines now %d\n",
		taken, runtime.NumGoroutine())

	fmt.Println("  golang.org/x/sync/errgroup wraps all of this: shared ctx, first error wins")
	fmt.Println("  see concurrency/find_files.go for a real three-stage pipeline")
}

func m011Generate(ctx context.Context, from, to int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for i := from; i <= to; i++ {
			select {
			case out <- i:
			case <-ctx.Done(): // without this, the generator leaks when the consumer stops early
				return
			}
		}
	}()
	return out
}

func m011Filter(ctx context.Context, in <-chan int, keep func(int) bool) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for v := range in {
			if !keep(v) {
				continue
			}
			select {
			case out <- v:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func m011Square(ctx context.Context, in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for v := range in {
			select {
			case out <- v * v:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// =================================================================================================
// Section 7: The Memory Model, Races and Common Failures
// =================================================================================================

/*
## The Memory Model, Races and Common Failures

- The **Go memory model** defines when a read in one goroutine is guaranteed to see a write from
  another. It is expressed as a *happens-before* relation, and the guarantees you get are:
    - a `go` statement happens before the goroutine starts
    - a send on a channel happens before the corresponding receive completes
    - a **close** happens before a receive that returns because the channel is closed
    - for an unbuffered channel, a **receive** happens before the corresponding send completes
    - `mu.Unlock()` happens before a later `mu.Lock()` returns
    - `once.Do(f)` — `f` happens before any `Do` returns
    - the `sync/atomic` operations are sequentially consistent (made explicit in Go 1.19)
- **Without one of those, there is no guarantee at all** — not even that a write will ever become
  visible. A "benign" data race is not benign: the compiler may cache the value in a register, or
  reorder the write, and the loop that waits for a plain `bool` flag may spin forever.
- The failure modes, and which are recoverable:
    - a **data race** — undefined behaviour; found only by `-race`
    - **`fatal error: concurrent map writes`** — detected by the runtime, **not recoverable**
    - **`fatal error: all goroutines are asleep - deadlock!`** — detected only when *every*
      goroutine is blocked; **not recoverable**
    - a **partial deadlock** — some goroutines blocked forever while others run. The runtime cannot
      detect it, so the program just hangs. This is the common case in real servers.
    - a **goroutine leak** — a blocked goroutine that is never collected
    - a **panic in a goroutine** — kills the whole program
- Debugging tools, in the order you should reach for them:
   1. `go test -race` / `go run -race`
   2. `runtime.NumGoroutine()` before and after, as a cheap leak assertion
   3. the `pprof` goroutine profile (`import _ "net/http/pprof"`) to see where they are blocked
   4. `GOTRACEBACK=all` to dump every goroutine's stack on a crash
   5. **`testing/synctest`** (Go 1.25) — a fake clock and deterministic scheduling for concurrent
      tests, so a timeout test runs instantly and never flakes. Module 014 covers it.
*/

func m011MemoryModelAndFailures() {
	fmt.Println("\n--- Section 7: The Memory Model, Races and Common Failures ---")

	// A correct handoff: the channel send/receive establishes happens-before.
	var data string
	ready := make(chan struct{})
	go func() {
		data = "written before the close" // safe: the close below establishes the ordering
		close(ready)
	}()
	<-ready
	fmt.Printf("  channel close as a happens-before edge: %q\n", data)

	// The same with a mutex.
	var mu sync.Mutex
	var guarded int
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			mu.Lock()
			defer mu.Unlock()
			guarded++
		})
	}
	wg.Wait()
	fmt.Printf("  Unlock-then-Lock as a happens-before edge: %d\n", guarded)

	// And with an atomic.
	var flag atomic.Bool
	var payload atomic.Pointer[string]
	msg := "published atomically"
	go func() { payload.Store(&msg); flag.Store(true) }()
	for !flag.Load() {
		runtime.Gosched()
	}
	fmt.Printf("  atomics are sequentially consistent: %q\n", *payload.Load())

	// --- What NOT to do ---
	fmt.Println("  a plain bool flag without sync is a data race, not a 'benign' one:")
	fmt.Println("    the compiler may hoist the read out of the loop and spin forever")

	fmt.Println("  failure modes and whether recover() helps:")
	fmt.Println("    data race                     -> undefined; only -race finds it")
	fmt.Println("    concurrent map writes         -> fatal error, NOT recoverable")
	fmt.Println("    all goroutines asleep         -> fatal error, NOT recoverable")
	fmt.Println("    partial deadlock              -> undetected; the program just hangs")
	fmt.Println("    panic in any goroutine        -> kills the process unless IT recovers")

	fmt.Printf("  goroutines still alive at the end of this module: %d\n", runtime.NumGoroutine())
	fmt.Println("  testing/synctest (Go 1.25) makes concurrent tests deterministic - module 014")
}

// Run011 runs every section of module 011 in order.
func Run011() {
	m011Goroutines()
	m011Channels()
	m011Select()
	m011SyncPrimitives()
	m011Context()
	m011Patterns()
	m011MemoryModelAndFailures()
}

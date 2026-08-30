package basics

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
	"unicode/utf8"
)

/*
# mod_014_testing_test.go — the runnable half of module 014

Every technique described in mod_014_testing.go is exercised here. Run them with:

	go test -v ./basics/
	go test -run 'TestTableDriven/utf8' ./basics/
	go test -bench . -benchmem ./basics/
	go test -race ./basics/
	go test -fuzz FuzzReverse ./basics/
	go test -cover ./basics/

This file is in `package basics` (the internal test package), so it can reach the unexported
identifiers the module defines. `TestExternalPackageStyle` explains the alternative.
*/

// =================================================================================================
// Section 1: The basics — a plain test, helpers and cleanup
// =================================================================================================

func TestReverse(t *testing.T) {
	got := m014Reverse("hello")
	want := "olleh"
	if got != want {
		// The conventional message shape: the call, what we got, what we wanted.
		t.Errorf("m014Reverse(%q) = %q, want %q", "hello", got, want)
	}
}

// m014AssertEqual is an assertion helper. t.Helper() makes a failure point at the CALLER's line.
func m014AssertEqual[T comparable](t *testing.T, got, want T, context string) {
	t.Helper() // always the first line of a helper
	if got != want {
		t.Errorf("%s = %v, want %v", context, got, want)
	}
}

func TestHelperAndCleanup(t *testing.T) {
	// t.Cleanup runs in LIFO order, after the test and all its subtests finish - and, unlike
	// defer, it still runs after t.Fatal and works when registered from a helper.
	// Cleanups run LIFO, so this one - registered FIRST - runs LAST, and sees both appends.
	var order []string
	t.Cleanup(func() {
		want := []string{"registered last, runs first", "registered first, runs last"}
		if !slices.Equal(order, want) {
			t.Errorf("cleanup order = %v, want %v", order, want)
		}
	})
	t.Cleanup(func() { order = append(order, "registered first, runs last") })
	t.Cleanup(func() { order = append(order, "registered last, runs first") })

	m014AssertEqual(t, m014Reverse("ab"), "ba", `m014Reverse("ab")`)
	m014AssertEqual(t, m014IsEven(4), true, "m014IsEven(4)")

	// t.TempDir is removed automatically when the test ends.
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/scratch.txt", []byte("x"), 0o600); err != nil {
		t.Fatalf("WriteFile in t.TempDir(): %v", err)
	}

	// t.Setenv restores the previous value afterwards (and forces this test non-parallel).
	t.Setenv("M014_EXAMPLE", "set for this test only")
	m014AssertEqual(t, os.Getenv("M014_EXAMPLE"), "set for this test only", "os.Getenv")
}

func TestSkipping(t *testing.T) {
	if testing.Short() {
		t.Skip("skipped under -short; this is the conventional guard for a slow test")
	}
	t.Log("this line is shown only with -v, or when the test fails")
}

// =================================================================================================
// Section 2: Table-driven tests and subtests
// =================================================================================================

func TestTableDriven(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", ""},
		{"single", "a", "a"},
		{"ascii", "hello", "olleh"},
		{"utf8", "Gęś", "śęG"},
		{"emoji", "ab🙂", "🙂ba"},
		{"palindrome", "kajak", "kajak"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := m014Reverse(tt.in); got != tt.want {
				t.Errorf("m014Reverse(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTableDrivenWithErrors(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    map[string]string
		wantErr bool  // expect some error
		errIs   error // if non-nil, the error must match this sentinel (errors.Is)
	}{
		{"single", "a=1", map[string]string{"a": "1"}, false, nil},
		{"several", "a=1, b=2", map[string]string{"a": "1", "b": "2"}, false, nil},
		{"spaces trimmed", " a = 1 ", map[string]string{"a": "1"}, false, nil},
		{"empty", "  ", nil, true, m014ErrEmpty},
		{"no equals", "a=1,oops", nil, true, nil}, // any error is fine here
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m014ParseKV(tt.in)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("m014ParseKV(%q) succeeded, want an error", tt.in)
				}
				// Compare errors with errors.Is, never with == or by message (module 009).
				if tt.errIs != nil && !errors.Is(err, tt.errIs) {
					t.Fatalf("m014ParseKV(%q) error = %v, want %v", tt.in, err, tt.errIs)
				}
				return
			}

			if err != nil {
				t.Fatalf("m014ParseKV(%q) unexpected error: %v", tt.in, err)
			}
			// maps.Equal beats reflect.DeepEqual: typed, faster, and clearer.
			if !maps.Equal(got, tt.want) {
				t.Errorf("m014ParseKV(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParallelSubtests(t *testing.T) {
	// Registered on the PARENT. A `defer` here would run before the parallel children do -
	// which is exactly why t.Cleanup exists.
	t.Cleanup(func() { t.Log("parent cleanup runs after every parallel child has finished") })

	for _, in := range []string{"alpha", "beta", "gamma"} {
		// Since Go 1.22 the loop variable is per-iteration, so no `in := in` is needed here.
		t.Run(in, func(t *testing.T) {
			t.Parallel()
			if got := m014Reverse(m014Reverse(in)); got != in {
				t.Errorf("round trip of %q = %q", in, got)
			}
		})
	}
}

// =================================================================================================
// Section 3: Benchmarks
// =================================================================================================

func BenchmarkReverse(b *testing.B) {
	// b.Loop (Go 1.24): setup before it is excluded from the timing automatically, and the
	// compiler cannot optimise the body away.
	input := strings.Repeat("gopher ", 20)
	for b.Loop() {
		m014Reverse(input)
	}
}

func BenchmarkReverseBySize(b *testing.B) {
	for _, size := range []int{8, 128, 2048} {
		b.Run(fmt.Sprintf("size=%d", size), func(b *testing.B) {
			input := strings.Repeat("a", size)
			b.ReportAllocs()
			for b.Loop() {
				m014Reverse(input)
			}
		})
	}
}

func BenchmarkParseKV(b *testing.B) {
	input := "host=localhost, port=8080, tls=true, timeout=30s"
	b.ReportAllocs()
	for b.Loop() {
		if _, err := m014ParseKV(input); err != nil {
			b.Fatal(err)
		}
	}
}

// The two ways of building a string, so -benchmem shows why Builder wins (module 002a).
func BenchmarkConcatPlus(b *testing.B) {
	for b.Loop() {
		s := ""
		for i := range 100 {
			s += string(rune('a' + i%26))
		}
		_ = s
	}
}

func BenchmarkConcatBuilder(b *testing.B) {
	for b.Loop() {
		var sb strings.Builder
		for i := range 100 {
			sb.WriteByte(byte('a' + i%26))
		}
		_ = sb.String()
	}
}

// =================================================================================================
// Section 4: Fuzzing
// =================================================================================================

// FuzzReverse checks a PROPERTY - reversing twice is the identity - rather than an expected value.
//
//	go test -fuzz FuzzReverse ./basics/
func FuzzReverse(f *testing.F) {
	for _, seed := range []string{"", "a", "hello", "Gęś", "🙂🙃", "kajak"} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, in string) {
		once := m014Reverse(in)
		twice := m014Reverse(once)

		// The round-trip property holds only for valid UTF-8: m014Reverse goes through []rune,
		// which replaces every invalid byte with U+FFFD.
		if !utf8.ValidString(in) {
			t.Skip("invalid UTF-8 input: []rune conversion is lossy by design")
		}
		if twice != in {
			t.Errorf("m014Reverse(m014Reverse(%q)) = %q, want %q", in, twice, in)
		}
		// A second invariant: reversing preserves the rune count.
		if a, b := utf8.RuneCountInString(in), utf8.RuneCountInString(once); a != b {
			t.Errorf("rune count changed: %d -> %d", a, b)
		}
	})
}

// FuzzParseKV checks the weakest useful property: it must never panic.
func FuzzParseKV(f *testing.F) {
	f.Add("a=1")
	f.Add("a=1,b=2")
	f.Add("=")
	f.Add(",,,")

	f.Fuzz(func(t *testing.T, in string) {
		got, err := m014ParseKV(in) // must return an error, never panic
		if err == nil && got == nil {
			t.Errorf("m014ParseKV(%q) returned a nil map and a nil error", in)
		}
	})
}

// =================================================================================================
// Section 5: Examples as documentation
// =================================================================================================

func Example_reverse() {
	fmt.Println(m014Reverse("hello"))
	fmt.Println(m014Reverse("Gęś"))
	// Output:
	// olleh
	// śęG
}

func Example_parseKV() {
	kv, err := m014ParseKV("host=localhost, port=8080")
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(kv["host"], kv["port"])
	// Output: localhost 8080
}

// Unordered output compares the lines as a SET - which is what map iteration requires.
func Example_parseKVUnordered() {
	kv, _ := m014ParseKV("a=1, b=2, c=3")
	for k, v := range kv {
		fmt.Printf("%s=%s\n", k, v)
	}
	// Unordered output:
	// b=2
	// a=1
	// c=3
}

// =================================================================================================
// Section 6: testing/synctest (Go 1.25)
// =================================================================================================

// TestSynctestFakeClock sleeps for an hour, instantly.
func TestSynctestFakeClock(t *testing.T) {
	realStart := time.Now()

	synctest.Test(t, func(t *testing.T) {
		fakeStart := time.Now()

		done := make(chan struct{})
		go func() {
			time.Sleep(time.Hour) // the fake clock jumps as soon as everything is blocked
			close(done)
		}()
		<-done

		if elapsed := time.Since(fakeStart); elapsed != time.Hour {
			t.Errorf("fake elapsed = %v, want exactly 1h", elapsed)
		}
	})

	// The real test took microseconds.
	if realElapsed := time.Since(realStart); realElapsed > time.Second {
		t.Errorf("the real test took %v; the whole point is that it should be instant", realElapsed)
	}
}

// TestSynctestWait replaces the `time.Sleep(10ms) // let the goroutine catch up` anti-pattern.
func TestSynctestWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		results := make([]int, 3)
		for i := range 3 {
			go func() { results[i] = i * 10 }() // each writes a distinct index, so no race
		}

		// Block until every OTHER goroutine in the bubble is finished or durably blocked.
		// No sleeping, no polling, and no flakiness.
		synctest.Wait()

		want := []int{0, 10, 20}
		for i := range want {
			if results[i] != want[i] {
				t.Errorf("results[%d] = %d, want %d", i, results[i], want[i])
			}
		}
	})
}

// TestSynctestSleepAdvancesTheClock shows the distinction that trips people up: Wait() returns as
// soon as the other goroutines are DURABLY BLOCKED - and a sleeping goroutine is durably blocked.
// It does not itself move the clock. synctest.Sleep (Go 1.27) advances the clock and then waits.
func TestSynctestSleepAdvancesTheClock(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		var finished []time.Duration

		for _, d := range []time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second} {
			go func() {
				time.Sleep(d)
				mu.Lock()
				finished = append(finished, d)
				mu.Unlock()
			}()
		}

		// Wait() alone proves nothing here: all three are asleep, hence durably blocked.
		synctest.Wait()
		mu.Lock()
		afterWait := len(finished)
		mu.Unlock()
		if afterWait != 0 {
			t.Errorf("after Wait(), %d goroutines had finished, want 0 - Wait does not move the clock",
				afterWait)
		}

		// Sleep() moves the fake clock forward 2.5s, so the 1s and 2s sleepers wake.
		synctest.Sleep(2500 * time.Millisecond)
		mu.Lock()
		afterSleep := len(finished)
		mu.Unlock()
		if afterSleep != 2 {
			t.Errorf("after Sleep(2.5s), %d goroutines had finished, want 2", afterSleep)
		}

		// synctest.Test waits for every bubbled goroutine to exit before returning; a goroutine
		// that can never finish deadlocks the bubble and FAILS the test - which is how a leak
		// becomes a test failure. This Sleep is illustrative, not required: Test would advance
		// the clock and let the last sleeper finish on its own.
		synctest.Sleep(time.Second)
	})
}

// TestSynctestTimeout is the shape this makes easy: asserting on a real timeout, instantly.
func TestSynctestTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const timeout = 5 * time.Minute

		slow := make(chan int)
		go func() {
			time.Sleep(10 * time.Minute) // never arrives within the budget
			slow <- 1
		}()

		start := time.Now()
		select {
		case <-slow:
			t.Error("the slow operation should not have completed within the timeout")
		case <-time.After(timeout):
			if elapsed := time.Since(start); elapsed != timeout {
				t.Errorf("timed out after %v, want exactly %v", elapsed, timeout)
			}
		}

		// synctest.Test waits for the sleeping goroutine to finish (advancing the fake clock as
		// needed); a goroutine blocked forever would fail the test. This Sleep is illustrative.
		synctest.Sleep(10 * time.Minute) // Go 1.27: advance the clock and wait
		<-slow
	})
}

// =================================================================================================
// Section 7: Test doubles, httptest and the external test package
// =================================================================================================

// m014Store is the small, consumer-side interface the code under test depends on (module 008).
type m014Store interface {
	Get(key string) (string, error)
}

// m014FakeStore is the whole mocking framework you need.
type m014FakeStore struct {
	data map[string]string
	err  error
	// calls records what was asked for, so the test can assert on the interaction.
	calls []string
}

func (f *m014FakeStore) Get(key string) (string, error) {
	f.calls = append(f.calls, key)
	if f.err != nil {
		return "", f.err
	}
	v, ok := f.data[key]
	if !ok {
		return "", fmt.Errorf("m014FakeStore: %q: %w", key, m009ErrNotFound)
	}
	return v, nil
}

// m014Greet is the unit under test: it depends on the interface, not on a concrete store.
func m014Greet(s m014Store, key string) string {
	name, err := s.Get(key)
	if err != nil {
		return "Hello, stranger!"
	}
	return "Hello, " + name + "!"
}

func TestWithATestDouble(t *testing.T) {
	fake := &m014FakeStore{data: map[string]string{"user": "Ada"}}

	m014AssertEqual(t, m014Greet(fake, "user"), "Hello, Ada!", `m014Greet(fake, "user")`)
	m014AssertEqual(t, m014Greet(fake, "absent"), "Hello, stranger!", `m014Greet(fake, "absent")`)

	// Assert on the interaction as well as the result.
	if len(fake.calls) != 2 {
		t.Errorf("fake.calls = %v, want 2 calls", fake.calls)
	}

	// A failing store, with no framework in sight.
	broken := &m014FakeStore{err: errors.New("connection refused")}
	m014AssertEqual(t, m014Greet(broken, "user"), "Hello, stranger!", "m014Greet(broken)")
}

func TestReaderDouble(t *testing.T) {
	// strings.NewReader substitutes for a file: no filesystem, no mocking library.
	m014AssertEqual(t, m008CountLines(strings.NewReader("a\nb\nc")), 3, "m008CountLines")
	m014AssertEqual(t, m008CountLines(strings.NewReader("")), 0, "m008CountLines(empty)")
}

func TestHTTPTestServer(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/fail" {
			http.Error(w, "nope", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "hello from %s", r.URL.Path)
	})

	// httptest.NewTestServer (Go 1.27) registers its own t.Cleanup, so there is no
	// `defer srv.Close()` to forget. Note it returns an UNSTARTED server: you still call
	// Start() (or StartTLS()) yourself, unlike httptest.NewServer which starts immediately.
	srv := httptest.NewTestServer(t, handler)
	srv.Start()

	resp, err := http.Get(srv.URL + "/greeting")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body) // drain so the keep-alive connection can be reused
	m014AssertEqual(t, resp.StatusCode, http.StatusOK, "status")

	// ResponseRecorder tests a handler with no server at all.
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/fail", nil))
	m014AssertEqual(t, rec.Code, http.StatusInternalServerError, "recorder status")
	m014AssertEqual(t, strings.TrimSpace(rec.Body.String()), "nope", "recorder body")
}

func TestExternalPackageStyle(t *testing.T) {
	// This file is `package basics`, so it can see m014Reverse and every other unexported name.
	// A file declaring `package basics_test` in this same directory would see only the exported
	// API - Run001a, Modules, RunAll - which is a useful discipline: it tests what callers can
	// actually reach, and it breaks import cycles.
	t.Log("internal test package: unexported identifiers are visible")

	// t.Attr (Go 1.25) attaches structured metadata to the result, for CI to pick up.
	t.Attr("module", "014")
	t.Attr("topic", "testing")

	if len(Modules) == 0 {
		t.Fatal("Modules registry is empty")
	}
	for _, m := range Modules {
		if m.ID == "" || m.Title == "" || m.Run == nil {
			t.Errorf("module %+v is incompletely registered", m)
		}
	}
}

package basics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync/atomic"
	"time"
)

/*
# Module 018 — HTTP Services

`net/http` is a complete, production-grade HTTP client and server in the standard library. Go was
designed at a company that runs servers, and it shows: TLS, HTTP/2, connection pooling, graceful
shutdown and a routing language are all here, with no framework required.

Since **Go 1.22** the built-in `ServeMux` understands methods and path wildcards, which removed the
last common reason to reach for a third-party router. This module builds a small JSON service with
nothing but the standard library, and finishes with the part every tutorial skips: **shutting it
down properly**.

Everything here runs offline. The demo server binds `127.0.0.1:0` — an ephemeral loopback port —
serves the demo's own requests, and stops before the section ends.
*/

// =================================================================================================
// Section 1: Handlers
// =================================================================================================

/*
## Handlers

- The whole server side rests on one interface:

	type Handler interface {
	    ServeHTTP(ResponseWriter, *Request)
	}

- **`http.HandlerFunc`** adapts a plain function to it. It is a *defined function type with a
  method* — the technique from module 007, Section 1 — and it is why you can write a handler as a
  closure and still satisfy an interface.
- **`http.ResponseWriter`** is an interface with three methods: `Header()`, `Write()`,
  `WriteHeader(status)`. The order matters and is the most common beginner bug:
    1. set headers **first** — after the body starts they are ignored
    2. `WriteHeader(status)` **once** — a second call logs "superfluous WriteHeader"
    3. then write the body; the first `Write` implies `WriteHeader(http.StatusOK)`
- **`*http.Request`** carries the method, URL, headers, and `Body` (an `io.ReadCloser` the server
  closes for you). `r.Context()` is cancelled when the client disconnects — pass it to every
  downstream call so a hung request does not outlive its caller.
- **Always limit the request body.** `http.MaxBytesReader(w, r.Body, n)` caps it; without one, a
  client can stream gigabytes into your `io.ReadAll`.
- A handler must be **safe for concurrent use**: the server runs each request in its own goroutine.
  Anything shared needs a mutex or an atomic (module 011, Section 4).
*/

func m018Handlers() {
	fmt.Println("--- Section 1: Handlers ---")

	// A handler as a struct with state, and the same thing as a function.
	var counter atomic.Int64
	structHandler := &m018CountingHandler{count: &counter}
	funcHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8") // 1. headers first
		w.WriteHeader(http.StatusTeapot)                            // 2. status once
		fmt.Fprintf(w, "method=%s path=%s", r.Method, r.URL.Path)   // 3. then the body
	})

	// httptest.NewRecorder runs a handler with no server at all (module 014 §7).
	rec := httptest.NewRecorder()
	structHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/a", nil))
	structHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/b", nil))
	fmt.Printf("  a Handler with state: after 2 requests count=%d\n", counter.Load())

	rec = httptest.NewRecorder()
	funcHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/teapot", nil))
	fmt.Printf("  a HandlerFunc: status=%d contentType=%q body=%q\n",
		rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())

	// --- The ordering rule ---
	wrong := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("body first"))                 // this implies WriteHeader(200)
		w.Header().Set("X-Too-Late", "ignored")       // set after the body: never sent
		w.WriteHeader(http.StatusInternalServerError) // superfluous: logged, not applied
	})
	rec = httptest.NewRecorder()
	wrong.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	// rec.Header() is the LIVE map (it would still show the late value); rec.Result() is the
	// snapshot of what was actually sent, which is what a client would see.
	fmt.Printf("  headers set after the body are dropped: X-Too-Late=%q, status stayed %d\n",
		rec.Result().Header.Get("X-Too-Late"), rec.Code)
	fmt.Println("  order is: Header().Set(...) -> WriteHeader(status) -> Write(body)")

	// --- Limiting the body ---
	limited := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 16) // cap it before reading a single byte
		data, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		fmt.Fprintf(w, "read %d bytes", len(data))
	})
	for _, body := range []string{"small", strings.Repeat("x", 100)} {
		rec = httptest.NewRecorder()
		limited.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))
		fmt.Printf("  MaxBytesReader with a %3d-byte body: status=%d %q\n",
			len(body), rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	// --- The request context ---
	fmt.Println("  r.Context() is cancelled when the client disconnects - pass it downstream")
	fmt.Println("  every request runs in its own goroutine, so shared state needs a mutex")
}

type m018CountingHandler struct{ count *atomic.Int64 }

func (h *m018CountingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.count.Add(1)
	fmt.Fprintln(w, "ok")
}

// =================================================================================================
// Section 2: Routing with ServeMux (Go 1.22 patterns)
// =================================================================================================

/*
## Routing with ServeMux (Go 1.22 patterns)

Before Go 1.22, `ServeMux` matched paths and nothing else: no methods, no path parameters. Every
project therefore imported `gorilla/mux`, `chi` or `gin`. **Go 1.22 changed the pattern language**,
and for most services the standard library is now enough.

A pattern is `[METHOD ]/path`:

	mux.HandleFunc("/items", h)             // any method, exact path
	mux.HandleFunc("GET /items", h)         // GET only
	mux.HandleFunc("GET /items/{id}", h)    // a wildcard segment
	mux.HandleFunc("GET /files/{path...}", h) // a trailing wildcard: matches the rest
	mux.HandleFunc("GET /items/{$}", h)     // {$} anchors: matches /items/ EXACTLY

- **`r.PathValue("id")`** reads a wildcard. `r.SetPathValue` exists for tests and middleware.
- Registering a method-less pattern matches **all** methods; registering `GET /x` makes any other
  method to `/x` return **405 Method Not Allowed** automatically, with an `Allow` header.
- **Precedence is by specificity, not registration order**: the most specific pattern wins, and
  `ServeMux` **panics at registration time** if two patterns are ambiguous (neither is more specific
  than the other). A conflict is a startup crash, not a silent mis-route — which is what you want.
- A trailing slash still means a **subtree**: `/static/` matches everything beneath it. Use `{$}` to
  match only the exact path.
- `http.StripPrefix`, `http.FileServer` and `http.FS` (over an `embed.FS`, module 015 §3) serve
  static files in three lines.
- **What a framework still buys you**: grouped routes with shared middleware, automatic binding and
  validation, and a large ecosystem. This repo's `examples/rest.go` uses Gin, which predates 1.22.
  For a new service, start with `net/http` and add a router when you can name what it gives you.
*/

func m018Routing() {
	fmt.Println("\n--- Section 2: Routing with ServeMux (Go 1.22 patterns) ---")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /items", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "list of items")
	})
	mux.HandleFunc("POST /items", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "created an item")
	})
	mux.HandleFunc("GET /items/{id}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "item %s", r.PathValue("id"))
	})
	mux.HandleFunc("GET /items/{id}/tags/{tag}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "item %s tag %s", r.PathValue("id"), r.PathValue("tag"))
	})
	mux.HandleFunc("GET /files/{path...}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "file at %q", r.PathValue("path"))
	})
	mux.HandleFunc("GET /exact/{$}", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "matched /exact/ exactly")
	})

	// HandleFunc wants a FUNCTION; a type implementing http.Handler goes to mux.Handle:
	//	mux.HandleFunc("GET /x", m018CountingHandler{}) // ERROR: cannot use m018CountingHandler{} (value of struct type m018CountingHandler) as func(http.ResponseWriter, *http.Request) value in argument to mux.HandleFunc
	// and the signature is fixed - both parameters, in that order:
	//	mux.HandleFunc("GET /y", func(w http.ResponseWriter) {}) // ERROR: cannot use func(w http.ResponseWriter) {} (value of type func(w http.ResponseWriter)) as func(http.ResponseWriter, *http.Request) value in argument to mux.HandleFunc

	for _, probe := range []struct{ method, path string }{
		{http.MethodGet, "/items"},
		{http.MethodPost, "/items"},
		{http.MethodDelete, "/items"},
		{http.MethodGet, "/items/42"},
		{http.MethodGet, "/items/42/tags/urgent"},
		{http.MethodGet, "/files/a/b/c.txt"},
		{http.MethodGet, "/exact/"},
		{http.MethodGet, "/exact/deeper"},
		{http.MethodGet, "/nothing-here"},
	} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(probe.method, probe.path, nil))
		body := strings.TrimSpace(rec.Body.String())
		extra := ""
		if allow := rec.Header().Get("Allow"); allow != "" {
			extra = fmt.Sprintf("  Allow: %s", allow)
		}
		fmt.Printf("  %-6s %-22s -> %d %s%s\n", probe.method, probe.path, rec.Code, body, extra)
	}

	fmt.Println("  DELETE /items got 405 with an Allow header, generated by ServeMux itself")
	fmt.Println("  /items/42 matched /items/{id}; had a literal GET /items/42 been registered too,")
	fmt.Println("  it would win: precedence is by SPECIFICITY, not by registration order")

	// --- An ambiguous pair panics at registration, not at request time ---
	conflict := fmt.Sprint(m005CatchPanic(func() {
		bad := http.NewServeMux()
		bad.HandleFunc("GET /a/{x}/c", func(http.ResponseWriter, *http.Request) {})
		bad.HandleFunc("GET /a/b/{y}", func(http.ResponseWriter, *http.Request) {})
	}))
	fmt.Println("  registering `GET /a/{x}/c` and `GET /a/b/{y}` panics at registration:")
	for i, line := range strings.Split(conflict, "\n") {
		if i >= 3 {
			fmt.Println("    ...")
			break
		}
		fmt.Printf("    %s\n", strings.TrimSpace(line))
	}
	fmt.Println("  a routing conflict is a STARTUP crash, never a silent mis-route")
}

// =================================================================================================
// Section 3: Middleware
// =================================================================================================

/*
## Middleware

Middleware in Go needs no framework support, because a handler that wraps a handler is just a
function:

	func Middleware(next http.Handler) http.Handler {
	    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	        // before
	        next.ServeHTTP(w, r)
	        // after
	    })
	}

- Composition is function application, so a small helper chains them:
  `Chain(mux, Logging, Recovery, RequestID)`. Order matters and reads **outermost first** — the
  first in the list sees the request first and the response last.
- **`Recovery` must be outermost.** A panic in a handler would otherwise kill... actually it would
  not: `net/http` recovers panics per connection so one bad request cannot take the server down.
  But it logs a stack trace and closes the connection with no response. Your own recovery
  middleware turns that into a clean `500` — and lets you log it the way you log everything else.
- To record the **status code** you must wrap `ResponseWriter`, because it has no getter. Wrapping
  it correctly is fiddly: a naive wrapper hides `http.Flusher`, `http.Hijacker` and
  `io.ReaderFrom`, which breaks streaming, WebSockets and `io.Copy` fast paths. Implement the
  interfaces you need, or use `http.ResponseController` (Go 1.20), which reaches through wrappers.
- **Request-scoped values** go in the context, with an unexported key type (module 011, Section 5).
  A request ID set by middleware and read by every log line is the canonical use.
- Middleware that applies to a subset of routes: register a second `ServeMux` and wrap that, or
  wrap the individual handlers. `net/http` has no route groups — that *is* something a router buys.
*/

// m018Middleware is the shape every middleware takes.
type m018Middleware func(http.Handler) http.Handler

// m018Chain applies middleware so that the FIRST listed is the outermost.
func m018Chain(h http.Handler, mw ...m018Middleware) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}

type m018ctxKey string

const m018RequestIDKey m018ctxKey = "requestID"

func m018RequestID(next http.Handler) http.Handler {
	var seq atomic.Int64
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := fmt.Sprintf("req-%03d", seq.Add(1))
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), m018RequestIDKey, id)))
	})
}

func m018Logging(log *slog.Logger) m018Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &m018StatusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)
			id, _ := r.Context().Value(m018RequestIDKey).(string)
			log.Info("request",
				slog.String("id", id),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rw.status),
				slog.Int("bytes", rw.written),
				slog.Bool("fast", time.Since(start) < time.Second))
		})
	}
}

func m018Recovery(log *slog.Logger) m018Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered", slog.Any("value", rec), slog.String("path", r.URL.Path))
					m018WriteError(w, r, m018APIError{
						Status: http.StatusInternalServerError,
						Code:   "internal",
						Detail: "the server encountered an unexpected condition",
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// m018StatusRecorder captures the status and byte count. Note the Flush/Unwrap methods: a naive
// wrapper would hide http.Flusher and break streaming.
type m018StatusRecorder struct {
	http.ResponseWriter
	status  int
	written int
	wrote   bool
}

func (r *m018StatusRecorder) WriteHeader(status int) {
	if r.wrote {
		return // a second WriteHeader is a no-op, as in the real ResponseWriter
	}
	r.status, r.wrote = status, true
	r.ResponseWriter.WriteHeader(status)
}

func (r *m018StatusRecorder) Write(b []byte) (int, error) {
	if !r.wrote {
		r.WriteHeader(http.StatusOK)
	}
	n, err := r.ResponseWriter.Write(b)
	r.written += n
	return n, err
}

// Unwrap lets http.ResponseController reach the real ResponseWriter through this wrapper.
func (r *m018StatusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func m018MiddlewareSection() {
	fmt.Println("\n--- Section 3: Middleware ---")

	log := m018QuietLogger()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ok", func(w http.ResponseWriter, r *http.Request) {
		id, _ := r.Context().Value(m018RequestIDKey).(string)
		fmt.Fprintf(w, "handled %s", id)
	})
	mux.HandleFunc("GET /boom", func(w http.ResponseWriter, r *http.Request) {
		panic("something went badly wrong")
	})

	// Outermost first: Recovery wraps everything (so a panic in Logging or RequestID is caught
	// too), then RequestID, then Logging, which is the closest to the mux.
	handler := m018Chain(mux, m018Recovery(log), m018RequestID, m018Logging(log))

	for _, path := range []string{"/ok", "/ok", "/boom"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		fmt.Printf("  GET %-6s -> %d  X-Request-Id=%s  body=%s\n",
			path, rec.Code, rec.Header().Get("X-Request-Id"),
			strings.TrimSpace(rec.Body.String()))
	}

	fmt.Println("  the panic became a clean 500 with a JSON body, and the request ID survived")
	fmt.Println("  Recovery must be OUTERMOST of anything that could panic")
	fmt.Println("  the status recorder implements Unwrap, so http.ResponseController (Go 1.20)")
	fmt.Println("  can still reach Flush and SetWriteDeadline through it")
}

// m018QuietLogger writes to stdout without timestamps, so the course output stays reproducible.
func m018QuietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	}))
}

// =================================================================================================
// Section 4: Consistent Error Handling
// =================================================================================================

/*
## Consistent Error Handling

`http.Handler` returns nothing, so the compiler cannot make you handle an error — which is exactly
why HTTP handlers drift into inconsistency: some return plain text, some JSON, some forget the
status code, some leak an internal message to the client.

Two techniques fix it, and they compose:

**1. One error type and one writer.** Define an API error carrying a status, a stable machine
code and a human detail, and exactly one function that serialises it. Every failure path goes
through that function, so every error response has the same shape.

**2. Handlers that return `error`.** Define your own handler signature and adapt it:

	type APIHandler func(http.ResponseWriter, *http.Request) error

	func (h APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	    if err := h(w, r); err != nil {
	        WriteError(w, r, err)
	    }
	}

Now a handler can `return err` instead of remembering to write *and* `return` — which removes the
classic bug of writing an error and then falling through into the success path.

Other rules that matter:

  - **Never leak internal detail.** Map an unexpected error to a generic 500 and log the real one
    with the request ID. `sql: no rows in result set` is not a client's business.
  - Map **sentinel errors** to statuses at the boundary (module 009, Section 2): `ErrNotFound` → 404,
    `ErrPermission` → 403, `context.DeadlineExceeded` → 504.
  - Use `http.Error` only for plain-text services. A JSON API must answer JSON, including on errors,
    or every client needs two parsers.
  - Set `Content-Type` before the status, and remember `http.Error` sets
    `X-Content-Type-Options: nosniff` for you.
*/

// m018APIError is the single error shape this service ever returns.
type m018APIError struct {
	Status int    `json:"-"`
	Code   string `json:"code"`
	Detail string `json:"detail"`
	ReqID  string `json:"requestId,omitempty"`
}

func (e m018APIError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Detail) }

// m018WriteJSON is the one place a success body is serialised.
func m018WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// The status is already sent, so we cannot change the response - but do not lose the
		// error either: a half-written body is worth a log line.
		slog.Warn("write json body", slog.Any("err", err))
	}
}

// m018WriteError is the one place a failure is serialised. Everything funnels through it.
func m018WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr m018APIError
	if !errors.As(err, &apiErr) {
		// An unexpected error: map it, and never show the client the real message.
		apiErr = m018APIError{
			Status: http.StatusInternalServerError,
			Code:   "internal",
			Detail: "the server encountered an unexpected condition",
		}
	}
	if id, ok := r.Context().Value(m018RequestIDKey).(string); ok {
		apiErr.ReqID = id
	}
	m018WriteJSON(w, apiErr.Status, apiErr)
}

// m018APIHandler lets a handler return an error instead of remembering to write and return.
type m018APIHandler func(http.ResponseWriter, *http.Request) error

func (h m018APIHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h(w, r); err != nil {
		m018WriteError(w, r, err)
	}
}

// m018MapDomainError turns the domain's sentinels into API errors, at the boundary and nowhere else.
func m018MapDomainError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, m009ErrNotFound):
		return m018APIError{Status: http.StatusNotFound, Code: "not_found", Detail: "no such item"}
	case errors.Is(err, m009ErrPermission):
		return m018APIError{Status: http.StatusForbidden, Code: "forbidden", Detail: "not allowed"}
	case errors.Is(err, context.DeadlineExceeded):
		return m018APIError{Status: http.StatusGatewayTimeout, Code: "timeout", Detail: "upstream timed out"}
	default:
		return err // unexpected: m018WriteError turns it into a generic 500
	}
}

func m018ErrorHandling() {
	fmt.Println("\n--- Section 4: Consistent Error Handling ---")

	mux := http.NewServeMux()
	mux.Handle("GET /items/{id}", m018APIHandler(func(w http.ResponseWriter, r *http.Request) error {
		switch id := r.PathValue("id"); id {
		case "1":
			m018WriteJSON(w, http.StatusOK, map[string]any{"id": 1, "name": "first"})
			return nil
		case "missing":
			return m018MapDomainError(fmt.Errorf("lookup %q: %w", id, m009ErrNotFound))
		case "secret":
			return m018MapDomainError(fmt.Errorf("lookup %q: %w", id, m009ErrPermission))
		case "slow":
			return m018MapDomainError(fmt.Errorf("lookup %q: %w", id, context.DeadlineExceeded))
		default:
			// An unexpected internal failure, with a message the client must never see.
			return fmt.Errorf("database connection string is postgres://admin:hunter2@db/prod")
		}
	}))

	handler := m018Chain(mux, m018RequestID)
	for _, id := range []string{"1", "missing", "secret", "slow", "kaboom"} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/items/"+id, nil))
		fmt.Printf("  GET /items/%-8s -> %d %s\n", id, rec.Code, strings.TrimSpace(rec.Body.String()))
	}

	fmt.Println("  every response is JSON, every error has the same shape, and the last one")
	fmt.Println("  did NOT leak the connection string - it became a generic `internal`")
	fmt.Println("  the handler could `return err` instead of writing-then-remembering-to-return")
}

// =================================================================================================
// Section 5: http.Server and Timeouts
// =================================================================================================

/*
## http.Server and Timeouts

**`http.ListenAndServe(addr, handler)` is a convenience for demos, not for production**, because it
uses a zero-value `http.Server` — and a zero `http.Server` has **no timeouts at all**. A client that
opens a connection and never sends a byte holds a goroutine and a file descriptor forever. That is
the whole Slowloris attack, and it needs no tooling.

Always construct the server explicitly:

	srv := &http.Server{
	    Addr:              ":8080",
	    Handler:           mux,
	    ReadHeaderTimeout: 5 * time.Second,   // headers must arrive quickly — the Slowloris fix
	    ReadTimeout:       15 * time.Second,  // the whole request, headers plus body
	    WriteTimeout:      30 * time.Second,  // from end of headers to end of response
	    IdleTimeout:       120 * time.Second, // keep-alive connections between requests
	    MaxHeaderBytes:    1 << 20,
	    ErrorLog:          slog.NewLogLogger(logHandler, slog.LevelError), // an slog.Handler
	}

- **`ReadHeaderTimeout` is the one you must not omit.** It is the only one that bounds a connection
  that has sent nothing.
- `WriteTimeout` is a hard deadline on the whole response, so it must exceed your slowest legitimate
  handler. For streaming or long-polling endpoints, leave it off and use
  `http.ResponseController.SetWriteDeadline` per request instead.
- **`BaseContext` and `ConnContext`** let you attach values or cancellation to every request's
  context — that is how you make a shutdown signal reach in-flight handlers.
- Bind explicitly with **`net.Listen`** when you need the actual address: passing `:0` asks the OS
  for a free port, and `ln.Addr()` tells you which one. That is what this demo does, and what tests
  should do instead of hard-coding a port.
- `srv.Serve(ln)` and `srv.ListenAndServe()` both return **`http.ErrServerClosed`** after a clean
  shutdown. That is a success, not a failure — treat it as such (Section 6).
- For TLS, `ListenAndServeTLS` plus `golang.org/x/crypto/acme/autocert` gets you automatic Let's
  Encrypt certificates. HTTP/2 is enabled automatically over TLS.
*/

// m018NewServer builds a properly configured server. Callers supply the listener.
func m018NewServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

func m018ServerAndTimeouts() {
	fmt.Println("\n--- Section 5: http.Server and Timeouts ---")

	before := runtime.NumGoroutine()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello/{name}", func(w http.ResponseWriter, r *http.Request) {
		m018WriteJSON(w, http.StatusOK, map[string]string{"hello": r.PathValue("name")})
	})

	// Bind an ephemeral loopback port: no fixed port to collide, no macOS firewall prompt.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Println("  could not listen:", err)
		return
	}
	srv := m018NewServer(mux)

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	base := "http://" + ln.Addr().String()
	fmt.Printf("  listening on %s (port 0 = the OS picked a free one)\n", base)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(base + "/hello/gopher")
	if err != nil {
		fmt.Println("  request failed:", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("  GET /hello/gopher -> %d %s", resp.StatusCode, body)
	}

	// A real 404 from a real server.
	resp, err = client.Get(base + "/nothing")
	if err == nil {
		resp.Body.Close()
		fmt.Printf("  GET /nothing      -> %d\n", resp.StatusCode)
	}

	// Shut it down and confirm the sentinel.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	fmt.Printf("  Serve returned %v — which means a CLEAN shutdown, not a failure\n", <-serveErr)

	fmt.Println("  a zero-value http.Server has NO timeouts: one silent connection holds a")
	fmt.Println("  goroutine and a file descriptor forever. ReadHeaderTimeout is the fix.")

	// Give the server's connection goroutines a moment to unwind before counting.
	time.Sleep(50 * time.Millisecond)
	fmt.Printf("  goroutines before=%d after=%d (no leak)\n", before, runtime.NumGoroutine())
}

// =================================================================================================
// Section 6: Graceful Shutdown and OS Signals
// =================================================================================================

/*
## Graceful Shutdown and OS Signals

When a container is stopped, Kubernetes sends **SIGTERM** and waits (30 seconds by default) before
sending **SIGKILL**, which cannot be caught. A process that ignores SIGTERM drops every in-flight
request. Handling it is a dozen lines and it is the difference between a rolling deploy nobody
notices and one that returns 502s.

### Catching the signal

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

**`signal.NotifyContext`** (Go 1.16) returns a context cancelled on the first matching signal.
It replaces the older `signal.Notify` plus a channel, and it composes with everything that already
takes a context.

  - `os.Interrupt` is SIGINT (Ctrl+C) and is portable; `syscall.SIGTERM` is what orchestrators send.
  - **`stop()` restores the default behaviour**, so a *second* Ctrl+C kills a process that is
    taking too long to drain. Deferring it is not optional politeness — without it, an operator
    cannot interrupt a wedged shutdown.
  - A signal arriving before `Notify` is registered uses the **default** action, which for SIGINT
    and SIGTERM is to terminate. Register early, before you start serving.

### Draining

	<-ctx.Done()                       // a signal arrived
	shutCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	err := srv.Shutdown(shutCtx)

**`srv.Shutdown`** stops accepting new connections, closes idle keep-alive connections, and waits
for in-flight handlers to finish. It returns when they are all done, or when *its* context expires —
at which point the remaining connections are abandoned. Give it less time than the orchestrator's
grace period, or SIGKILL will arrive mid-drain.

`srv.Close()` is the un-graceful sibling: it drops everything immediately. Use it as a last resort
after `Shutdown` times out.

**Note what `Shutdown` does not do**: it does not cancel the *contexts* of running handlers. A
handler blocked forever will block the shutdown until the timeout. If handlers must be told to stop,
pass them a separate cancellable context via `BaseContext`.

### Order matters

Release resources in the reverse of the order you acquired them, and only **after** the server has
drained — a handler still running needs its database:

	1. stop accepting requests, drain in-flight   (srv.Shutdown)
	2. close the database pool                    (db.Close)
	3. flush telemetry, close the log file
	4. return from main

`errgroup` (`golang.org/x/sync`) is the usual way to run the server and the signal watcher together
and collect the first error.
*/

func m018GracefulShutdown() {
	fmt.Println("\n--- Section 6: Graceful Shutdown and OS Signals ---")

	before := runtime.NumGoroutine()

	// A handler that takes long enough to still be in flight when the signal arrives.
	var completed atomic.Int64
	mux := http.NewServeMux()
	mux.HandleFunc("GET /slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		completed.Add(1)
		fmt.Fprint(w, "finished")
	})

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Println("  could not listen:", err)
		return
	}
	srv := m018NewServer(mux)
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	base := "http://" + ln.Addr().String()

	// --- Arm the signal handler BEFORE anything can send one ---
	// os.Interrupt is portable. Once NotifyContext returns, the signal is caught rather than
	// terminating the process - which is what makes the self-signal below safe.
	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop() // restores the default, so a second Ctrl+C can still kill a wedged shutdown
	fmt.Println("  signal handler armed for SIGINT (os.Interrupt)")

	// Start a slow request, so shutdown has something to drain.
	inFlight := make(chan int, 1)
	go func() {
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(base + "/slow")
		if err != nil {
			inFlight <- 0
			return
		}
		defer resp.Body.Close()
		if _, err := io.Copy(io.Discard, resp.Body); err != nil {
			fmt.Println("  draining response body:", err)
		}
		inFlight <- resp.StatusCode
	}()
	time.Sleep(50 * time.Millisecond) // let the request reach the handler

	// --- Send ourselves the signal, exactly as an orchestrator would ---
	sent := false
	if p, err := os.FindProcess(os.Getpid()); err == nil {
		if err := p.Signal(os.Interrupt); err == nil {
			sent = true
		}
	}
	if !sent {
		// Windows cannot signal itself; fall back so the demo still shows the drain.
		fmt.Println("  (this platform cannot signal itself - cancelling the context instead)")
		stop()
	}

	select {
	case <-sigCtx.Done():
		fmt.Println("  signal received; the context is cancelled and shutdown begins")
	case <-time.After(2 * time.Second):
		fmt.Println("  no signal arrived within 2s - continuing anyway")
	}

	// --- Drain with a deadline ---
	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	shutdownErr := srv.Shutdown(shutCtx)
	elapsed := time.Since(start)

	status := <-inFlight
	fmt.Printf("  Shutdown returned %v after %v\n", shutdownErr, elapsed.Round(10*time.Millisecond))
	fmt.Printf("  the in-flight request still completed: status=%d handlersFinished=%d\n",
		status, completed.Load())
	fmt.Printf("  Serve returned %v\n", <-serveErr)

	fmt.Println()
	fmt.Println("  that is the whole contract: stop accepting, let in-flight requests finish,")
	fmt.Println("  return when they are done or when the deadline expires")
	fmt.Println("  release order: drain the server, THEN close the database, THEN flush telemetry")
	fmt.Println("  give Shutdown less time than the orchestrator's grace period (K8s: 30s),")
	fmt.Println("  or SIGKILL arrives mid-drain")
	fmt.Println("  `defer stop()` matters: it lets a second Ctrl+C kill a wedged shutdown")

	// stop() ends the context watcher and restores the default signal behaviour. It is
	// idempotent, so the deferred stop() above remains correct.
	stop()
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
	time.Sleep(50 * time.Millisecond)
	fmt.Printf("  goroutines before=%d after=%d\n", before, runtime.NumGoroutine())
	fmt.Println("  the extra one is os/signal.loop, started by the process's FIRST signal.Notify")
	fmt.Println("  and never exiting by design - a permanent runtime goroutine, not a leak.")
	fmt.Println("  Worth knowing before you assert on NumGoroutine in a test.")
}

// =================================================================================================
// Section 7: Testing HTTP Handlers
// =================================================================================================

/*
## Testing HTTP Handlers

`net/http/httptest` makes HTTP the easiest part of a Go codebase to test, because a handler is just
a function and a request is just a struct.

  - **`httptest.NewRequest(method, target, body)`** builds a `*http.Request` with no network. It
    panics on a malformed target — deliberately, since a test's own input should not be in doubt.
  - **`httptest.NewRecorder()`** is a `ResponseWriter` that records. Afterwards, `rec.Code`,
    `rec.Body`, `rec.Header()` and `rec.Result()` hold everything. This is the fastest way to test a
    handler and needs no server at all.
  - **`httptest.NewServer(handler)`** starts a real server on a real loopback port, for testing a
    *client*, TLS behaviour, or anything involving actual connections. `httptest.NewTLSServer` does
    the same with a self-signed certificate, and `srv.Client()` returns a client that trusts it.
  - **`httptest.NewTestServer(t, handler)`** (**Go 1.27**) registers its own `t.Cleanup`, so there
    is no `defer srv.Close()` to forget. Note it returns an **unstarted** server: call `srv.Start()`.
  - Testing a handler that reads a **path wildcard** without going through the mux needs
    `r.SetPathValue("id", "42")`, otherwise `PathValue` returns `""`.
  - Assert on **status, headers and the decoded body** — not on the raw JSON string, which breaks
    whenever a field is added or reordered.
  - `http.RoundTripper` is the client-side seam: a fake transport substitutes for an entire remote
    service with no server.

Module 014 covers the testing techniques themselves; `mod_014_testing_test.go` has a runnable
`TestHTTPTestServer`.
*/

func m018TestingHandlers() {
	fmt.Println("\n--- Section 7: Testing HTTP Handlers ---")

	handler := m018APIHandler(func(w http.ResponseWriter, r *http.Request) error {
		id := r.PathValue("id")
		if id == "" {
			return m018APIError{Status: http.StatusBadRequest, Code: "bad_request", Detail: "id required"}
		}
		m018WriteJSON(w, http.StatusOK, map[string]string{"id": id})
		return nil
	})

	// 1. Recorder, no server, no mux.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/items/42", nil)
	req.SetPathValue("id", "42") // without this, PathValue is "" outside a mux
	handler.ServeHTTP(rec, req)

	var decoded map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &decoded)
	fmt.Printf("  NewRecorder: status=%d contentType=%q decoded=%v\n",
		rec.Code, rec.Header().Get("Content-Type"), decoded)
	fmt.Println("  assert on the DECODED body, not the raw JSON string")

	// The missing-wildcard case.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/items/", nil))
	fmt.Printf("  without SetPathValue: status=%d body=%s\n", rec.Code, strings.TrimSpace(rec.Body.String()))

	// 2. A real server, for testing a client.
	mux := http.NewServeMux()
	mux.Handle("GET /items/{id}", handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/items/99")
	if err == nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("  httptest.NewServer at %s -> %d %s", srv.URL, resp.StatusCode, body)
	}
	fmt.Println("  httptest.NewTestServer(t, h) (Go 1.27) registers its own cleanup, but returns")
	fmt.Println("  an UNSTARTED server - call srv.Start() yourself")

	// 3. A fake RoundTripper: an entire remote service, with no server.
	fakeClient := &http.Client{Transport: m018FakeTransport(func(r *http.Request) *http.Response {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"source":"fake transport"}`)),
		}
	})}
	resp, err = fakeClient.Get("https://api.example.com/anything")
	if err == nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("  a fake RoundTripper stands in for a whole remote API: %s\n", body)
	}
	fmt.Println("  see TestHTTPTestServer in mod_014_testing_test.go for the real thing")
}

// m018FakeTransport adapts a function to http.RoundTripper - the client-side seam.
type m018FakeTransport func(*http.Request) *http.Response

func (f m018FakeTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r), nil }

// Run018 runs every section of module 018 in order.
func Run018() {
	m018Handlers()
	m018Routing()
	m018MiddlewareSection()
	m018ErrorHandling()
	m018ServerAndTimeouts()
	m018GracefulShutdown()
	m018TestingHandlers()
}

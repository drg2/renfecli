package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seifreed/renfecli/internal/client"
	"github.com/seifreed/renfecli/internal/store"
)

// The scaffolding every command test shares: a stand-in for venta.renfe.com, a
// throwaway ~/.renfe, and a way to read what a command printed.

// renfeServer answers any DWR call with the script dwr returns for that path,
// and anything else with a page — which is all the client reads of the HTML.
func renfeServer(t *testing.T, dwr func(path string) []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".dwr") {
			_, _ = w.Write(dwr(r.URL.Path))
			return
		}
		_, _ = io.WriteString(w, "<html><body>ok</body></html>")
	}))
	t.Cleanup(srv.Close)
	return srv
}

// fixture reads one of the captured DWR replies the client tests use.
func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "internal", "client", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// replaying serves the same captured reply to every DWR call.
func replaying(t *testing.T, name string) *httptest.Server {
	body := fixture(t, name)
	return renfeServer(t, func(string) []byte { return body })
}

// signedInAs answers the session beans with a name; "" means "not signed in".
func signedInAs(t *testing.T, name string) *httptest.Server {
	script := []byte(fmt.Sprintf("throw 'x';\n(function(){\nvar r=window.dwr._[0];\n"+
		"r.handleCallback(\"0\",\"0\",%q);\n})();", name))
	return renfeServer(t, func(string) []byte { return script })
}

// withRenfe points the CLI at srv and gives it a throwaway config dir holding a
// primed station cache, so no command in the test reaches the real renfe.com.
func withRenfe(t *testing.T, srv *httptest.Server) {
	t.Helper()
	t.Setenv("RENFE_CONFIG_DIR", t.TempDir())
	t.Setenv("RENFE_BASE_URL", srv.URL)
	err := store.Save(stationsCache, stationsFile{
		FetchedAt: time.Now(),
		Stations: []client.Station{
			{Code: "60000", Name: "MADRID (TODAS)", Flat: "MADRID (TODAS)"},
			{Code: "71801", Name: "BARCELONA-SANTS", Flat: "BARCELONA-SANTS"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

// capture runs fn with os.Stdout replaced by a pipe and returns what it wrote.
func capture(t *testing.T, fn func() error) (string, error) {
	return captureFile(t, &os.Stdout, fn)
}

// captureStderr is capture for the diagnostics stream.
func captureStderr(t *testing.T, fn func()) string {
	out, _ := captureFile(t, &os.Stderr, func() error { fn(); return nil })
	return out
}

// captureFile swaps one of the process's standard files for a pipe while fn
// runs. The reader drains in a goroutine, so output larger than the pipe buffer
// cannot deadlock the test.
func captureFile(t *testing.T, target **os.File, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := *target
	*target = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	runErr := fn()
	w.Close()
	*target = orig
	return <-done, runErr
}

// dwrPayload wraps a value as the reply script Renfe's DWR endpoint returns.
// JSON is a subset of the object literal DWR emits, so marshalling is enough —
// which lets a test build the odd replies the captures do not contain.
func dwrPayload(t *testing.T, v any) []byte {
	t.Helper()
	body, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return []byte("throw 'x';\n(function(){\nvar r=window.dwr._[0];\n" +
		"r.handleCallback(\"0\",\"0\"," + string(body) + ");\n})();")
}

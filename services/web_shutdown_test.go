package services

import (
	"flag"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/urfave/cli"
)

// exitListener records the connections it accepts, so the test can cut them
// the way the process exit does once Close has returned.
type exitListener struct {
	net.Listener
	mu    sync.Mutex
	conns []net.Conn
}

func (l *exitListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err == nil {
		l.mu.Lock()
		l.conns = append(l.conns, c)
		l.mu.Unlock()
	}
	return c, err
}

func (l *exitListener) exit() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, c := range l.conns {
		_ = c.Close()
	}
}

// A conversion in flight when SIGTERM lands is answered whole: Close returns
// only after it, so the process exit that follows cuts nothing. New
// connections are refused as soon as the drain starts. The Web comes from
// NewWeb over RegisterWebFlags, so the WEB_SHUTDOWN_TIMEOUT flag must be
// registered there: without it the timeout is 0 and the request is cut.
func TestWebCloseDrainsInFlightRequest(t *testing.T) {
	fetched := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	releaseSrc := func() { once.Do(func() { close(release) }) }
	defer releaseSrc()
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(fetched)
		<-release
		_, _ = io.WriteString(w, "1\n00:00:01,000 --> 00:00:03,000\ndrained cue\n")
	}))
	defer src.Close()

	set := flag.NewFlagSet("web", flag.ContinueOnError)
	for _, fl := range RegisterWebFlags(nil) {
		fl.Apply(set)
	}
	web := NewWeb(cli.NewContext(nil, set, nil), NewSRT2VTT(src.Client()))
	inner, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln := &exitListener{Listener: inner}
	go func() { _ = web.serve(ln) }()
	addr := inner.Addr().String()

	type result struct {
		status int
		body   string
		err    error
	}
	got := make(chan result, 1)
	go func() {
		req, _ := http.NewRequest(http.MethodGet, "http://"+addr+"/", nil)
		req.Header.Set("X-Source-Url", src.URL+"/a.srt")
		resp, err := (&http.Client{Transport: &http.Transport{}}).Do(req)
		if err != nil {
			got <- result{err: err}
			return
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		got <- result{resp.StatusCode, string(b), err}
	}()
	select {
	case <-fetched:
	case <-time.After(5 * time.Second):
		t.Fatal("the request did not reach the source")
	}

	closed := make(chan struct{})
	go func() {
		web.Close()
		close(closed)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		c, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err != nil {
			break
		}
		_ = c.Close()
		if time.Now().After(deadline) {
			t.Fatal("new connections still accepted 2 s after Close started")
		}
		time.Sleep(20 * time.Millisecond)
	}

	// A Close that returns while the request is still in flight lets the
	// process exit under it.
	select {
	case <-closed:
		ln.exit()
	case <-time.After(300 * time.Millisecond):
	}
	releaseSrc()
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("Close did not return within 10 s")
	}
	ln.exit()

	r := <-got
	if r.err != nil || r.status != http.StatusOK || !strings.Contains(r.body, "drained cue") {
		t.Fatalf("in-flight request cut: status %d, err %v, body %q", r.status, r.err, r.body)
	}
}

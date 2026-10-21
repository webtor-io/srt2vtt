package services

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/urfave/cli"
)

// astisub v0.32.0 panics (index out of range in ReadFromSRT) on a timing line
// with nothing after "-->", e.g. a file cut off mid-download. Such a file must
// get a 404 with the panic's stack logged, and must not use up the converter:
// lazymap before 55fd9cb let the panic escape Get and kept its concurrency
// slot, so after 10 of them no subtitle converted at all.
func TestPanickingConversionDoesNotWedge(t *testing.T) {
	src := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok.srt" {
			_, _ = io.WriteString(w, "1\n00:00:01,000 --> 00:00:03,000\nfine\n")
			return
		}
		_, _ = io.WriteString(w, "1\n00:00:01,000 -->")
	}))
	defer src.Close()
	hook := test.NewGlobal()
	defer log.StandardLogger().ReplaceHooks(make(log.LevelHooks))

	set := flag.NewFlagSet("web", flag.ContinueOnError)
	for _, fl := range RegisterWebFlags(nil) {
		fl.Apply(set)
	}
	web := NewWeb(cli.NewContext(nil, set, nil), NewSRT2VTT(src.Client()))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = web.serve(ln) }()
	defer web.Close()

	cl := &http.Client{Timeout: 5 * time.Second}
	get := func(path string) int {
		req, _ := http.NewRequest(http.MethodGet, "http://"+ln.Addr().String()+"/", nil)
		req.Header.Set("X-Source-Url", src.URL+path)
		resp, err := cl.Do(req)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			return 0
		}
		_ = resp.Body.Close()
		return resp.StatusCode
	}
	// More distinct keys than lazymap's default Concurrency (10), then the
	// first key again.
	for i := 0; i < 12; i++ {
		if got := get(fmt.Sprintf("/cut%d.srt", i)); got != http.StatusNotFound {
			t.Errorf("cut%d.srt: status %d, want 404", i, got)
		}
	}
	if got := get("/cut0.srt"); got != http.StatusNotFound {
		t.Errorf("cut0.srt again: status %d, want 404", got)
	}
	if got := get("/ok.srt"); got != http.StatusOK {
		t.Errorf("ok.srt after the panics: status %d, want 200", got)
	}

	for _, e := range hook.AllEntries() {
		if s, _ := e.Data["stack"].(string); strings.Contains(s, "astisub.ReadFromSRT") {
			return
		}
	}
	t.Errorf("no log entry carries the panic's stack")
}

package services

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gogs/chardet"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
)

// Polish subtitles, long enough for chardet to settle on ISO-8859-2.
const polishSRT = `1
00:00:01,000 --> 00:00:03,000
Zażółć gęślą jaźń, przyjacielu.

2
00:00:04,000 --> 00:00:06,000
Nie mogę uwierzyć, że to się stało.

3
00:00:07,000 --> 00:00:09,000
Wszyscy poszli do lasu i śpiewali.

4
00:00:10,000 --> 00:00:12,000
Może jutro pójdziemy nad rzekę razem.

5
00:00:13,000 --> 00:00:15,000
Świat jest pełen dziwnych rzeczy i ludzi.

6
00:00:16,000 --> 00:00:18,000
Czy naprawdę myślisz, że można żyć inaczej?

7
00:00:19,000 --> 00:00:21,000
Sześć lat później wrócił do rodzinnego miasta.

8
00:00:22,000 --> 00:00:24,000
Wciąż pamiętam ten dzień, kiedy się poznaliśmy.
`

// French subtitles. Detected as ISO-8859-1, the most common charset in
// production, because every byte used also exists in Latin-1.
const frenchSRT = `1
00:00:01,000 --> 00:00:03,000
Le café était très bon, mon frère.

2
00:00:04,000 --> 00:00:06,000
Où est passé le garçon qui chantait là-bas ?

3
00:00:07,000 --> 00:00:09,000
Nous étions déjà arrivés à la gare.

4
00:00:10,000 --> 00:00:12,000
Il a payé cinquante euros pour une pièce de théâtre.
`

// Same French subtitles plus characters that only exist in windows-1252
// (the 0x80-0x9F range), which is what tips chardet over to windows-1252.
const frenchWin1252SRT = `1
00:00:01,000 --> 00:00:03,000
Le café était très bon, mon frère.

2
00:00:04,000 --> 00:00:06,000
Où est passé le garçon qui chantait là-bas ?

3
00:00:07,000 --> 00:00:09,000
Nous étions déjà arrivés à la gare.

4
00:00:10,000 --> 00:00:12,000
Il a payé 50 € pour une pièce de théâtre.

5
00:00:13,000 --> 00:00:15,000
« Bien sûr », dit-il, ‘tout est déjà prêt’.
`

// TestSRT2VTTConvertsLegacyEncodings serves an SRT in a legacy 8-bit charset
// and asserts the resulting WebVTT carries the correct UTF-8 text.
func TestSRT2VTTConvertsLegacyEncodings(t *testing.T) {
	for _, tc := range []struct {
		name string
		enc  encoding.Encoding
		// srt is the source subtitle in UTF-8, encoded to enc before serving.
		srt string
		// charset is what chardet must report for this fixture, pinning which
		// production charset the case actually exercises.
		charset string
		// probe is a byte that must be present in the encoded fixture, proving
		// the fixture really is in enc and not accidentally plain ASCII.
		probe byte
		// want are UTF-8 fragments expected in the resulting WebVTT.
		want []string
	}{
		{
			name:    "iso-8859-2",
			enc:     charmap.ISO8859_2,
			srt:     polishSRT,
			charset: "ISO-8859-2",
			probe:   0xB1, // 'ą'
			want: []string{
				"Zażółć gęślą jaźń, przyjacielu.",
				"Sześć lat później wrócił do rodzinnego miasta.",
			},
		},
		{
			name:    "iso-8859-1",
			enc:     charmap.ISO8859_1,
			srt:     frenchSRT,
			charset: "ISO-8859-1",
			probe:   0xE9, // 'é'
			want: []string{
				"Le café était très bon, mon frère.",
				"Où est passé le garçon qui chantait là-bas ?",
			},
		},
		{
			name:    "windows-1252",
			enc:     charmap.Windows1252,
			srt:     frenchWin1252SRT,
			charset: "windows-1252",
			probe:   0x80, // '€', only defined in windows-1252
			want: []string{
				"Il a payé 50 € pour une pièce de théâtre.",
				"« Bien sûr », dit-il, ‘tout est déjà prêt’.",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, err := tc.enc.NewEncoder().Bytes([]byte(tc.srt))
			if err != nil {
				t.Fatalf("failed to encode fixture: %v", err)
			}
			if bytes.IndexByte(body, tc.probe) < 0 {
				t.Fatalf("fixture does not contain byte %#x, it is not really %v", tc.probe, tc.name)
			}
			if bytes.Equal(body, []byte(tc.srt)) {
				t.Fatalf("fixture is byte-identical to its UTF-8 source, nothing to convert")
			}
			// Guard against the fixture silently drifting to another charset,
			// which would make the case stop testing what it claims to test.
			detected, err := chardet.NewTextDetector().DetectBest(body)
			if err != nil {
				t.Fatalf("failed to detect fixture encoding: %v", err)
			}
			if detected.Charset != tc.charset {
				t.Fatalf("fixture detected as %v, want %v", detected.Charset, tc.charset)
			}

			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write(body)
			}))
			defer srv.Close()

			vtt, err := NewSRT2VTT(srv.Client()).Get(context.Background(), srv.URL+"/"+tc.name+".srt")
			if err != nil {
				t.Fatalf("failed to convert: %v", err)
			}
			for _, w := range tc.want {
				if !strings.Contains(vtt, w) {
					t.Errorf("vtt is missing %q\ngot:\n%s", w, vtt)
				}
			}
		})
	}
}

// TestGetEncoding checks that every charset observed in production resolves.
func TestGetEncoding(t *testing.T) {
	for _, charset := range []string{
		"ISO-8859-1", "ISO-8859-2", "ISO-8859-9", "windows-1250",
		"windows-1251", "windows-1252", "windows-1256", "UTF-16LE",
		"UTF-32LE", "UTF-32BE",
	} {
		t.Run(charset, func(t *testing.T) {
			e, err := getEncoding(charset)
			if err != nil {
				t.Fatalf("failed to resolve %v: %v", charset, err)
			}
			if e == nil {
				t.Fatalf("resolved %v to a nil encoding", charset)
			}
		})
	}
	if _, err := getEncoding("IBM424_ltr"); err == nil {
		t.Errorf("expected an error for an unsupported charset, got none")
	}
}

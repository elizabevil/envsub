package extended_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/elizabevil/envsub/internal/diag"
	"github.com/elizabevil/envsub/internal/engine"
	"github.com/elizabevil/envsub/internal/engine/extended"
	"github.com/elizabevil/envsub/internal/values"
)

func TestScan(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  []extended.Token
	}{
		{
			name:  "text only",
			input: "hello",
			want:  []extended.Token{{Kind: extended.TokenText, Text: "hello"}},
		},
		{
			name:  "simple variables",
			input: "hello $NAME ${USER}",
			want: []extended.Token{
				{Kind: extended.TokenText, Text: "hello "},
				{Kind: extended.TokenVariable, Path: "NAME", Pos: 6, Raw: "$NAME"},
				{Kind: extended.TokenText, Text: " "},
				{Kind: extended.TokenVariable, Path: "USER", Braced: true, Pos: 12, Raw: "${USER}"},
			},
		},
		{
			name:  "literal dollar",
			input: "cost $$5",
			want: []extended.Token{
				{Kind: extended.TokenText, Text: "cost $5"},
			},
		},
		{
			name:  "digit is literal",
			input: "$1 $2",
			want:  []extended.Token{{Kind: extended.TokenText, Text: "$1 $2"}},
		},
		{
			name:  "escaped braced",
			input: "$${NAME}",
			want:  []extended.Token{{Kind: extended.TokenText, Text: "${NAME}"}},
		},
		{
			name:  "path",
			input: "${app.port}",
			want: []extended.Token{
				{Kind: extended.TokenVariable, Path: "app.port", Braced: true, Pos: 0, Raw: "${app.port}"},
			},
		},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := extended.Scan([]byte(tt.input))
			if err != nil {
				t.Fatalf("Scan() error = %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Scan() len = %d, want %d; got %#v", len(got), len(tt.want), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("Scan()[%d] = %#v, want %#v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestScanErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		input   string
		wantMsg string
	}{
		{name: "unclosed", input: "${NAME", wantMsg: "unclosed variable expression"},
		{name: "empty", input: "${}", wantMsg: "empty variable expression"},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := extended.Scan([]byte(tt.input))
			if err == nil {
				t.Fatal("Scan() error = nil, want error")
			}
			if got := diag.KindOf(err); got != diag.KindTemplate {
				t.Fatalf("error kind = %q, want %q (err=%v)", got, diag.KindTemplate, err)
			}
			if got := diag.ExitCode(err); got != 3 {
				t.Fatalf("ExitCode() = %d, want 3 (err=%v)", got, err)
			}
			if got := err.Error(); !strings.Contains(got, tt.wantMsg) {
				t.Fatalf("error = %q, want it to contain %q", got, tt.wantMsg)
			}
		})
	}
}

// TestScannerErrorCarriesOffset pins that the scanner reports the byte offset as
// a diag.Error field, not only inside the message.
func TestScannerErrorCarriesOffset(t *testing.T) {
	t.Parallel()
	_, err := extended.New().Render(context.Background(), engine.Input{
		Template: []byte("ok ${A"),
		Values:   values.NewTree(),
	})
	if err == nil {
		t.Fatal("Render() error = nil, want a scanner error")
	}
	var de *diag.Error
	if !errors.As(err, &de) {
		t.Fatalf("errors.As(%v, *diag.Error) = false", err)
	}
	if de.Offset < 0 {
		t.Fatalf("scanner error offset = %d, want a recorded offset", de.Offset)
	}
}

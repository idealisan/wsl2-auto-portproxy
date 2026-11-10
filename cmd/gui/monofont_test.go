package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestMonoFontStackFallsBackToGeneric(t *testing.T) {
	for _, families := range [][]ui.FontFamily{
		nil,
		{{Name: "Segoe UI"}},
		{{Name: "Helvetica", Monospaced: false}},
	} {
		if got := monoFontStack(families); got != "monospace" {
			t.Fatalf("got %q, want the generic monospace family", got)
		}
	}
}

func TestMonoFontStackPrefersBuiltInMono(t *testing.T) {
	got := monoFontStack([]ui.FontFamily{
		{Name: "Segoe UI"},
		{Name: "DejaVu Sans Mono", Monospaced: true},
		{Name: "Menlo", Monospaced: true},
		{Name: "Consolas", Monospaced: true},
	})
	want := "Consolas, Menlo, DejaVu Sans Mono, monospace"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMonoFontStackDedupesAndSkipsUnsafeNames(t *testing.T) {
	got := monoFontStack([]ui.FontFamily{
		{Name: "Fira Code", Monospaced: true},
		{Name: "consolas", Monospaced: true},  // case-insensitive duplicate
		{Name: "Consolas", Monospaced: true},  // of this one
		{Name: "Bad, Name", Monospaced: true}, // comma would split the list
		{Name: "Bad\"Name", Monospaced: true}, // quote would break parsing
		{Name: "", Monospaced: true},
	})
	want := "consolas, Fira Code, monospace"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

package main

import (
	"sort"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"
)

// preferredMono lists the well-known monospaced families shipped with each
// OS, best looking first. A name is used only when the system really has it;
// see monoFontStack.
var preferredMono = []string{
	"Cascadia Mono",    // Windows Terminal / Windows 10+
	"Consolas",         // Windows
	"SF Mono",          // macOS
	"Menlo",            // macOS
	"DejaVu Sans Mono", // Linux
	"Liberation Mono",  // Linux
	"Noto Sans Mono",   // Linux / cross-platform
	"Courier New",      // Windows / macOS
}

// monoFontStack builds a Font() family list from the families the system
// actually has. Only monospaced families are used, the preferred built-in
// ones come first, and any other monospaced font follows in a stable order.
// The generic "monospace" is always last: mygo resolves it to a built-in
// system font on every platform (DirectWrite, Core Text, Pango), so even a
// system where FontFamilies reports nothing still gets a monospaced font.
// No font is bundled and no family name is assumed to exist.
func monoFontStack(families []ui.FontFamily) string {
	available := map[string]string{} // lowercased name -> original
	for _, f := range families {
		if !f.Monospaced {
			continue
		}
		name := strings.TrimSpace(f.Name)
		key := strings.ToLower(name)
		// A comma or quote would break the family list; the generic name is
		// appended below, not taken from the system's list.
		if name == "" || key == "monospace" || strings.ContainsAny(name, ",\"") {
			continue
		}
		if _, ok := available[key]; !ok {
			available[key] = name
		}
	}

	seen := map[string]bool{}
	var chosen []string
	add := func(key string) {
		if name, ok := available[key]; ok && !seen[key] {
			seen[key] = true
			chosen = append(chosen, name)
		}
	}
	for _, name := range preferredMono {
		add(strings.ToLower(name))
	}
	var rest []string
	for key := range available {
		if !seen[key] {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	for _, key := range rest {
		add(key)
	}
	return strings.Join(append(chosen, "monospace"), ", ")
}

var (
	monoOnce   sync.Once
	monoFamily string
)

// monoFont resolves the family list once. The first FontFamilies() call asks
// the OS and can take a few tens of milliseconds, so it stays off the hot
// path of every frame.
func monoFont() string {
	monoOnce.Do(func() {
		monoFamily = monoFontStack(ui.FontFamilies())
	})
	return monoFamily
}

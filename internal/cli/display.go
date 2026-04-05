package cli

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ── ANSI color constants ──────────────────────────────────────────────────────

const (
	Reset   = "\033[0m"
	Bold    = "\033[1m"
	Dim     = "\033[2m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
)

// Color wraps s in the given ANSI color code.
func Color(color, s string) string {
	return color + s + Reset
}

// ── headers / sections ────────────────────────────────────────────────────────

// Header prints a cyan-bordered box with the given title.
func Header(title string) {
	const width = 52
	fmt.Println(Cyan + "╔" + strings.Repeat("═", width-2) + "╗" + Reset)
	inner := "  " + title
	pad := width - 2 - utf8.RuneCountInString(inner)
	if pad < 0 {
		pad = 0
	}
	fmt.Println(Cyan + "║" + Reset + inner + strings.Repeat(" ", pad) + Cyan + "║" + Reset)
	fmt.Println(Cyan + "╚" + strings.Repeat("═", width-2) + "╝" + Reset)
	fmt.Println()
}

// Section prints a yellow, underlined section heading.
func Section(title string) {
	fmt.Println(Yellow + Bold + title + Reset)
	fmt.Println(Yellow + strings.Repeat("─", utf8.RuneCountInString(title)) + Reset)
	fmt.Println()
}

// ── status lines ─────────────────────────────────────────────────────────────

func Success(msg string) { fmt.Println(Green + "  ✓ " + msg + Reset) }
func Error(msg string)   { fmt.Println(Red + "  ✗ " + msg + Reset) }
func Warning(msg string) { fmt.Println(Yellow + "  ⚠ " + msg + Reset) }
func Info(msg string)    { fmt.Println(Cyan + "  ℹ " + msg + Reset) }

// Step prints a step line; if done is true it uses a check mark.
func Step(msg string, done bool) {
	if done {
		fmt.Println(Green + "  ✓ " + msg + Reset)
	} else {
		fmt.Println(Dim + "  → " + msg + Reset)
	}
}

// Checkbox returns a colored checkbox symbol.
func Checkbox(checked bool) string {
	if checked {
		return Color(Green, "☑")
	}
	return Color(Dim, "☐")
}

// ── table ─────────────────────────────────────────────────────────────────────

// Table prints an ASCII bordered table.
// widths[i] is the inner column width (excluding padding and borders).
func Table(headers []string, rows [][]string, widths []int) {
	sep := "+" + joinSep(widths) + "+"

	fmt.Println(sep)
	fmt.Print("|")
	for i, h := range headers {
		w := colWidth(widths, i)
		fmt.Print(" " + Bold + padRight(stripANSI(h), w, h) + Reset + " |")
	}
	fmt.Println()
	fmt.Println(sep)

	for _, row := range rows {
		fmt.Print("|")
		for i, cell := range row {
			if i >= len(widths) {
				break
			}
			w := colWidth(widths, i)
			fmt.Print(" " + padRight(stripANSI(cell), w, cell) + " |")
		}
		fmt.Println()
	}
	fmt.Println(sep)
}

func joinSep(widths []int) string {
	parts := make([]string, len(widths))
	for i, w := range widths {
		parts[i] = strings.Repeat("-", w+2)
	}
	return strings.Join(parts, "+")
}

func colWidth(widths []int, i int) int {
	if i < len(widths) {
		return widths[i]
	}
	return 20
}

// padRight pads visible (ANSI-stripped) text to width w, using original to
// preserve color codes.
func padRight(visible string, w int, original string) string {
	vlen := utf8.RuneCountInString(visible)
	if vlen >= w {
		if vlen > w {
			// truncate visible
			runes := []rune(visible)
			return string(runes[:w])
		}
		return original
	}
	return original + strings.Repeat(" ", w-vlen)
}

// stripANSI removes ANSI escape codes from s to get the visible length.
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// skip until 'm'
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			i = j + 1
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

// ── key-value list ────────────────────────────────────────────────────────────

// KeyValue prints aligned key: value pairs.
func KeyValue(items [][2]string) {
	maxKey := 0
	for _, kv := range items {
		if n := utf8.RuneCountInString(kv[0]); n > maxKey {
			maxKey = n
		}
	}
	for _, kv := range items {
		pad := maxKey - utf8.RuneCountInString(kv[0])
		fmt.Printf("  %s%s : %s\n", kv[0], strings.Repeat(" ", pad), kv[1])
	}
}

// ── API key box ───────────────────────────────────────────────────────────────

// ApiKeyBox displays the new API key in a prominent green-bordered box.
func ApiKeyBox(key string) {
	width := len(key) + 6
	if width < 50 {
		width = 50
	}
	fmt.Println()
	fmt.Println(Green + "┌" + strings.Repeat("─", width-2) + "┐" + Reset)
	fmt.Println(Green + "│" + Reset + Bold + "  ¡GUARDA ESTA KEY — SE MUESTRA UNA SOLA VEZ!" + Reset + strings.Repeat(" ", width-2-47) + Green + "│" + Reset)
	fmt.Println(Green + "│" + Reset)
	inner := "  " + key
	pad := width - 2 - len(inner)
	if pad < 0 {
		pad = 0
	}
	fmt.Println(Green + "│" + Reset + Green + Bold + inner + Reset + strings.Repeat(" ", pad) + Green + "│" + Reset)
	fmt.Println(Green + "│" + Reset)
	fmt.Println(Green + "└" + strings.Repeat("─", width-2) + "┘" + Reset)
	fmt.Println()
}

// MaskKey shows the first 7 and last 4 chars of key, with "..." in between.
func MaskKey(key string) string {
	if len(key) < 12 {
		return key
	}
	return key[:7] + "..." + key[len(key)-4:]
}

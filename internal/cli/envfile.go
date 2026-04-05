package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Path returns the path to the .env file (cwd/.env).
func Path() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ".env"
	}
	return filepath.Join(cwd, ".env")
}

// templatePath returns the path to the .env.template file.
func templatePath() string {
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, ".env.template")
}

// Exists reports whether the .env file exists.
func Exists() bool {
	_, err := os.Stat(Path())
	return err == nil
}

// TemplateExists reports whether the .env.template file exists.
func TemplateExists() bool {
	_, err := os.Stat(templatePath())
	return err == nil
}

// CreateFromTemplate copies .env.template to .env. Reports (true, nil) on
// success, (false, nil) if no template exists, or (false, err) on I/O error.
func CreateFromTemplate() (bool, error) {
	if !TemplateExists() {
		return false, nil
	}
	data, err := os.ReadFile(templatePath())
	if err != nil {
		return false, fmt.Errorf("envfile: read template: %w", err)
	}
	if err := os.WriteFile(Path(), data, 0o600); err != nil {
		return false, fmt.Errorf("envfile: write .env: %w", err)
	}
	return true, nil
}

// Read parses the .env file and returns a map of key → value.
// Comments (#) and blank lines are skipped. Surrounding quotes are stripped.
func Read() (map[string]string, error) {
	f, err := os.Open(Path())
	if os.IsNotExist(err) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("envfile: open: %w", err)
	}
	defer f.Close() //nolint:errcheck

	m := make(map[string]string)
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		// Strip surrounding quotes.
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') ||
			(val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}
		m[key] = val
	}
	return m, sc.Err()
}

// GetValue returns the value of key from the .env file, or "" if not found.
func GetValue(key string) string {
	m, _ := Read()
	return m[key]
}

// HasValue reports whether key is set to a non-empty value in .env.
func HasValue(key string) bool {
	return GetValue(key) != ""
}

// WriteValue writes key=value to the .env file, updating an existing entry or
// appending a new one. It also calls os.Setenv so the current process sees the
// change immediately.
func WriteValue(key, value string) error {
	lines, err := readLines()
	if err != nil {
		return err
	}

	updated := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.IndexByte(trimmed, '=')
		if idx < 0 {
			continue
		}
		if strings.TrimSpace(trimmed[:idx]) == key {
			lines[i] = key + "=" + value
			updated = true
			break
		}
	}
	if !updated {
		lines = append(lines, key+"="+value)
	}

	if err := writeLines(lines); err != nil {
		return err
	}
	os.Setenv(key, value) //nolint:errcheck
	return nil
}

// RemoveValue deletes the line for key from the .env file.
func RemoveValue(key string) error {
	lines, err := readLines()
	if err != nil {
		return err
	}

	out := lines[:0]
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			out = append(out, line)
			continue
		}
		idx := strings.IndexByte(trimmed, '=')
		if idx >= 0 && strings.TrimSpace(trimmed[:idx]) == key {
			continue // skip this line
		}
		out = append(out, line)
	}

	if err := writeLines(out); err != nil {
		return err
	}
	os.Unsetenv(key) //nolint:errcheck
	return nil
}

// ── internal helpers ──────────────────────────────────────────────────────────

func readLines() ([]string, error) {
	data, err := os.ReadFile(Path())
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("envfile: read: %w", err)
	}
	lines := strings.Split(string(data), "\n")
	// Remove a trailing empty element from a final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

func writeLines(lines []string) error {
	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(Path(), []byte(content), 0o600)
}

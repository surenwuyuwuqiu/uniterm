//go:build windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

func getSystemFonts() ([]string, error) {
	key, err := registry.OpenKey(
		registry.LOCAL_MACHINE,
		`SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`,
		registry.QUERY_VALUE,
	)
	if err != nil {
		return nil, fmt.Errorf("open registry: %w", err)
	}
	defer key.Close()

	names, err := key.ReadValueNames(-1)
	if err != nil {
		return nil, fmt.Errorf("read value names: %w", err)
	}

	fontDir := `C:\Windows\Fonts`
	var families []string
	seen := make(map[string]bool)

	for _, name := range names {
		val, _, err := key.GetStringValue(name)
		if err != nil {
			continue
		}

		path := resolveFontPath(val, fontDir)
		if path == "" {
			continue
		}

		family, isMono, err := parseFont(path)
		if err != nil || !isMono || family == "" {
			continue
		}
		if seen[family] {
			continue
		}
		seen[family] = true
		families = append(families, family)
	}

	return families, nil
}

func resolveFontPath(val, fontDir string) string {
	// Absolute path
	if strings.Contains(val, `:\`) {
		if _, err := os.Stat(val); err == nil {
			return val
		}
		return ""
	}

	// Relative to fontDir
	path := val
	if !filepath.IsAbs(val) {
		path = filepath.Join(fontDir, val)
	}
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

// readFontFile reads a font file from disk for parsing.
func readFontFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

//go:build windows

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows/registry"
)

// getSystemFonts enumerates monospaced font families from both the
// machine-wide registry/font store and the per-user one. Windows 10 1809+
// allows installing a font without admin rights ("Install for me only" /
// double-click a .ttf preview and hit Install as a standard user), which
// writes to HKCU + %LOCALAPPDATA%\Microsoft\Windows\Fonts instead of HKLM +
// C:\Windows\Fonts. Other apps see such fonts via the GDI/DirectWrite system
// font enumeration APIs, which cover both scopes; our hand-rolled registry
// walk needs to check both explicitly or it silently misses user-installed
// fonts (https://github.com/ys-ll/uniterm/issues/145).
func getSystemFonts() ([]string, error) {
	var families []string
	var firstErr error

	if fam, err := readFontFamilies(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`, `C:\Windows\Fonts`); err != nil {
		firstErr = err
	} else {
		families = append(families, fam...)
	}

	// Per-user scope is optional: on systems/setups where nothing was ever
	// installed at user scope this key simply doesn't exist, which is normal
	// and not reported as an error as long as the machine-wide scan above
	// found something.
	if userFontDir := userFontsDir(); userFontDir != "" {
		if fam, err := readFontFamilies(registry.CURRENT_USER, `Software\Microsoft\Windows NT\CurrentVersion\Fonts`, userFontDir); err == nil {
			families = append(families, fam...)
		}
	}

	if len(families) == 0 && firstErr != nil {
		return nil, firstErr
	}
	return families, nil
}

// userFontsDir returns the per-user font install directory, or "" if
// %LOCALAPPDATA% isn't set (unexpected, but avoids a bogus relative path).
func userFontsDir() string {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return ""
	}
	return filepath.Join(local, "Microsoft", "Windows", "Fonts")
}

// readFontFamilies reads one Fonts registry key (HKLM or HKCU) and resolves
// each value against fontDir, returning the unique monospaced family names
// found in that scope.
func readFontFamilies(root registry.Key, keyPath, fontDir string) ([]string, error) {
	key, err := registry.OpenKey(root, keyPath, registry.QUERY_VALUE)
	if err != nil {
		return nil, fmt.Errorf("open registry: %w", err)
	}
	defer key.Close()

	names, err := key.ReadValueNames(-1)
	if err != nil {
		return nil, fmt.Errorf("read value names: %w", err)
	}

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

// parseFont reads a TTF/OTF/TTC file and returns (familyName, isMonospace, error).
func parseFont(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, err
	}

	// Handle TrueType Collection (.ttc)
	tag := tag4(data, 0)
	if tag == "ttcf" {
		if len(data) < 12 {
			return "", false, fmt.Errorf("ttc too small")
		}
		offset := u32(data, 8)
		if int(offset)+12 > len(data) {
			return "", false, fmt.Errorf("ttc offset out of range")
		}
		data = data[offset:]
		tag = tag4(data, 0)
	}

	// Validate font file
	if tag != "true" && tag != "\x00\x01\x00\x00" && tag != "OTTO" {
		return "", false, nil // not a supported font, skip silently
	}

	numTables := int(u16(data, 4))
	if numTables <= 0 || numTables > 200 {
		return "", false, fmt.Errorf("invalid numTables")
	}

	var nameOffset, nameLen uint32
	var postOffset uint32

	for i := 0; i < numTables; i++ {
		off := 12 + i*16
		if off+16 > len(data) {
			break
		}
		tableTag := tag4(data, off)
		tableOffset := u32(data, off+8)
		tableLength := u32(data, off+12)

		switch tableTag {
		case "name":
			nameOffset = tableOffset
			nameLen = tableLength
		case "post":
			postOffset = tableOffset
		}
	}

	if postOffset == 0 || nameOffset == 0 {
		return "", false, fmt.Errorf("required tables not found")
	}

	// Check isFixedPitch in post table (offset 12, uint32)
	if int(postOffset)+16 > len(data) {
		return "", false, fmt.Errorf("post table out of range")
	}
	isMono := u32(data, int(postOffset)+12) != 0
	if !isMono {
		return "", false, nil
	}

	// Read font family name from name table (Name ID 1)
	family := parseNameTable(data, int(nameOffset), int(nameLen))
	return family, true, nil
}

// parseNameTable extracts Name ID 1 (Font Family) from the name table.
func parseNameTable(data []byte, offset, length int) string {
	if offset+6 > len(data) {
		return ""
	}
	_ = length

	count := int(u16(data, offset+2))
	storageOffset := offset + int(u16(data, offset+4))
	if count > 500 || storageOffset > len(data) {
		return ""
	}

	// Priority: Platform 3 (Windows) > Platform 1 (Mac)
	var win16, macRoman string

	for i := 0; i < count; i++ {
		recOff := offset + 6 + i*12
		if recOff+12 > len(data) {
			break
		}
		platformID := u16(data, recOff)
		encodingID := u16(data, recOff+2)
		nameID := u16(data, recOff+6)
		recLen := int(u16(data, recOff+8))
		recOffset := int(u16(data, recOff+10))

		if nameID != 1 {
			continue
		}

		strOff := storageOffset + recOffset
		if strOff+recLen > len(data) {
			continue
		}
		raw := data[strOff : strOff+recLen]

		switch {
		case platformID == 3 && encodingID == 1: // Windows, UTF-16 BE
			win16 = decodeUTF16BE(raw)
		case platformID == 1 && encodingID == 0: // Mac Roman
			macRoman = decodeMacRoman(raw)
		}
	}

	if win16 != "" {
		return win16
	}
	return macRoman
}

func decodeUTF16BE(data []byte) string {
	if len(data)%2 != 0 {
		return ""
	}
	runes := make([]rune, len(data)/2)
	for i := 0; i < len(data); i += 2 {
		runes[i/2] = rune(u16(data, i))
	}
	return string(runes)
}

func decodeMacRoman(data []byte) string {
	runes := make([]rune, len(data))
	for i, b := range data {
		runes[i] = macRomanToRune(b)
	}
	return string(runes)
}

func macRomanToRune(b byte) rune {
	return macRomanTable[b]
}

// Mac OS Roman character set
var macRomanTable = [256]rune{
	0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07,
	0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F,
	0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17,
	0x18, 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E, 0x1F,
	0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27,
	0x28, 0x29, 0x2A, 0x2B, 0x2C, 0x2D, 0x2E, 0x2F,
	0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37,
	0x38, 0x39, 0x3A, 0x3B, 0x3C, 0x3D, 0x3E, 0x3F,
	0x40, 0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47,
	0x48, 0x49, 0x4A, 0x4B, 0x4C, 0x4D, 0x4E, 0x4F,
	0x50, 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57,
	0x58, 0x59, 0x5A, 0x5B, 0x5C, 0x5D, 0x5E, 0x5F,
	0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67,
	0x68, 0x69, 0x6A, 0x6B, 0x6C, 0x6D, 0x6E, 0x6F,
	0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77,
	0x78, 0x79, 0x7A, 0x7B, 0x7C, 0x7D, 0x7E, 0x7F,
	0xC4, 0xC5, 0xC7, 0xC9, 0xD1, 0xD6, 0xDC, 0xE1,
	0xE0, 0xE2, 0xE4, 0xE3, 0xE5, 0xE7, 0xE9, 0xE8,
	0xEA, 0xEB, 0xED, 0xEC, 0xEE, 0xEF, 0xF1, 0xF3,
	0xF2, 0xF4, 0xF6, 0xF5, 0xFA, 0xF9, 0xFB, 0xFC,
	0x2020, 0xB0, 0xA2, 0xA3, 0xA7, 0x2022, 0xB6, 0xDF,
	0xAE, 0xA9, 0x2122, 0xB4, 0xA8, 0x2260, 0xC6, 0xD8,
	0x221E, 0xB1, 0x2264, 0x2265, 0xA5, 0xB5, 0x2202, 0x2211,
	0x220F, 0x03C0, 0x222B, 0xAA, 0xBA, 0x03A9, 0xE6, 0xF8,
	0xBF, 0xA1, 0xAC, 0x221A, 0x0192, 0x2248, 0x2206, 0xAB,
	0xBB, 0x2026, 0xA0, 0xC0, 0xC3, 0xD5, 0x0152, 0x0153,
	0x2013, 0x2014, 0x201C, 0x201D, 0x2018, 0x2019, 0xF7,
	0x25CA, 0xFF, 0x0178, 0x2044, 0x20AC, 0x2039, 0x203A,
	0xFB01, 0xFB02, 0x2021, 0xB7, 0x201A, 0x201E, 0x2030,
	0xC2, 0xCA, 0xC1, 0xCB, 0xC8, 0xCD, 0xCE, 0xCF,
	0xCC, 0xD3, 0xD4, 0xF8FF, 0xD2, 0xDA, 0xDB, 0xD9,
}

// Binary parsing helpers
func tag4(data []byte, off int) string {
	if off+4 > len(data) {
		return ""
	}
	return string(data[off : off+4])
}

func u16(data []byte, off int) uint16 {
	return uint16(data[off])<<8 | uint16(data[off+1])
}

func u32(data []byte, off int) uint32 {
	return uint32(data[off])<<24 | uint32(data[off+1])<<16 | uint32(data[off+2])<<8 | uint32(data[off+3])
}

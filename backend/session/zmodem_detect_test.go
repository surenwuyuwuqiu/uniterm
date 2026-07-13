package session

import "testing"

// Regression test for issue #242: vim rendering a file whose content contains
// `**` followed by a long hex string (hashes, IDs in YAML/JSON) must NOT be
// mistaken for a ZMODEM header. A real header requires the ZDLE (0x18) byte.
func TestLooksLikeZmodemHeader(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"real ZRQINIT", []byte("**\x18B00000000000000"), true},
		{"real with ZPAD run", []byte("rz waiting...\r\n**\x18A0123456789abcdef"), true},
		{"vim content stars+hex (no ZDLE)", []byte("value: **B0a1b2c3d4e5f60718"), false},
		{"markdown bold", []byte("see **B1234567890 for details"), false},
		{"plain stars", []byte("=====**=====\r\n"), false},
		{"empty", []byte(""), false},
		{"stars at end truncated", []byte("abc**\x18"), false},
	}
	for _, tt := range tests {
		if got := looksLikeZmodemHeader(tt.data); got != tt.want {
			t.Errorf("%s: looksLikeZmodemHeader(%q) = %v, want %v", tt.name, tt.data, got, tt.want)
		}
	}
}

//go:build !windows && !darwin

package platform

import (
	"os/exec"
	"strings"
)

func getSystemFonts() ([]string, error) {
	cmd := exec.Command("fc-list", ":spacing=mono", "family")
	out, err := cmd.Output()
	if err != nil {
		// Fallback: try without the spacing filter
		cmd := exec.Command("fc-list", "", "family")
		out, err = cmd.Output()
		if err != nil {
			return nil, err
		}
	}

	seen := make(map[string]bool)
	var families []string

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// fc-list outputs comma-separated family names per line
		for _, name := range strings.Split(line, ",") {
			name = strings.TrimSpace(name)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			families = append(families, name)
		}
	}

	return families, nil
}

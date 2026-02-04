package apply

import (
	"fmt"
	"os"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// expandGlob expands glob patterns to file paths, supporting ** for recursive matching.
// It follows git-style pattern matching where later patterns override earlier ones
// and ! prefix negates (excludes) a pattern.
func expandGlob(patterns []string) ([]string, error) {
	seen := make(map[string]bool)
	var files []string

	type patternRule struct {
		pattern string
		include bool
	}

	var rules []patternRule
	for _, p := range patterns {
		if strings.HasPrefix(p, "!") {
			rules = append(rules, patternRule{strings.TrimPrefix(p, "!"), false})
		} else {
			rules = append(rules, patternRule{p, true})
		}
	}

	candidateFiles := make(map[string]bool)
	for _, rule := range rules {
		if rule.include {
			if isRemoteURL(rule.pattern) {
				candidateFiles[rule.pattern] = true
				continue
			}
			matches, err := doublestar.FilepathGlob(rule.pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid glob pattern '%s': %w", rule.pattern, err)
			}
			for _, match := range matches {
				info, err := os.Stat(match)
				if err != nil || info.IsDir() {
					continue
				}
				candidateFiles[match] = true
			}
		}
	}

	for filePath := range candidateFiles {
		included := false
		for _, rule := range rules {
			var matched bool
			if isRemoteURL(rule.pattern) {
				matched = filePath == rule.pattern
			} else {
				var err error
				matched, err = doublestar.PathMatch(rule.pattern, filePath)
				if err != nil {
					return nil, fmt.Errorf("invalid pattern '%s': %w", rule.pattern, err)
				}
			}
			if matched {
				included = rule.include
			}
		}
		if included && !seen[filePath] {
			seen[filePath] = true
			files = append(files, filePath)
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files matched patterns")
	}

	return files, nil
}

func isRemoteURL(value string) bool {
	return strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://")
}

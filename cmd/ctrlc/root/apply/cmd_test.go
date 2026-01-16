package apply

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// setupTestDir creates a temporary directory with test files and returns the path
func setupTestDir(t *testing.T) string {
	t.Helper()

	tmpDir := t.TempDir()

	// Create directory structure:
	// tmpDir/
	//   ├── config.yaml
	//   ├── system.yaml
	//   ├── test.yaml
	//   ├── test_config.yaml
	//   ├── apps/
	//   │   ├── app1.yaml
	//   │   ├── app2.yaml
	//   │   └── test_app.yaml
	//   ├── infra/
	//   │   ├── network.yaml
	//   │   └── storage.yaml
	//   └── staging/
	//       └── config.yaml

	dirs := []string{
		"apps",
		"infra",
		"staging",
	}

	files := []string{
		"config.yaml",
		"system.yaml",
		"test.yaml",
		"test_config.yaml",
		"apps/app1.yaml",
		"apps/app2.yaml",
		"apps/test_app.yaml",
		"infra/network.yaml",
		"infra/storage.yaml",
		"staging/config.yaml",
	}

	for _, dir := range dirs {
		err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
		if err != nil {
			t.Fatalf("failed to create directory %s: %v", dir, err)
		}
	}

	for _, file := range files {
		err := os.WriteFile(filepath.Join(tmpDir, file), []byte("test content"), 0644)
		if err != nil {
			t.Fatalf("failed to create file %s: %v", file, err)
		}
	}

	return tmpDir
}

func TestExpandGlob_SingleFile(t *testing.T) {
	tmpDir := setupTestDir(t)

	files, err := expandGlob([]string{filepath.Join(tmpDir, "config.yaml")})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	if files[0] != filepath.Join(tmpDir, "config.yaml") {
		t.Errorf("expected %s, got %s", filepath.Join(tmpDir, "config.yaml"), files[0])
	}
}

func TestExpandGlob_WildcardPattern(t *testing.T) {
	tmpDir := setupTestDir(t)

	files, err := expandGlob([]string{filepath.Join(tmpDir, "*.yaml")})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should match: config.yaml, system.yaml, test.yaml, test_config.yaml
	if len(files) != 4 {
		t.Fatalf("expected 4 files, got %d: %v", len(files), files)
	}
}

func TestExpandGlob_RecursivePattern(t *testing.T) {
	tmpDir := setupTestDir(t)

	files, err := expandGlob([]string{filepath.Join(tmpDir, "**/*.yaml")})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should match all 10 yaml files
	if len(files) != 10 {
		t.Fatalf("expected 10 files, got %d: %v", len(files), files)
	}
}

func TestExpandGlob_ExcludeWithBangPrefix(t *testing.T) {
	tmpDir := setupTestDir(t)

	files, err := expandGlob([]string{
		filepath.Join(tmpDir, "*.yaml"),
		"!" + filepath.Join(tmpDir, "test*.yaml"),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should match: config.yaml, system.yaml (excluding test.yaml, test_config.yaml)
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(files), files)
	}

	sort.Strings(files)
	expected := []string{
		filepath.Join(tmpDir, "config.yaml"),
		filepath.Join(tmpDir, "system.yaml"),
	}
	sort.Strings(expected)

	for i, f := range files {
		if f != expected[i] {
			t.Errorf("expected %s, got %s", expected[i], f)
		}
	}
}

func TestExpandGlob_RecursiveExclude(t *testing.T) {
	tmpDir := setupTestDir(t)

	files, err := expandGlob([]string{
		filepath.Join(tmpDir, "**/*.yaml"),
		"!" + filepath.Join(tmpDir, "**/test*.yaml"),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should exclude: test.yaml, test_config.yaml, apps/test_app.yaml
	// Remaining: config.yaml, system.yaml, apps/app1.yaml, apps/app2.yaml,
	//            infra/network.yaml, infra/storage.yaml, staging/config.yaml
	if len(files) != 7 {
		t.Fatalf("expected 7 files, got %d: %v", len(files), files)
	}

	// Verify test files are excluded
	for _, f := range files {
		base := filepath.Base(f)
		if len(base) >= 4 && base[:4] == "test" {
			t.Errorf("test file should be excluded: %s", f)
		}
	}
}

func TestExpandGlob_ExcludeDirectory(t *testing.T) {
	tmpDir := setupTestDir(t)

	files, err := expandGlob([]string{
		filepath.Join(tmpDir, "**/*.yaml"),
		"!" + filepath.Join(tmpDir, "staging/**"),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should exclude: staging/config.yaml
	// Remaining: 9 files
	if len(files) != 9 {
		t.Fatalf("expected 9 files, got %d: %v", len(files), files)
	}

	// Verify staging files are excluded
	for _, f := range files {
		if filepath.Dir(f) == filepath.Join(tmpDir, "staging") {
			t.Errorf("staging file should be excluded: %s", f)
		}
	}
}

func TestExpandGlob_GitStyleLastMatchWins(t *testing.T) {
	tmpDir := setupTestDir(t)

	// First exclude all test files, then re-include test.yaml
	files, err := expandGlob([]string{
		filepath.Join(tmpDir, "*.yaml"),
		"!" + filepath.Join(tmpDir, "test*.yaml"),
		filepath.Join(tmpDir, "test.yaml"), // Re-include test.yaml
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should match: config.yaml, system.yaml, test.yaml
	// (test_config.yaml stays excluded)
	if len(files) != 3 {
		t.Fatalf("expected 3 files, got %d: %v", len(files), files)
	}

	// Verify test.yaml is included
	hasTestYaml := false
	hasTestConfigYaml := false
	for _, f := range files {
		if filepath.Base(f) == "test.yaml" {
			hasTestYaml = true
		}
		if filepath.Base(f) == "test_config.yaml" {
			hasTestConfigYaml = true
		}
	}

	if !hasTestYaml {
		t.Error("test.yaml should be included (re-included by last pattern)")
	}
	if hasTestConfigYaml {
		t.Error("test_config.yaml should be excluded")
	}
}

func TestExpandGlob_MultiplePatterns(t *testing.T) {
	tmpDir := setupTestDir(t)

	files, err := expandGlob([]string{
		filepath.Join(tmpDir, "apps/*.yaml"),
		filepath.Join(tmpDir, "infra/*.yaml"),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// apps: 3 files, infra: 2 files = 5 total
	if len(files) != 5 {
		t.Fatalf("expected 5 files, got %d: %v", len(files), files)
	}
}

func TestExpandGlob_MultipleExcludePatterns(t *testing.T) {
	tmpDir := setupTestDir(t)

	files, err := expandGlob([]string{
		filepath.Join(tmpDir, "**/*.yaml"),
		"!" + filepath.Join(tmpDir, "**/test*.yaml"),
		"!" + filepath.Join(tmpDir, "staging/**"),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Exclude test files (3) and staging (1) = 10 - 4 = 6
	if len(files) != 6 {
		t.Fatalf("expected 6 files, got %d: %v", len(files), files)
	}
}

func TestExpandGlob_NoFilesMatched(t *testing.T) {
	tmpDir := setupTestDir(t)

	_, err := expandGlob([]string{filepath.Join(tmpDir, "*.json")}) // No JSON files exist

	if err == nil {
		t.Fatal("expected error for no files matched")
	}

	if err.Error() != "no files matched patterns" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExpandGlob_AllFilesExcluded(t *testing.T) {
	tmpDir := setupTestDir(t)

	_, err := expandGlob([]string{
		filepath.Join(tmpDir, "config.yaml"),
		"!" + filepath.Join(tmpDir, "config.yaml"), // Exclude the only file
	})

	if err == nil {
		t.Fatal("expected error when all files are excluded")
	}

	if err.Error() != "no files matched patterns" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestExpandGlob_DeduplicatesFiles(t *testing.T) {
	tmpDir := setupTestDir(t)

	files, err := expandGlob([]string{
		filepath.Join(tmpDir, "config.yaml"),
		filepath.Join(tmpDir, "*.yaml"),      // Also matches config.yaml
		filepath.Join(tmpDir, "config.yaml"), // Explicit duplicate
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should deduplicate and return 4 unique files
	if len(files) != 4 {
		t.Fatalf("expected 4 files (deduplicated), got %d: %v", len(files), files)
	}

	// Count config.yaml occurrences
	count := 0
	for _, f := range files {
		if filepath.Base(f) == "config.yaml" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("config.yaml should appear once, got %d", count)
	}
}

func TestExpandGlob_IgnoresDirectories(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Create a directory that matches the pattern
	err := os.MkdirAll(filepath.Join(tmpDir, "apps.yaml"), 0755)
	if err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}

	files, err := expandGlob([]string{filepath.Join(tmpDir, "*.yaml")})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should only match files, not the apps.yaml directory
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			t.Errorf("failed to stat %s: %v", f, err)
			continue
		}
		if info.IsDir() {
			t.Errorf("directory should not be included: %s", f)
		}
	}
}

func TestExpandGlob_MultipleBangExcludes(t *testing.T) {
	tmpDir := setupTestDir(t)

	files, err := expandGlob([]string{
		filepath.Join(tmpDir, "**/*.yaml"),
		"!" + filepath.Join(tmpDir, "apps/**"),  // Exclude apps
		"!" + filepath.Join(tmpDir, "infra/**"), // Exclude infra
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should exclude apps (3 files) and infra (2 files)
	// Remaining: config.yaml, system.yaml, test.yaml, test_config.yaml, staging/config.yaml = 5
	if len(files) != 5 {
		t.Fatalf("expected 5 files, got %d: %v", len(files), files)
	}

	for _, f := range files {
		dir := filepath.Dir(f)
		if dir == filepath.Join(tmpDir, "apps") {
			t.Errorf("apps file should be excluded: %s", f)
		}
		if dir == filepath.Join(tmpDir, "infra") {
			t.Errorf("infra file should be excluded: %s", f)
		}
	}
}

func TestExpandGlob_ExcludeThenReinclude(t *testing.T) {
	tmpDir := setupTestDir(t)

	// Exclude all apps, then re-include app1.yaml
	files, err := expandGlob([]string{
		filepath.Join(tmpDir, "**/*.yaml"),
		"!" + filepath.Join(tmpDir, "apps/**"),
		filepath.Join(tmpDir, "apps/app1.yaml"), // Re-include just app1
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 7 root files + staging + app1 = 8
	// config.yaml, system.yaml, test.yaml, test_config.yaml,
	// infra/network.yaml, infra/storage.yaml, staging/config.yaml, apps/app1.yaml
	if len(files) != 8 {
		t.Fatalf("expected 8 files, got %d: %v", len(files), files)
	}

	// Verify app1.yaml is included but app2.yaml and test_app.yaml are not
	hasApp1 := false
	hasApp2 := false
	hasTestApp := false
	for _, f := range files {
		switch filepath.Base(f) {
		case "app1.yaml":
			hasApp1 = true
		case "app2.yaml":
			hasApp2 = true
		case "test_app.yaml":
			hasTestApp = true
		}
	}

	if !hasApp1 {
		t.Error("app1.yaml should be included (re-included)")
	}
	if hasApp2 {
		t.Error("app2.yaml should be excluded")
	}
	if hasTestApp {
		t.Error("test_app.yaml should be excluded")
	}
}

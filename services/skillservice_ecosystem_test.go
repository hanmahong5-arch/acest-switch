package services

import (
	"os"
	"path/filepath"
	"testing"
)

// Test version comparison logic
func TestCompareVersion(t *testing.T) {
	tests := []struct {
		v1       string
		v2       string
		expected int
	}{
		// Equal versions
		{"1.0.0", "1.0.0", 0},
		{"v1.0.0", "1.0.0", 0},
		{"2.5.3", "2.5.3", 0},

		// v1 < v2
		{"1.0.0", "1.0.1", -1},
		{"1.0.0", "2.0.0", -1},
		{"1.5.0", "1.10.0", -1},
		{"0.9.0", "1.0.0", -1},
		{"", "1.0.0", -1},

		// v1 > v2
		{"1.0.1", "1.0.0", 1},
		{"2.0.0", "1.0.0", 1},
		{"1.10.0", "1.5.0", 1},
		{"1.0.0", "", 1},

		// Pre-release versions
		{"1.0.0-alpha", "1.0.0-beta", 0}, // Same major.minor.patch
		{"1.0.0", "1.0.1-alpha", -1},
		{"2.0.0-beta", "1.9.9", 1},
	}

	for _, test := range tests {
		result := compareVersion(test.v1, test.v2)
		if result != test.expected {
			t.Errorf("compareVersion(%q, %q) = %d, expected %d", test.v1, test.v2, result, test.expected)
		}
	}
}

// Test update detection in ListSkills
func TestSkillService_UpdateDetection(t *testing.T) {
	// Setup test environment
	tmpDir := t.TempDir()
	originalHome := os.Getenv("HOME")
	if originalHome == "" {
		originalHome = os.Getenv("USERPROFILE")
	}
	defer func() {
		if originalHome != "" {
			os.Setenv("HOME", originalHome)
			os.Setenv("USERPROFILE", originalHome)
		}
	}()

	os.Setenv("HOME", tmpDir)
	os.Setenv("USERPROFILE", tmpDir)

	// Create skill install directory
	installDir := filepath.Join(tmpDir, ".claude", "skills")
	os.MkdirAll(installDir, 0o755)

	// Create a local skill with version 1.0.0
	localSkillDir := filepath.Join(installDir, "test-skill")
	os.MkdirAll(localSkillDir, 0o755)

	// Write SKILL.md with version 1.0.0
	skillMD := `---
name: Test Skill
description: A test skill
version: 1.0.0
author: Test Author
tags:
  - test
  - demo
---

# Test Skill

This is a test skill.
`
	os.WriteFile(filepath.Join(localSkillDir, "SKILL.md"), []byte(skillMD), 0o644)

	// Note: We can't easily test update detection without creating a mock GitHub repository
	// This test verifies that the skill is loaded with version information

	ss := NewSkillService()
	ss.installDir = installDir

	// Create store file
	storeDir := filepath.Join(tmpDir, skillStoreDir)
	os.MkdirAll(storeDir, 0o755)

	store := skillStore{
		Skills: map[string]skillState{
			"test-skill": {Installed: true},
		},
		Repos: []skillRepoConfig{},
	}
	ss.saveStoreLocked(store)

	// Test: Local skill should be loaded with version
	// Note: ListSkills would try to fetch from repositories, which would fail in tests
	// So we'll just test the local skill merge logic
	skillMap := make(map[string]Skill)
	ss.mergeLocalSkills(skillMap)

	if len(skillMap) != 1 {
		t.Fatalf("Expected 1 skill, got %d", len(skillMap))
	}

	skill := skillMap["test-skill"]
	if skill.Name != "Test Skill" {
		t.Errorf("Expected name 'Test Skill', got %q", skill.Name)
	}

	if skill.LocalVersion != "1.0.0" {
		t.Errorf("Expected local version '1.0.0', got %q", skill.LocalVersion)
	}

	if skill.Author != "Test Author" {
		t.Errorf("Expected author 'Test Author', got %q", skill.Author)
	}

	if len(skill.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(skill.Tags))
	}

	if !skill.Installed {
		t.Errorf("Skill should be marked as installed")
	}
}

// Test CheckUpdates method
func TestSkillService_CheckUpdates(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	os.Setenv("USERPROFILE", tmpDir)

	installDir := filepath.Join(tmpDir, ".claude", "skills")
	os.MkdirAll(installDir, 0o755)

	// Create a local skill with old version
	localSkillDir := filepath.Join(installDir, "old-skill")
	os.MkdirAll(localSkillDir, 0o755)

	skillMD := `---
name: Old Skill
description: An outdated skill
version: 1.0.0
---

# Old Skill
`
	os.WriteFile(filepath.Join(localSkillDir, "SKILL.md"), []byte(skillMD), 0o644)

	ss := NewSkillService()
	ss.installDir = installDir

	// Create store with no repositories (to avoid GitHub fetches)
	storeDir := filepath.Join(tmpDir, skillStoreDir)
	os.MkdirAll(storeDir, 0o755)

	store := skillStore{
		Skills: map[string]skillState{
			"old-skill": {Installed: true},
		},
		Repos: []skillRepoConfig{}, // Empty repos
	}
	ss.saveStoreLocked(store)

	// Test CheckUpdates
	// Since we have no remote repos, this should return empty
	updatable, err := ss.CheckUpdates()
	if err != nil {
		t.Fatalf("CheckUpdates failed: %v", err)
	}

	if len(updatable) != 0 {
		t.Errorf("Expected 0 updatable skills (no remote repos), got %d", len(updatable))
	}
}

// Test UpdateSkill method
func TestSkillService_UpdateSkill(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	os.Setenv("USERPROFILE", tmpDir)

	installDir := filepath.Join(tmpDir, ".claude", "skills")
	os.MkdirAll(installDir, 0o755)

	// Create a local skill
	localSkillDir := filepath.Join(installDir, "test-skill")
	os.MkdirAll(localSkillDir, 0o755)

	skillMD := `---
name: Test Skill
version: 1.0.0
---

# Test Skill
`
	os.WriteFile(filepath.Join(localSkillDir, "SKILL.md"), []byte(skillMD), 0o644)

	ss := NewSkillService()
	ss.installDir = installDir

	storeDir := filepath.Join(tmpDir, skillStoreDir)
	os.MkdirAll(storeDir, 0o755)

	store := skillStore{
		Skills: map[string]skillState{
			"test-skill": {Installed: true},
		},
		Repos: []skillRepoConfig{}, // Empty repos
	}
	ss.saveStoreLocked(store)

	// Test: Update non-existent skill should fail
	err := ss.UpdateSkill("non-existent-skill")
	if err == nil {
		t.Errorf("Expected error for non-existent skill, got nil")
	}

	// Test: Update existing skill without remote repo should fail gracefully
	err = ss.UpdateSkill("test-skill")
	if err == nil {
		t.Errorf("Expected error for skill not found in repositories, got nil")
	}
	if err != nil && err.Error() != "skill test-skill 在仓库中未找到" {
		t.Errorf("Expected 'skill not found in repositories' error, got: %v", err)
	}
}

// Test DiscoverSkills method
func TestSkillService_DiscoverSkills(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)
	os.Setenv("USERPROFILE", tmpDir)

	installDir := filepath.Join(tmpDir, ".claude", "skills")
	os.MkdirAll(installDir, 0o755)

	ss := NewSkillService()
	ss.installDir = installDir

	storeDir := filepath.Join(tmpDir, skillStoreDir)
	os.MkdirAll(storeDir, 0o755)

	store := skillStore{
		Skills: map[string]skillState{},
		Repos:  []skillRepoConfig{}, // Empty repos
	}
	ss.saveStoreLocked(store)

	// Test: DiscoverSkills should work (same as ListSkills)
	// Note: Default repositories will be added automatically if repos list is empty
	skills, err := ss.DiscoverSkills()
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	// DiscoverSkills will fetch from default repos if none are specified
	// So we just verify it doesn't error out
	t.Logf("Discovered %d skills from default repositories", len(skills))
}

// Test skill metadata parsing with all fields
func TestParseSkillMetadataWithAllFields(t *testing.T) {
	content := `---
name: Advanced Skill
description: A skill with all metadata fields
version: 2.1.0
author: John Doe
tags:
  - advanced
  - test
  - automation
---

# Advanced Skill

This skill has complete metadata.
`

	meta, err := parseSkillMetadata(content)
	if err != nil {
		t.Fatalf("Failed to parse metadata: %v", err)
	}

	if meta.Name != "Advanced Skill" {
		t.Errorf("Expected name 'Advanced Skill', got %q", meta.Name)
	}

	if meta.Description != "A skill with all metadata fields" {
		t.Errorf("Expected description, got %q", meta.Description)
	}

	if meta.Version != "2.1.0" {
		t.Errorf("Expected version '2.1.0', got %q", meta.Version)
	}

	if meta.Author != "John Doe" {
		t.Errorf("Expected author 'John Doe', got %q", meta.Author)
	}

	if len(meta.Tags) != 3 {
		t.Errorf("Expected 3 tags, got %d", len(meta.Tags))
	}

	expectedTags := map[string]bool{"advanced": true, "test": true, "automation": true}
	for _, tag := range meta.Tags {
		if !expectedTags[tag] {
			t.Errorf("Unexpected tag: %q", tag)
		}
	}
}

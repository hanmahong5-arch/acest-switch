package services

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	skillStoreDir  = ".code-switch"
	skillStoreFile = "skill.json"
)

var (
	defaultRepoBranches = []string{"main", "master"}
	defaultSkillRepos   = []skillRepoConfig{
		{Owner: "ComposioHQ", Name: "awesome-claude-skills", Branch: "main", Enabled: true},
		{Owner: "anthropics", Name: "skills", Branch: "main", Enabled: true},
	}
)

type Skill struct {
	Key              string `json:"key"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Version          string `json:"version,omitempty"`           // Remote version (from repo)
	LocalVersion     string `json:"local_version,omitempty"`     // Installed version
	RemoteVersion    string `json:"remote_version,omitempty"`    // Latest available version
	UpdateAvailable  bool   `json:"update_available"`            // True if remote version > local version
	Directory        string `json:"directory"`
	ReadmeURL        string `json:"readme_url"`
	Installed        bool   `json:"installed"`
	RepoOwner        string `json:"repo_owner,omitempty"`
	RepoName         string `json:"repo_name,omitempty"`
	RepoBranch       string `json:"repo_branch,omitempty"`
	Author           string `json:"author,omitempty"`            // Skill author
	Tags             []string `json:"tags,omitempty"`            // Skill tags
}

type skillMetadata struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Version     string   `yaml:"version"`
	Author      string   `yaml:"author"`
	Tags        []string `yaml:"tags"`
}

type skillStore struct {
	Skills map[string]skillState `json:"skills"`
	Repos  []skillRepoConfig     `json:"repos"`
}

type skillState struct {
	Installed   bool      `json:"installed"`
	InstalledAt time.Time `json:"installed_at,omitempty"`
}

type skillRepoConfig struct {
	Owner   string `json:"owner"`
	Name    string `json:"name"`
	Branch  string `json:"branch"`
	Enabled bool   `json:"enabled"`
}

type installRequest struct {
	Directory string `json:"directory"`
	RepoOwner string `json:"repo_owner"`
	RepoName  string `json:"repo_name"`
	Branch    string `json:"repo_branch"`
}

type SkillService struct {
	httpClient *http.Client
	storePath  string
	installDir string
	mu         sync.Mutex
}

func NewSkillService() *SkillService {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return &SkillService{
		httpClient: &http.Client{Timeout: 60 * time.Second},
		storePath:  filepath.Join(home, skillStoreDir, skillStoreFile),
		installDir: filepath.Join(home, ".claude", "skills"),
	}
}

// ListSkills aggregates skills from configured repositories and the local install directory.
func (ss *SkillService) ListSkills() ([]Skill, error) {
	store, err := ss.loadStore()
	if err != nil {
		return nil, err
	}

	skillMap := make(map[string]Skill)
	for _, repo := range store.Repos {
		if !repo.Enabled {
			continue
		}
		repoDir, branch, cleanup, err := ss.prepareRepoSnapshot(repo)
		if err != nil {
			log.Printf("skill repo fetch failed for %s/%s: %v", repo.Owner, repo.Name, err)
			continue
		}
		entries, err := os.ReadDir(repoDir)
		if err != nil {
			cleanup()
			log.Printf("skill repo read failed for %s/%s: %v", repo.Owner, repo.Name, err)
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			dirKey := normalizeDirectoryKey(entry.Name())
			if _, exists := skillMap[dirKey]; exists {
				continue
			}
			skillPath := filepath.Join(repoDir, entry.Name())
			meta, err := readSkillMetadata(skillPath)
			if err != nil {
				continue
			}
			name := strings.TrimSpace(meta.Name)
			if name == "" {
				name = entry.Name()
			}
			key := buildSkillKey(repo.Owner, repo.Name, entry.Name())

			// Check version and update availability
			installed := ss.isInstalled(entry.Name())
			remoteVersion := strings.TrimSpace(meta.Version)
			localVersion := ""
			updateAvailable := false

			if installed {
				localMeta, _ := readSkillMetadata(filepath.Join(ss.installDir, entry.Name()))
				localVersion = strings.TrimSpace(localMeta.Version)
				if localVersion != "" && remoteVersion != "" {
					updateAvailable = compareVersion(localVersion, remoteVersion) < 0
				}
			}

			skillMap[dirKey] = Skill{
				Key:             key,
				Name:            name,
				Description:     strings.TrimSpace(meta.Description),
				Version:         remoteVersion,
				LocalVersion:    localVersion,
				RemoteVersion:   remoteVersion,
				UpdateAvailable: updateAvailable,
				Directory:       entry.Name(),
				ReadmeURL:       buildRepoURL(repo, branch, entry.Name()),
				Installed:       installed,
				RepoOwner:       repo.Owner,
				RepoName:        repo.Name,
				RepoBranch:      branch,
				Author:          strings.TrimSpace(meta.Author),
				Tags:            meta.Tags,
			}
		}
		cleanup()
	}

	ss.mergeLocalSkills(skillMap)
	skills := make([]Skill, 0, len(skillMap))
	for _, skill := range skillMap {
		skills = append(skills, skill)
	}
	sort.SliceStable(skills, func(i, j int) bool {
		li := strings.ToLower(skills[i].Name)
		lj := strings.ToLower(skills[j].Name)
		if li == lj {
			return strings.ToLower(skills[i].Directory) < strings.ToLower(skills[j].Directory)
		}
		return li < lj
	})
	return skills, nil
}

// InstallSkill installs a skill directory from the configured repositories.
func (ss *SkillService) InstallSkill(req installRequest) error {
	req.Directory = strings.TrimSpace(req.Directory)
	if req.Directory == "" {
		return errors.New("skill directory 不能为空")
	}
	store, err := ss.loadStore()
	if err != nil {
		return err
	}
	repos := ss.resolveReposForInstall(req, store.Repos)
	if len(repos) == 0 {
		return errors.New("未找到可用的技能仓库")
	}

	var lastErr error
	for _, repo := range repos {
		repoDir, _, cleanup, err := ss.prepareRepoSnapshot(repo)
		if err != nil {
			lastErr = err
			continue
		}
		skillPath := filepath.Join(repoDir, req.Directory)
		info, err := os.Stat(skillPath)
		if err != nil || !info.IsDir() {
			cleanup()
			lastErr = fmt.Errorf("仓库 %s/%s 中未找到 %s", repo.Owner, repo.Name, req.Directory)
			continue
		}
		if err := ss.installFromPath(req.Directory, skillPath); err != nil {
			cleanup()
			lastErr = err
			continue
		}
		cleanup()
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("skill %s 未找到", req.Directory)
	}
	return lastErr
}

func (ss *SkillService) installFromPath(directory, source string) error {
	if _, err := os.Stat(filepath.Join(source, "SKILL.md")); err != nil {
		return fmt.Errorf("%s 缺少 SKILL.md", directory)
	}
	if err := os.MkdirAll(ss.installDir, 0o755); err != nil {
		return err
	}
	target := filepath.Join(ss.installDir, directory)
	if err := os.RemoveAll(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := copyDirectory(source, target); err != nil {
		return err
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	store, err := ss.loadStoreLocked()
	if err != nil {
		return err
	}
	if store.Skills == nil {
		store.Skills = make(map[string]skillState)
	}
	store.Skills[directory] = skillState{Installed: true, InstalledAt: time.Now()}
	return ss.saveStoreLocked(store)
}

func (ss *SkillService) UninstallSkill(directory string) error {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return errors.New("skill directory 不能为空")
	}
	target := filepath.Join(ss.installDir, directory)
	if err := os.RemoveAll(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	store, err := ss.loadStoreLocked()
	if err != nil {
		return err
	}
	if store.Skills == nil {
		store.Skills = make(map[string]skillState)
	}
	delete(store.Skills, directory)
	return ss.saveStoreLocked(store)
}

// Repository management ----------------------------------------------------

func (ss *SkillService) ListRepos() ([]skillRepoConfig, error) {
	store, err := ss.loadStore()
	if err != nil {
		return nil, err
	}
	return cloneRepoConfigs(store.Repos), nil
}

func (ss *SkillService) AddRepo(repo skillRepoConfig) ([]skillRepoConfig, error) {
	repo = normalizeRepoConfig(repo)
	if err := validateRepoConfig(repo); err != nil {
		return nil, err
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	store, err := ss.loadStoreLocked()
	if err != nil {
		return nil, err
	}
	replaced := false
	for i := range store.Repos {
		if equalRepo(store.Repos[i], repo) {
			store.Repos[i] = repo
			replaced = true
			break
		}
	}
	if !replaced {
		store.Repos = append(store.Repos, repo)
	}
	if err := ss.saveStoreLocked(store); err != nil {
		return nil, err
	}
	return cloneRepoConfigs(store.Repos), nil
}

func (ss *SkillService) RemoveRepo(owner, name string) ([]skillRepoConfig, error) {
	owner = strings.TrimSpace(owner)
	name = strings.TrimSpace(name)
	if owner == "" || name == "" {
		return nil, errors.New("owner/name 不能为空")
	}
	ss.mu.Lock()
	defer ss.mu.Unlock()
	store, err := ss.loadStoreLocked()
	if err != nil {
		return nil, err
	}
	filtered := make([]skillRepoConfig, 0, len(store.Repos))
	for _, repo := range store.Repos {
		if strings.EqualFold(repo.Owner, owner) && strings.EqualFold(repo.Name, name) {
			continue
		}
		filtered = append(filtered, repo)
	}
	if len(filtered) == 0 {
		filtered = cloneDefaultRepos()
	}
	store.Repos = filtered
	if err := ss.saveStoreLocked(store); err != nil {
		return nil, err
	}
	return cloneRepoConfigs(store.Repos), nil
}

// Internal helpers ---------------------------------------------------------

func (ss *SkillService) loadStore() (skillStore, error) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	return ss.loadStoreLocked()
}

func (ss *SkillService) loadStoreLocked() (skillStore, error) {
	data, err := os.ReadFile(ss.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			store := skillStore{Skills: make(map[string]skillState)}
			store.ensureRepos()
			return store, nil
		}
		return skillStore{Skills: make(map[string]skillState)}, err
	}
	store := skillStore{}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &store); err != nil {
			return skillStore{Skills: make(map[string]skillState)}, err
		}
	}
	if store.Skills == nil {
		store.Skills = make(map[string]skillState)
	}
	store.ensureRepos()
	return store, nil
}

func (store *skillStore) ensureRepos() {
	if len(store.Repos) == 0 {
		store.Repos = cloneDefaultRepos()
	}
	for i := range store.Repos {
		store.Repos[i] = normalizeRepoConfig(store.Repos[i])
		if !store.Repos[i].Enabled {
			store.Repos[i].Enabled = true
		}
	}
}

func cloneDefaultRepos() []skillRepoConfig {
	repos := make([]skillRepoConfig, len(defaultSkillRepos))
	copy(repos, defaultSkillRepos)
	return repos
}

func cloneRepoConfigs(repos []skillRepoConfig) []skillRepoConfig {
	copyRepos := make([]skillRepoConfig, len(repos))
	copy(copyRepos, repos)
	return copyRepos
}

func normalizeRepoConfig(repo skillRepoConfig) skillRepoConfig {
	repo.Owner = strings.TrimSpace(repo.Owner)
	repo.Name = strings.TrimSpace(repo.Name)
	repo.Branch = strings.TrimSpace(repo.Branch)
	if repo.Branch == "" {
		repo.Branch = "main"
	}
	if !repo.Enabled {
		repo.Enabled = true
	}
	return repo
}

func validateRepoConfig(repo skillRepoConfig) error {
	if repo.Owner == "" || repo.Name == "" {
		return errors.New("owner/name 不能为空")
	}
	return nil
}

func equalRepo(a, b skillRepoConfig) bool {
	return strings.EqualFold(a.Owner, b.Owner) && strings.EqualFold(a.Name, b.Name)
}

func (ss *SkillService) saveStoreLocked(store skillStore) error {
	if err := os.MkdirAll(filepath.Dir(ss.storePath), 0o755); err != nil {
		return err
	}
	store.ensureRepos()
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	tmp := ss.storePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, ss.storePath)
}

func (ss *SkillService) prepareRepoSnapshot(repo skillRepoConfig) (string, string, func(), error) {
	tmpDir, err := os.MkdirTemp("", "skill-repo-")
	if err != nil {
		return "", "", nil, err
	}
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}
	archivePath := filepath.Join(tmpDir, "repo.zip")
	branches := buildBranchCandidates(repo.Branch)
	var lastErr error
	for _, branch := range branches {
		archiveURL := fmt.Sprintf("https://github.com/%s/%s/archive/refs/heads/%s.zip", repo.Owner, repo.Name, branch)
		if err := ss.downloadFile(archiveURL, archivePath); err != nil {
			lastErr = err
			continue
		}
		rootDir, err := unzipArchive(archivePath, tmpDir)
		if err != nil {
			lastErr = err
			continue
		}
		return rootDir, branch, cleanup, nil
	}
	cleanup()
	if lastErr == nil {
		lastErr = fmt.Errorf("无法下载仓库 %s/%s", repo.Owner, repo.Name)
	}
	return "", "", nil, lastErr
}

func buildBranchCandidates(preferred string) []string {
	set := make(map[string]struct{})
	ordered := make([]string, 0, len(defaultRepoBranches)+1)
	if preferred != "" {
		set[strings.ToLower(preferred)] = struct{}{}
		ordered = append(ordered, preferred)
	}
	for _, branch := range defaultRepoBranches {
		key := strings.ToLower(branch)
		if _, ok := set[key]; ok {
			continue
		}
		set[key] = struct{}{}
		ordered = append(ordered, branch)
	}
	return ordered
}

func (ss *SkillService) downloadFile(rawURL, dest string) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "ai-code-studio")
	resp, err := ss.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: %s", resp.Status)
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, resp.Body); err != nil {
		return err
	}
	return nil
}

func unzipArchive(zipPath, dest string) (string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	var root string
	for _, file := range reader.File {
		name := file.Name
		if name == "" {
			continue
		}
		if root == "" {
			root = strings.Split(name, "/")[0]
		}
		targetPath := filepath.Join(dest, name)
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return "", err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return "", err
		}
		src, err := file.Open()
		if err != nil {
			return "", err
		}
		dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
		if err != nil {
			src.Close()
			return "", err
		}
		if _, err := io.Copy(dst, src); err != nil {
			src.Close()
			dst.Close()
			return "", err
		}
		src.Close()
		dst.Close()
	}
	if root == "" {
		return "", errors.New("压缩包内容为空")
	}
	return filepath.Join(dest, root), nil
}

func (ss *SkillService) mergeLocalSkills(skills map[string]Skill) {
	entries, err := os.ReadDir(ss.installDir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := entry.Name()
		dirKey := normalizeDirectoryKey(dir)
		if existing, ok := skills[dirKey]; ok {
			existing.Installed = true
			skills[dirKey] = existing
			continue
		}
		meta, err := readSkillMetadata(filepath.Join(ss.installDir, dir))
		name := strings.TrimSpace(meta.Name)
		desc := strings.TrimSpace(meta.Description)
		version := strings.TrimSpace(meta.Version)
		author := strings.TrimSpace(meta.Author)
		if err != nil || name == "" {
			name = dir
		}
		skills[dirKey] = Skill{
			Key:          buildSkillKey("", "", dir),
			Name:         name,
			Description:  desc,
			LocalVersion: version,
			Directory:    dir,
			ReadmeURL:    "",
			Installed:    true,
			Author:       author,
			Tags:         meta.Tags,
		}
	}
}

func (ss *SkillService) resolveReposForInstall(req installRequest, repos []skillRepoConfig) []skillRepoConfig {
	owner := strings.TrimSpace(req.RepoOwner)
	name := strings.TrimSpace(req.RepoName)
	var target []skillRepoConfig
	if owner != "" && name != "" {
		for _, repo := range repos {
			if !repo.Enabled {
				continue
			}
			if strings.EqualFold(repo.Owner, owner) && strings.EqualFold(repo.Name, name) {
				target = append(target, repo)
			}
		}
		return target
	}
	for _, repo := range repos {
		if repo.Enabled {
			target = append(target, repo)
		}
	}
	return target
}

func buildRepoURL(repo skillRepoConfig, branch, directory string) string {
	dir := strings.Trim(directory, "/")
	if dir == "" {
		return fmt.Sprintf("https://github.com/%s/%s", repo.Owner, repo.Name)
	}
	return fmt.Sprintf("https://github.com/%s/%s/tree/%s/%s", repo.Owner, repo.Name, branch, dir)
}

func buildSkillKey(owner, name, directory string) string {
	owner = strings.ToLower(strings.TrimSpace(owner))
	name = strings.ToLower(strings.TrimSpace(name))
	directory = strings.ToLower(directory)
	if owner == "" && name == "" {
		return fmt.Sprintf("local:%s", directory)
	}
	return fmt.Sprintf("%s/%s:%s", owner, name, directory)
}

func normalizeDirectoryKey(directory string) string {
	return strings.ToLower(strings.TrimSpace(directory))
}

func (ss *SkillService) isInstalled(directory string) bool {
	info, err := os.Stat(filepath.Join(ss.installDir, directory))
	return err == nil && info.IsDir()
}

func readSkillMetadata(dir string) (skillMetadata, error) {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return skillMetadata{}, err
	}
	return parseSkillMetadata(string(data))
}

func parseSkillMetadata(content string) (skillMetadata, error) {
	var meta skillMetadata
	content = strings.TrimLeft(content, "\ufeff")
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return meta, errors.New("SKILL.md 缺少 front matter")
	}
	frontMatter := strings.TrimSpace(parts[1])
	if err := yaml.Unmarshal([]byte(frontMatter), &meta); err != nil {
		return meta, err
	}
	return meta, nil
}

func copyDirectory(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			if rel == "." {
				return os.MkdirAll(dst, 0o755)
			}
			return os.MkdirAll(target, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

// UpdateSkill updates an installed skill to the latest version
func (ss *SkillService) UpdateSkill(directory string) error {
	directory = strings.TrimSpace(directory)
	if directory == "" {
		return errors.New("skill directory 不能为空")
	}

	// Check if skill is installed
	if !ss.isInstalled(directory) {
		return fmt.Errorf("skill %s 未安装", directory)
	}

	// Find the skill in repositories
	store, err := ss.loadStore()
	if err != nil {
		return err
	}

	// Try to find skill in enabled repositories
	var foundSkill *Skill
	for _, repo := range store.Repos {
		if !repo.Enabled {
			continue
		}
		repoDir, _, cleanup, err := ss.prepareRepoSnapshot(repo)
		if err != nil {
			continue
		}

		skillPath := filepath.Join(repoDir, directory)
		info, err := os.Stat(skillPath)
		if err == nil && info.IsDir() {
			// Found the skill, now update it
			if err := ss.installFromPath(directory, skillPath); err != nil {
				cleanup()
				return fmt.Errorf("更新失败: %w", err)
			}
			cleanup()

			// Read version after update
			meta, _ := readSkillMetadata(filepath.Join(ss.installDir, directory))
			foundSkill = &Skill{
				Directory: directory,
				Version:   strings.TrimSpace(meta.Version),
			}
			break
		}
		cleanup()
	}

	if foundSkill == nil {
		return fmt.Errorf("skill %s 在仓库中未找到", directory)
	}

	return nil
}

// DiscoverSkills discovers all available skills with metadata (alias for ListSkills with update check)
func (ss *SkillService) DiscoverSkills() ([]Skill, error) {
	return ss.ListSkills()
}

// CheckUpdates checks for updates for all installed skills
func (ss *SkillService) CheckUpdates() ([]Skill, error) {
	skills, err := ss.ListSkills()
	if err != nil {
		return nil, err
	}

	// Filter only installed skills with updates available
	updatable := make([]Skill, 0)
	for _, skill := range skills {
		if skill.Installed && skill.UpdateAvailable {
			updatable = append(updatable, skill)
		}
	}

	return updatable, nil
}

// compareVersion compares two semantic version strings
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func compareVersion(v1, v2 string) int {
	if v1 == v2 {
		return 0
	}
	if v1 == "" {
		return -1
	}
	if v2 == "" {
		return 1
	}

	// Normalize versions (remove 'v' prefix)
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	parts1 := strings.Split(v1, ".")
	parts2 := strings.Split(v2, ".")

	maxLen := len(parts1)
	if len(parts2) > maxLen {
		maxLen = len(parts2)
	}

	for i := 0; i < maxLen; i++ {
		var num1, num2 int

		if i < len(parts1) {
			// Parse number part (ignore non-numeric suffixes like "1.0.0-alpha")
			part := parts1[i]
			if idx := strings.IndexAny(part, "-+"); idx >= 0 {
				part = part[:idx]
			}
			fmt.Sscanf(part, "%d", &num1)
		}

		if i < len(parts2) {
			part := parts2[i]
			if idx := strings.IndexAny(part, "-+"); idx >= 0 {
				part = part[:idx]
			}
			fmt.Sscanf(part, "%d", &num2)
		}

		if num1 < num2 {
			return -1
		}
		if num1 > num2 {
			return 1
		}
	}

	return 0
}

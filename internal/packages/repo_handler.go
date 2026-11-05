package packages

import (
	"bytes"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
)

// sourcesDir is where apt reads additional repository definitions from.
const sourcesDir = "/etc/apt/sources.list.d"

// validRepoName restricts repository names so they are safe to use in a file name.
var validRepoName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// NewRepoHandler constructs a new instance of a RepoHandler.
func NewRepoHandler(system system.Worker, repos map[string]config.RepositoryConfig) *RepoHandler {
	return &RepoHandler{
		Repos:  repos,
		system: system,
	}
}

// RepoHandler can add or remove a set of third-party apt repositories.
type RepoHandler struct {
	Repos  map[string]config.RepositoryConfig
	system system.Worker
}

// Prepare adds the configured repositories to the host's apt sources.
func (h *RepoHandler) Prepare() error {
	for _, name := range slices.Sorted(maps.Keys(h.Repos)) {
		err := h.addRepo(name, h.Repos[name])
		if err != nil {
			return fmt.Errorf("failed to add apt repository '%s': %w", name, err)
		}
		slog.Info("Added apt repository", "repository", name)
	}
	return nil
}

// Restore removes the configured repositories from the host's apt sources.
func (h *RepoHandler) Restore() error {
	for _, name := range slices.Sorted(maps.Keys(h.Repos)) {
		err := h.removeRepo(name, h.Repos[name])
		if err != nil {
			return fmt.Errorf("failed to remove apt repository '%s': %w", name, err)
		}
		slog.Info("Removed apt repository", "repository", name)
	}
	return nil
}

func (h *RepoHandler) addRepo(name string, repo config.RepositoryConfig) error {
	if repo.PPA != "" {
		// -n skips apt-get update, which the deb handler runs before installing.
		cmd := system.NewCommand("add-apt-repository", []string{"-y", "-n", "ppa:" + repo.PPA})
		_, err := system.RunExclusive(h.system, cmd)
		return err
	}

	path, err := sourcesPath(name)
	if err != nil {
		return err
	}
	if !strings.HasPrefix(repo.URL, "https://") && !strings.HasPrefix(repo.URL, "http://") {
		return fmt.Errorf("url must start with https:// or http://, got '%s'", repo.URL)
	}
	if !strings.HasPrefix(repo.Key, "https://") {
		return fmt.Errorf("key must be an https:// URL, got '%s'", repo.Key)
	}
	if len(repo.Components) == 0 {
		return fmt.Errorf("at least one component is required")
	}

	suite := repo.Suite
	if suite == "" {
		suite, err = h.releaseCodename()
		if err != nil {
			return err
		}
	}

	// Read-only so that dry-run still fetches and validates the key.
	cmd := system.NewCommand("curl", []string{"-fsSL", repo.Key})
	cmd.ReadOnly = true
	key, err := h.system.Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to download signing key: %w", err)
	}
	key = bytes.TrimSpace(key)
	if !bytes.HasPrefix(key, []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----")) {
		return fmt.Errorf("signing key at '%s' is not an ASCII-armored PGP public key", repo.Key)
	}

	return h.system.WriteFile(path, []byte(sourcesFile(repo.URL, suite, repo.Components, string(key))), 0o644)
}

func (h *RepoHandler) removeRepo(name string, repo config.RepositoryConfig) error {
	if repo.PPA != "" {
		cmd := system.NewCommand("add-apt-repository", []string{"-y", "-n", "-r", "ppa:" + repo.PPA})
		_, err := system.RunExclusive(h.system, cmd)
		return err
	}

	path, err := sourcesPath(name)
	if err != nil {
		return err
	}
	return h.system.RemovePath(path)
}

// releaseCodename returns the host's release codename, e.g. "noble".
func (h *RepoHandler) releaseCodename() (string, error) {
	osRelease, err := h.system.ReadFile("/etc/os-release")
	if err != nil {
		return "", fmt.Errorf("failed to read release codename: %w", err)
	}
	for line := range strings.Lines(string(osRelease)) {
		if codename, ok := strings.CutPrefix(strings.TrimSpace(line), "VERSION_CODENAME="); ok && codename != "" {
			return strings.Trim(codename, `"`), nil
		}
	}
	return "", fmt.Errorf("no VERSION_CODENAME in /etc/os-release")
}

// sourcesPath returns the path of the deb822 sources file concierge owns for a repository.
func sourcesPath(name string) (string, error) {
	if !validRepoName.MatchString(name) {
		return "", fmt.Errorf("invalid repository name '%s'", name)
	}
	return filepath.Join(sourcesDir, "concierge-"+name+".sources"), nil
}

// sourcesFile renders a deb822 sources file with the signing key embedded in Signed-By.
func sourcesFile(url, suite string, components []string, key string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Types: deb\nURIs: %s\nSuites: %s\nComponents: %s\nSigned-By:\n", url, suite, strings.Join(components, " "))
	for line := range strings.Lines(key) {
		line = strings.TrimRight(line, "\r\n")
		// deb822 continuation lines are indented; empty lines are written as " .".
		if line == "" {
			line = "."
		}
		fmt.Fprintf(&b, " %s\n", line)
	}
	return b.String()
}

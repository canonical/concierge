package packages

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
)

// sourcesDir is where apt reads additional repository definitions from.
const sourcesDir = "/etc/apt/sources.list.d"

// Commands that use the network are retried, because they can fail on a freshly
// booted machine before DNS is up. Launchpad often takes tens of seconds to answer
// add-apt-repository, so PPAs get a longer retry window than key downloads.
const (
	keyRetry = time.Minute
	ppaRetry = 5 * time.Minute
)

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

// ValidateRepository checks a repository's configuration without touching the host.
func ValidateRepository(name string, repo config.RepositoryConfig) error {
	if !validRepoName.MatchString(name) {
		return fmt.Errorf("invalid repository name '%s'", name)
	}

	if repo.PPA != "" {
		if repo.URL != "" || repo.Key != "" || repo.Suite != "" || len(repo.Components) > 0 {
			return fmt.Errorf("repository '%s': ppa cannot be combined with url, key, suite or components", name)
		}
		return nil
	}

	if repo.URL == "" {
		return fmt.Errorf("repository '%s': either ppa or url must be set", name)
	}
	if !strings.HasPrefix(repo.URL, "https://") && !strings.HasPrefix(repo.URL, "http://") {
		return fmt.Errorf("repository '%s': url must start with https:// or http://, got '%s'", name, repo.URL)
	}
	if !strings.HasPrefix(repo.Key, "https://") {
		return fmt.Errorf("repository '%s': key must be an https:// URL, got '%s'", name, repo.Key)
	}
	if len(repo.Components) == 0 {
		return fmt.Errorf("repository '%s': at least one component is required", name)
	}

	// These values go into the sources file, where a space would add another
	// value and a newline another field, such as "Trusted: yes".
	for _, value := range append([]string{repo.URL, repo.Suite}, repo.Components...) {
		if strings.ContainsFunc(value, unicode.IsSpace) {
			return fmt.Errorf("repository '%s': url, suite and components cannot contain whitespace, got %q", name, value)
		}
	}
	return nil
}

func (h *RepoHandler) addRepo(name string, repo config.RepositoryConfig) error {
	if repo.PPA != "" {
		return h.runAddAptRepository("ppa:" + repo.PPA)
	}

	path := sourcesPath(name)
	suite := repo.Suite
	if suite == "" {
		var err error
		suite, err = h.releaseCodename()
		if err != nil {
			return err
		}
	}

	// Read-only so that dry-run still fetches and validates the key.
	// --proto also applies to redirects, so the key can't be served over plain HTTP.
	// Limit each attempt: by default curl waits minutes for an unreachable host.
	cmd := system.NewCommand("curl", []string{"-fsSL", "--proto", "=https", "--max-time", "30", repo.Key})
	cmd.ReadOnly = true
	key, err := system.RunWithRetries(h.system, cmd, keyRetry)
	if err != nil {
		return fmt.Errorf("failed to download signing key: %w", err)
	}
	key = bytes.TrimSpace(key)
	if !bytes.HasPrefix(key, []byte("-----BEGIN PGP PUBLIC KEY BLOCK-----")) {
		return fmt.Errorf("signing key at '%s' is not an ASCII-armored PGP public key", repo.Key)
	}

	err = h.system.WriteFile(path, []byte(sourcesFile(repo.URL, suite, repo.Components, string(key))), 0o644)
	if err != nil {
		return err
	}

	// Update this repository on its own, so a wrong url, suite or key fails here
	// and is removed again, rather than breaking every apt-get update on the host.
	cmd = aptCommand("update", "--error-on=any",
		"-o", "Dir::Etc::sourcelist="+path,
		"-o", "Dir::Etc::sourceparts=-",
		"-o", "APT::Get::List-Cleanup=0")
	_, err = system.RunExclusive(h.system, cmd)
	if err != nil {
		err = fmt.Errorf("apt cannot use the repository, check its url, suite and key: %w", err)
		return errors.Join(err, h.system.RemovePath(path))
	}
	return nil
}

func (h *RepoHandler) removeRepo(name string, repo config.RepositoryConfig) error {
	if repo.PPA != "" {
		return h.runAddAptRepository("-r", "ppa:"+repo.PPA)
	}

	return h.system.RemovePath(sourcesPath(name))
}

// runAddAptRepository runs add-apt-repository without updating the apt cache,
// which the deb handler does before installing. It runs under timeout because
// launchpadlib has no network timeout and hangs forever on a stalled connection.
func (h *RepoHandler) runAddAptRepository(args ...string) error {
	cmd := system.NewCommand("timeout", append([]string{"180", "add-apt-repository", "-y", "-n"}, args...))
	_, err := system.RunWithRetries(h.system, cmd, ppaRetry)
	return err
}

// releaseCodename returns the host's release codename, for example "noble".
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
// ValidateRepository has already checked that the name is safe to use in a path.
func sourcesPath(name string) string {
	return filepath.Join(sourcesDir, "concierge-"+name+".sources")
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

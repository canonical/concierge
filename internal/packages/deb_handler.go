package packages

import (
	"fmt"
	"log/slog"

	"github.com/canonical/concierge/internal/system"
)

// NewDeb constructs a new Deb instance.
func NewDeb(name string) *Deb {
	return &Deb{Name: name}
}

// Deb is a simple representation of a package installed from the Ubuntu archive.
type Deb struct {
	Name string
}

// NewDebHandler constructs a new instance of a DebHandler.
func NewDebHandler(system system.Worker, debs []*Deb) *DebHandler {
	return &DebHandler{
		Debs:   debs,
		system: system,
	}
}

// DebHandler can install or remove a set of debs.
type DebHandler struct {
	Debs   []*Deb
	system system.Worker
}

// aptEnv contains environment variables that prevent apt/dpkg (and tools
// hooked into them, such as needrestart) from blocking on interactive
// prompts during unattended package operations.
var aptEnv = []string{
	"DEBIAN_FRONTEND=noninteractive",
	"NEEDRESTART_MODE=a",
}

// aptCommand constructs an apt-get command that runs non-interactively so
// package operations never hang waiting for user input.
func aptCommand(args ...string) *system.Command {
	cmd := system.NewCommand("apt-get", append([]string{"-y"}, args...))
	cmd.Env = aptEnv
	return cmd
}

// Prepare updates the apt cache and installs a set of debs from the archive.
func (h *DebHandler) Prepare() error {
	if len(h.Debs) == 0 {
		return nil
	}

	err := h.updateAptCache()
	if err != nil {
		return fmt.Errorf("failed to update apt cache: %w", err)
	}

	for _, deb := range h.Debs {
		err := h.installDeb(deb)
		if err != nil {
			return fmt.Errorf("failed to install deb: %w", err)
		}
	}
	return nil
}

// Restore removes a set of debs from the machine.
func (h *DebHandler) Restore() error {
	for _, deb := range h.Debs {
		err := h.removeDeb(deb)
		if err != nil {
			return fmt.Errorf("failed to remove deb: %w", err)
		}
	}

	cmd := aptCommand("autoremove")

	_, err := system.RunExclusive(h.system, cmd)
	if err != nil {
		return fmt.Errorf("failed to install apt package: %w", err)
	}

	return nil
}

// installDeb uses `apt` to install the package on the system from the archives.
func (h *DebHandler) installDeb(d *Deb) error {
	cmd := aptCommand("install",
		"-o", "Dpkg::Options::=--force-confdef",
		"-o", "Dpkg::Options::=--force-confold",
		d.Name)

	_, err := system.RunExclusive(h.system, cmd)
	if err != nil {
		return fmt.Errorf("failed to install apt package '%s': %w", d.Name, err)
	}

	slog.Info("Installed apt package", "package", d.Name)
	return nil
}

// Remove uninstalls the deb from the system with `apt`.
func (h *DebHandler) removeDeb(d *Deb) error {
	cmd := aptCommand("remove", d.Name)

	_, err := system.RunExclusive(h.system, cmd)
	if err != nil {
		return fmt.Errorf("failed to remove apt package '%s': %w", d.Name, err)
	}

	slog.Info("Removed apt package", "package", d.Name)
	return nil
}

// updateAptCache is a helper method to update the host's package cache.
func (h *DebHandler) updateAptCache() error {
	cmd := aptCommand("update")

	_, err := system.RunExclusive(h.system, cmd)
	if err != nil {
		return fmt.Errorf("failed to update apt package lists: %w", err)
	}

	return nil
}

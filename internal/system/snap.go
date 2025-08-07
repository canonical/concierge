package system

import (
	"fmt"
	"log/slog"
	"os/exec"
	"slices"
	"strings"
)

// SnapInfo represents information about a snap fetched from the snap CLI.
type SnapInfo struct {
	Installed bool
	Classic   bool
}

// Snap represents a given snap on a given channel.
type Snap struct {
	Name        string
	Channel     string
	Connections []string
}

// NewSnap returns a new Snap package.
func NewSnap(name, channel string, connections []string) *Snap {
	return &Snap{Name: name, Channel: channel, Connections: connections}
}

// NewSnapFromString returns a constructed snap instance, where the snap is
// specified in shorthand form, i.e. `charmcraft/latest/edge`.
func NewSnapFromString(snap string) *Snap {
	before, after, found := strings.Cut(snap, "/")
	if found {
		return NewSnap(before, after, []string{})
	} else {
		return NewSnap(before, "", []string{})
	}
}

// SnapInfo returns information about a given snap, looking up details using
// the snap CLI where necessary.
func (s *System) SnapInfo(snap string, channel string) (*SnapInfo, error) {
	classic, err := s.snapIsClassic(snap, channel)
	if err != nil {
		return nil, err
	}

	installed := s.snapInstalled(snap)

	slog.Debug("Queried snap CLI", "snap", snap, "installed", installed, "classic", classic)
	return &SnapInfo{Installed: installed, Classic: classic}, nil
}

// SnapChannels returns the list of channels available for a given snap.
func (s *System) SnapChannels(snap string) ([]string, error) {
	if _, err := exec.LookPath("snap"); err != nil {
		return nil, fmt.Errorf("snap command not found: %w", err)
	}

	// Use 'snap info <snap>' to get channel information.
	cmd := NewCommand("snap", []string{"info", snap})
	output, err := s.Run(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to get info for snap %s: %w", snap, err)
	}

	// The snap CLI only has human-readable output, so we need to parse it.
	channels := s.parseChannelsFromSnapInfo(string(output))

	if len(channels) == 0 {
		return nil, fmt.Errorf("no channels found for snap %s", snap)
	}

	slices.Sort(channels)
	slices.Reverse(channels)

	return channels, nil
}

// snapInstalled is a helper that reports if the snap is currently installed.
func (s *System) snapInstalled(name string) bool {
	cmd := NewCommand("snap", []string{"list", name})
	_, err := s.Run(cmd)
	return err == nil
}

// snapIsClassic reports whether or not the snap at the tip of the specified channel uses
// Classic confinement or not.
func (s *System) snapIsClassic(name, channel string) (bool, error) {
	cmd := NewCommand("snap", []string{"info", name})
	output, err := s.Run(cmd)
	if err != nil {
		return false, fmt.Errorf("failed to get info for snap %s: %w", name, err)
	}

	return s.parseConfinementFromSnapInfo(string(output), channel), nil
}

// parseChannelsFromSnapInfo extracts channel names from snap info output.
func (s *System) parseChannelsFromSnapInfo(output string) []string {
	var channels []string

	lines := strings.Split(output, "\n")
	inChannelsSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "channels:") {
			inChannelsSection = true
			continue
		}
		if inChannelsSection && line != "" && !strings.HasPrefix(line, " ") && !strings.Contains(line, "/") {
			break
		}
		if inChannelsSection && strings.Contains(line, "/") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				channel := parts[0]
				channel = strings.TrimSuffix(channel, ":")
				if channel != "" && !slices.Contains(channels, channel) {
					channels = append(channels, channel)
				}
			}
		}
	}

	return channels
}

// parseConfinementFromSnapInfo checks if a snap uses classic confinement.
func (s *System) parseConfinementFromSnapInfo(output, channel string) bool {
	lines := strings.Split(output, "\n")
	inChannelsSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.Contains(line, "channels:") {
			inChannelsSection = true
			continue
		}

		if inChannelsSection && line != "" && !strings.HasPrefix(line, " ") && !strings.Contains(line, "/") {
			break
		}

		if inChannelsSection && strings.Contains(line, "/") {
			if channel != "" && strings.Contains(line, channel) {
				return strings.Contains(line, "classic")
			}

			if channel == "" && strings.Contains(line, "classic") {
				return true
			}
		}
	}

	return false
}

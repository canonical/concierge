package system

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	retry "github.com/sethvargo/go-retry"
	client "github.com/snapcore/snapd/client"
)

// SnapInfo represents information about a snap fetched from the snapd API.
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

// SnapInfo returns information about a given snap, looking up details in the snap
// store using the snapd client API where necessary.
func (s *System) SnapInfo(snap string, channel string) (*SnapInfo, error) {
	classic, err := s.snapIsClassic(snap, channel)
	if err != nil {
		return nil, err
	}

	installed := s.snapInstalled(snap)

	slog.Debug("Queried snapd API", "snap", snap, "installed", installed, "classic", classic)
	return &SnapInfo{Installed: installed, Classic: classic}, nil
}

// snapInstalled is a helper that reports if the snap is currently Installed.
func (s *System) snapInstalled(name string) bool {
	snap, err := s.withRetry(func(ctx context.Context) (*client.Snap, error) {
		snap, _, err := s.snapd.Snap(name)
		if err != nil && strings.Contains(err.Error(), "snap not installed") {
			return snap, nil
		} else if err != nil {
			return nil, retry.RetryableError(err)
		}
		return snap, nil
	})
	if err != nil || snap == nil {
		return false
	}

	return snap.Status == client.StatusActive
}

// snapIsClassic reports whether or not the snap at the tip of the specified channel uses
// Classic confinement or not.
func (s *System) snapIsClassic(name, channel string) (bool, error) {
	snap, err := s.withRetry(func(ctx context.Context) (*client.Snap, error) {
		snap, _, err := s.snapd.FindOne(name)
		if err != nil {
			return nil, retry.RetryableError(err)
		}
		return snap, nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to find snap: %w", err)
	}

	c, ok := snap.Channels[channel]
	if ok {
		return c.Confinement == "classic", nil
	}

	return snap.Confinement == "classic", nil
}

func (s *System) withRetry(f func(ctx context.Context) (*client.Snap, error)) (*client.Snap, error) {
	backoff := retry.NewExponential(1 * time.Second)
	backoff = retry.WithMaxRetries(10, backoff)
	ctx := context.Background()
	return retry.DoValue(ctx, backoff, f)
}

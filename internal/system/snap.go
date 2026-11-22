package system

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/canonical/concierge/internal/snapd"
	retry "github.com/sethvargo/go-retry"
)

// InstalledSnapInfo represents information about the locally installed copy of a
// snap, fetched from the snapd API without querying the snap store.
type InstalledSnapInfo struct {
	Installed       bool
	Active          bool
	TrackingChannel string
	Revision        string
	Version         string
}

// SnapInfo represents information about a snap fetched from the snapd API,
// including details that require querying the snap store.
type SnapInfo struct {
	InstalledSnapInfo
	Classic bool
}

// Snap represents a given snap on a given channel.
type Snap struct {
	Name    string
	Channel string
	// DefaultChannel is the channel to install from when Channel is empty and the
	// snap is not yet installed. An installed snap stays on its tracking channel.
	DefaultChannel string
	Revision       string
	Connections    []string
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

	installed, err := s.SnapInstalledInfo(context.Background(), snap)
	if err != nil {
		return nil, err
	}
	info := &SnapInfo{InstalledSnapInfo: *installed, Classic: classic}

	attrs := []any{"snap", snap, "installed", info.Installed, "classic", classic}
	if info.Installed {
		attrs = append(attrs, "active", info.Active, "version", info.Version, "revision", info.Revision, "tracking", info.TrackingChannel)
	}
	slog.Debug("Queried snapd API", attrs...)
	return info, nil
}

// SnapChannels returns the list of channels available for a given snap.
func (s *System) SnapChannels(snap string) ([]string, error) {
	// Fetch the channels from
	if _, err := os.Stat("/run/snapd.socket"); errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	snapInfo, err := s.withRetry(context.Background(), func(ctx context.Context) (*snapd.Snap, error) {
		snap, err := s.snapd.FindOne(ctx, snap)
		if err != nil {
			if errors.Is(err, snapd.ErrNotFound) {
				return nil, err
			}
			return nil, retry.RetryableError(err)

		}
		return snap, nil
	})
	if err != nil {
		return nil, err
	}

	channels := make([]string, len(snapInfo.Channels))

	i := 0
	for k := range snapInfo.Channels {
		channels[i] = k
		i++
	}

	slices.Sort(channels)
	slices.Reverse(channels)

	return channels, nil
}

// SnapInstalledInfo reports if the snap is currently installed, along with its
// tracking channel, revision and version, using only the local snapd API. The
// tracking channel is the channel the snap is currently following (e.g.,
// "latest/stable"). A snap that is not installed is not an error, but failing
// to query snapd is. Failed queries are retried until the retry limit is reached
// or ctx is done.
func (s *System) SnapInstalledInfo(ctx context.Context, name string) (*InstalledSnapInfo, error) {
	snap, err := s.withRetry(ctx, func(ctx context.Context) (*snapd.Snap, error) {
		snap, err := s.snapd.Snap(ctx, name)
		if err != nil && errors.Is(err, snapd.ErrNotInstalled) {
			return nil, nil
		} else if err != nil {
			return nil, retry.RetryableError(err)
		}
		return snap, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query installed snap %q: %w", name, err)
	}

	if snap == nil || (snap.Status != snapd.StatusActive && snap.Status != snapd.StatusInstalled) {
		return &InstalledSnapInfo{}, nil
	}

	tc := snap.TrackingChannel
	if tc == "" {
		tc = snap.Channel
	}
	return &InstalledSnapInfo{
		Installed:       true,
		Active:          snap.Status == snapd.StatusActive,
		TrackingChannel: tc,
		Revision:        snap.Revision,
		Version:         snap.Version,
	}, nil
}

// snapIsClassic reports whether or not the snap at the tip of the specified channel uses
// Classic confinement or not.
func (s *System) snapIsClassic(name, channel string) (bool, error) {
	snap, err := s.withRetry(context.Background(), func(ctx context.Context) (*snapd.Snap, error) {
		snap, err := s.snapd.FindOne(ctx, name)
		if err != nil {
			if errors.Is(err, snapd.ErrNotFound) {
				return nil, err
			}
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

func (s *System) withRetry(ctx context.Context, f func(ctx context.Context) (*snapd.Snap, error)) (*snapd.Snap, error) {
	backoff := retry.NewExponential(1 * time.Second)
	backoff = retry.WithMaxRetries(10, backoff)
	return retry.DoValue(ctx, backoff, f)
}

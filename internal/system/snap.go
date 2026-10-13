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

// SnapInfo represents information about a snap fetched from the snapd API.
type SnapInfo struct {
	Installed       bool
	Active          bool
	Classic         bool
	TrackingChannel string
	Revision        string
	Version         string
}

// Snap represents a given snap on a given channel.
type Snap struct {
	Name        string
	Channel     string
	Revision    string
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

	info := s.SnapInstalledInfo(snap)
	info.Classic = classic

	if info.Installed {
		slog.Debug("Queried snapd API", "snap", snap, "installed", true, "active", info.Active, "classic", classic, "version", info.Version, "revision", info.Revision, "tracking", info.TrackingChannel)
	} else {
		slog.Debug("Queried snapd API", "snap", snap, "installed", false, "classic", classic)
	}
	return info, nil
}

// SnapChannels returns the list of channels available for a given snap.
func (s *System) SnapChannels(snap string) ([]string, error) {
	// Fetch the channels from
	if _, err := os.Stat("/run/snapd.socket"); errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	snapInfo, err := s.withRetry(func(ctx context.Context) (*snapd.Snap, error) {
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
// "latest/stable"). The Classic field is not populated.
func (s *System) SnapInstalledInfo(name string) *SnapInfo {
	snap, err := s.withRetry(func(ctx context.Context) (*snapd.Snap, error) {
		snap, err := s.snapd.Snap(ctx, name)
		if err != nil && errors.Is(err, snapd.ErrNotInstalled) {
			return snap, nil
		} else if err != nil {
			return nil, retry.RetryableError(err)
		}
		return snap, nil
	})
	if err != nil || snap == nil {
		return &SnapInfo{}
	}

	if snap.Status == snapd.StatusActive || snap.Status == snapd.StatusInstalled {
		tc := snap.TrackingChannel
		if tc == "" {
			tc = snap.Channel
		}
		return &SnapInfo{
			Installed:       true,
			Active:          snap.Status == snapd.StatusActive,
			TrackingChannel: tc,
			Revision:        snap.Revision,
			Version:         snap.Version,
		}
	}

	return &SnapInfo{}
}

// snapIsClassic reports whether or not the snap at the tip of the specified channel uses
// Classic confinement or not.
func (s *System) snapIsClassic(name, channel string) (bool, error) {
	snap, err := s.withRetry(func(ctx context.Context) (*snapd.Snap, error) {
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

func (s *System) withRetry(f func(ctx context.Context) (*snapd.Snap, error)) (*snapd.Snap, error) {
	backoff := retry.NewExponential(1 * time.Second)
	backoff = retry.WithMaxRetries(10, backoff)
	ctx := context.Background()
	return retry.DoValue(ctx, backoff, f)
}

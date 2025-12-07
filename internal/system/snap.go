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
	Classic         bool
	TrackingChannel string
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

	installed, trackingChannel := s.snapInstalledInfo(snap)

	slog.Debug("Queried snapd API", "snap", snap, "installed", installed, "classic", classic, "tracking", trackingChannel)
	return &SnapInfo{Installed: installed, Classic: classic, TrackingChannel: trackingChannel}, nil
}

// SnapChannels returns the list of channels available for a given snap.
func (s *System) SnapChannels(snap string) ([]string, error) {
	// Fetch the channels from
	if _, err := os.Stat("/run/snapd.socket"); errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	snapInfo, err := s.withRetry(func(ctx context.Context) (*snapd.Snap, error) {
		snap, err := s.snapd.FindOne(snap)
		if err != nil {
			if strings.Contains(err.Error(), "snap not found") {
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

// snapInstalledInfo is a helper that reports if the snap is currently installed
// and returns its tracking channel. The tracking channel is the channel the snap
// is currently following (e.g., "latest/stable"). Returns empty string if the
// snap is not installed or if the tracking channel cannot be determined.
func (s *System) snapInstalledInfo(name string) (bool, string) {
	snap, err := s.withRetry(func(ctx context.Context) (*snapd.Snap, error) {
		snap, err := s.snapd.Snap(name)
		if err != nil && strings.Contains(err.Error(), "snap not installed") {
			return snap, nil
		} else if err != nil {
			return nil, retry.RetryableError(err)
		}
		return snap, nil
	})
	if err != nil || snap == nil {
		return false, ""
	}

	if snap.Status == snapd.StatusActive {
		trackingChannel := snap.TrackingChannel
		if trackingChannel == "" {
			trackingChannel = snap.Channel
		}
		return true, trackingChannel
	}

	return false, ""
}

// snapIsClassic reports whether or not the snap at the tip of the specified channel uses
// Classic confinement or not.
func (s *System) snapIsClassic(name, channel string) (bool, error) {
	snap, err := s.withRetry(func(ctx context.Context) (*snapd.Snap, error) {
		snap, err := s.snapd.FindOne(name)
		if err != nil {
			if strings.Contains(err.Error(), "snap not found") {
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

package services

import (
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
)

func TestNewServiceDisabled(t *testing.T) {
	cfg := &config.Config{}
	sys := system.NewMockSystem()

	if s := NewService("microceph", sys, cfg, nil); s != nil {
		t.Fatalf("expected nil service when microceph is disabled, got: %v", s)
	}
}

func TestNewServiceEnabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Services.Microceph.Enable = true
	sys := system.NewMockSystem()

	s := NewService("microceph", sys, cfg, nil)
	if s == nil {
		t.Fatal("expected a microceph service to be returned")
	}

	if s.Name() != "microceph" {
		t.Fatalf("expected service name 'microceph', got: %v", s.Name())
	}
}

func TestNewServiceUnknown(t *testing.T) {
	cfg := &config.Config{}
	sys := system.NewMockSystem()

	if s := NewService("unknown", sys, cfg, nil); s != nil {
		t.Fatalf("expected nil for unknown service name, got: %v", s)
	}
}

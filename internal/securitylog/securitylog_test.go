package securitylog

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// decodeEvent configures the logger to write to a buffer, runs emit, and
// returns the decoded JSON record.
func decodeEvent(t *testing.T, emit func()) map[string]any {
	t.Helper()

	var buf bytes.Buffer
	Configure(&buf, "concierge@test")
	emit()

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to decode security event %q: %v", buf.String(), err)
	}
	return record
}

func TestEmitFields(t *testing.T) {
	record := decodeEvent(t, func() {
		Emit(EventSysStartup, "1000", "machine provisioning started", "action", "prepare")
	})

	checks := map[string]any{
		"type":        "security",
		"appid":       "concierge@test",
		"event":       "sys_startup:1000",
		"description": "machine provisioning started",
		"level":       "WARN",
		"action":      "prepare",
	}
	for key, want := range checks {
		if got := record[key]; got != want {
			t.Errorf("field %q = %v, want %v", key, got, want)
		}
	}

	// The OWASP vocabulary uses "datetime" for the timestamp rather than the
	// slog default of "time".
	if _, ok := record["datetime"]; !ok {
		t.Errorf("expected a datetime field, got %v", record)
	}
	if _, ok := record["time"]; ok {
		t.Errorf("did not expect a time field, got %v", record)
	}
}

func TestEmitWithoutArg(t *testing.T) {
	record := decodeEvent(t, func() {
		Emit(EventAuthzAdmin, "", "no arg")
	})
	if got := record["event"]; got != "authz_admin" {
		t.Errorf("event = %v, want bare authz_admin", got)
	}
}

func TestEmitAuthzAdminEventFormat(t *testing.T) {
	record := decodeEvent(t, func() {
		Emit(EventAuthzAdmin, "0,exec", "privileged command executed",
			"command", "snap install foo")
	})
	if got, ok := record["event"].(string); !ok || !strings.HasPrefix(got, "authz_admin:") {
		t.Errorf("event = %v, want authz_admin:<userid>,exec form", record["event"])
	}
	if got := record["level"]; got != "WARN" {
		t.Errorf("level = %v, want WARN", got)
	}
}

func TestUserIDIsNumeric(t *testing.T) {
	id := UserID()
	if id == "" {
		t.Fatal("UserID() returned empty string")
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			t.Fatalf("UserID() = %q, want only digits", id)
		}
	}
}

func TestConfigureDefaultDoesNotPanic(t *testing.T) {
	// ConfigureDefault tries to attach to /dev/log; on test hosts it may
	// succeed (CI runs on Ubuntu with journald) or fall back to stderr (some
	// stripped environments). Either path is acceptable — the contract is
	// just that it doesn't panic and leaves a usable logger behind.
	ConfigureDefault("concierge@test")
	Emit(EventSysStartup, UserID(), "configured-default smoke test")
}

func TestConfigureEmptyIDKeepsExisting(t *testing.T) {
	var buf bytes.Buffer
	Configure(&buf, "concierge@first")
	Configure(&buf, "")

	buf.Reset()
	Emit(EventSysShutdown, UserID(), "machine restoration started")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to decode security event: %v", err)
	}
	if got := record["appid"]; got != "concierge@first" {
		t.Errorf("appid = %v, want concierge@first", got)
	}
}

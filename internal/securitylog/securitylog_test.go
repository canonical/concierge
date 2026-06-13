package securitylog

import (
	"bytes"
	"encoding/json"
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
		Emit(EventSysStartup, "machine provisioning started", "action", "prepare")
	})

	checks := map[string]any{
		"type":        "security",
		"appid":       "concierge@test",
		"event":       "sys_startup",
		"description": "machine provisioning started",
		"level":       "INFO",
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

func TestEmitWarnLevel(t *testing.T) {
	record := decodeEvent(t, func() {
		EmitWarn(EventAuthzAdmin, "privileged command failed", "outcome", "failure")
	})

	if got := record["level"]; got != "WARN" {
		t.Errorf("level = %v, want WARN", got)
	}
	if got := record["event"]; got != "authz_admin" {
		t.Errorf("event = %v, want authz_admin", got)
	}
}

func TestConfigureEmptyIDKeepsExisting(t *testing.T) {
	var buf bytes.Buffer
	Configure(&buf, "concierge@first")
	Configure(&buf, "")

	buf.Reset()
	Emit(EventSysShutdown, "machine restoration started")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("failed to decode security event: %v", err)
	}
	if got := record["appid"]; got != "concierge@first" {
		t.Errorf("appid = %v, want concierge@first", got)
	}
}

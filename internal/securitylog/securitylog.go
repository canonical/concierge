// Package securitylog emits security-relevant events using the OWASP
// Application Logging Vocabulary, as required by Canonical's SEC0045
// (Security Event Logging) standard.
//
// concierge runs as root in order to provision charm development and testing
// machines: it executes privileged commands, installs snaps and debs, writes
// cloud credentials, and changes filesystem ownership. Those are exactly the
// kinds of security-relevant actions the OWASP vocabulary is meant to capture.
//
// Events are emitted as structured JSON so that they form a machine-parseable
// audit trail that is independent of concierge's human-readable logging
// verbosity (--verbose/--trace). Each record carries the fields recommended by
// the vocabulary: a "datetime" timestamp, a "level", a constant "type" of
// "security", an "appid" identifying the process, the OWASP "event" name, and a
// human-readable "description". This mirrors the approach taken for the
// operator framework in canonical/operator#1905.
//
// Structured JSON (rather than OTLP via the owasp-logger library) is used
// because concierge is a short-lived CLI with no existing telemetry pipeline;
// emitting JSON to stderr keeps the audit trail close to the existing slog
// output without pulling in an OTLP exporter and collector.
package securitylog

import (
	"context"
	"io"
	"log/slog"
	"os"
	"sync"
)

// Event names from the OWASP Application Logging Vocabulary that concierge
// emits. Only the subset relevant to concierge's behaviour is defined here.
const (
	// EventSysStartup records that a system (here, machine provisioning) has
	// been started. Emitted when concierge begins to prepare a machine.
	EventSysStartup = "sys_startup"
	// EventSysShutdown records that a system has been shut down. Emitted when
	// concierge restores (decommissions) a previously prepared machine.
	EventSysShutdown = "sys_shutdown"
	// EventAuthzAdmin records activity performed with administrative
	// privileges. concierge runs as root, so every privileged command it
	// executes, and the writing of cloud credentials, is administrative
	// activity.
	EventAuthzAdmin = "authz_admin"
	// EventPrivilegePermissionsChanged records a change to the permissions or
	// ownership of a resource. Emitted when concierge recursively changes the
	// ownership of files and directories it creates.
	EventPrivilegePermissionsChanged = "privilege_permissions_changed"
)

// securityType is the constant value of the "type" field on every security
// event, identifying the record as a security event for downstream tooling.
const securityType = "security"

var (
	mu     sync.Mutex
	logger *slog.Logger
	appID  = "concierge"
)

// Configure sets the destination writer and application identifier used for
// security events. It is intended to be called once during start-up. If it is
// never called, events are written to os.Stderr with an appid of "concierge".
// An empty id leaves the existing appid unchanged.
func Configure(w io.Writer, id string) {
	mu.Lock()
	defer mu.Unlock()
	if id != "" {
		appID = id
	}
	logger = newLogger(w)
}

// newLogger constructs a JSON logger whose field names follow the OWASP
// vocabulary: the slog time and message keys are renamed to "datetime" and
// "description" respectively.
func newLogger(w io.Writer) *slog.Logger {
	h := slog.NewJSONHandler(w, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			switch a.Key {
			case slog.TimeKey:
				a.Key = "datetime"
			case slog.MessageKey:
				a.Key = "description"
			}
			return a
		},
	})
	return slog.New(h)
}

// current returns the configured logger and appid, lazily initialising the
// logger against os.Stderr the first time it is needed.
func current() (*slog.Logger, string) {
	mu.Lock()
	defer mu.Unlock()
	if logger == nil {
		logger = newLogger(os.Stderr)
	}
	return logger, appID
}

// Emit records a security event at INFO level. event is one of the OWASP
// vocabulary event names; description is a human-readable summary; attrs are
// optional slog-style key/value pairs giving event-specific context. Callers
// must not pass secret values (such as credential contents) as attrs.
func Emit(event, description string, attrs ...any) {
	emit(slog.LevelInfo, event, description, attrs...)
}

// EmitWarn records a security event at WARNING level. It is used for
// security-relevant failures, such as a privileged command that did not
// succeed.
func EmitWarn(event, description string, attrs ...any) {
	emit(slog.LevelWarn, event, description, attrs...)
}

func emit(level slog.Level, event, description string, attrs ...any) {
	l, id := current()
	args := make([]any, 0, len(attrs)+6)
	args = append(args, "type", securityType, "appid", id, "event", event)
	args = append(args, attrs...)
	l.Log(context.Background(), level, description, args...)
}

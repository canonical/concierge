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
// human-readable "description". Per the vocabulary, the "event" field embeds
// event-specific parameters (such as the userid) as "name:arg".
//
// Structured JSON (rather than OTLP via the owasp-logger library) is used
// because concierge is a short-lived CLI with no existing telemetry pipeline;
// records are delivered to the system journal via syslog(3) so the audit
// stream stays separate from concierge's human-readable stderr output.
// `journalctl -t concierge` surfaces the events, and each record's JSON body
// can be parsed back out (e.g. `journalctl -t concierge -o cat | jq .`).
package securitylog

import (
	"context"
	"io"
	"log/slog"
	"log/syslog"
	"os"
	"strconv"
	"sync"
)

// Event names from the OWASP Application Logging Vocabulary that concierge
// emits. Only the subset relevant to concierge's behaviour is defined here.
// The cheat sheet specifies WARN as the level for each of these events.
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
// security events. It is intended for tests that need to capture the JSON
// output in a buffer; production callers should use ConfigureDefault. If
// Configure is never called, events are written to os.Stderr with an appid of
// "concierge". An empty id leaves the existing appid unchanged.
func Configure(w io.Writer, id string) {
	mu.Lock()
	defer mu.Unlock()
	if id != "" {
		appID = id
	}
	logger = newLogger(w)
}

// ConfigureDefault wires up the production destination: structured JSON audit
// records emitted to the system journal via syslog(3), tagged "concierge" so
// `journalctl -t concierge` returns the audit stream without mixing it into
// concierge's stderr output. If syslog is not reachable (such as a stripped-down
// container without /dev/log) the records fall back to os.Stderr so they are
// never silently dropped. The syslog priority is LOG_AUTHPRIV|LOG_WARNING;
// every event in concierge's vocabulary subset is WARN per the cheat sheet,
// and the per-record severity is also carried in the JSON "level" field.
func ConfigureDefault(id string) {
	w, err := syslog.New(syslog.LOG_AUTHPRIV|syslog.LOG_WARNING, "concierge")
	if err != nil {
		Configure(os.Stderr, id)
		return
	}
	Configure(w, id)
}

// UserID returns the userid string that should be embedded in OWASP event
// names. The effective UID is used: concierge runs as root, and SUDO_USER is
// not consulted because the privilege actually in play is root's. Callers
// compose this with any per-event sub-action, e.g.
// `securitylog.UserID() + ",exec"`.
func UserID() string {
	return strconv.Itoa(os.Getuid())
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

// Emit records a security event at WARN level — the level the OWASP Logging
// Vocabulary specifies for every event in concierge's subset (sys_startup,
// sys_shutdown, authz_admin, privilege_permissions_changed). event is one of
// the OWASP vocabulary event names. arg is the event's parameter list per the
// vocabulary schema (e.g. the userid for sys_startup; "userid,sub_event" for
// authz_admin; "userid,file" for privilege_permissions_changed); it is
// appended to the event name as "event:arg" in the JSON record. description
// is a human-readable summary; attrs are optional slog-style key/value pairs
// giving event-specific context. Callers must not pass secret values (such as
// credential contents) as attrs.
func Emit(event, arg, description string, attrs ...any) {
	l, id := current()
	eventField := event
	if arg != "" {
		eventField = event + ":" + arg
	}
	args := make([]any, 0, len(attrs)+6) // 3 key/value pairs appended below
	args = append(args, "type", securityType, "appid", id, "event", eventField)
	args = append(args, attrs...)
	l.Log(context.Background(), slog.LevelWarn, description, args...)
}

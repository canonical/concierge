package concierge

import (
	"fmt"

	"github.com/canonical/concierge/internal/services"
)

const (
	RestoreAction string = "restore"
	PrepareAction string = "prepare"
)

// Executable is an interface that represents any struct implementing the Prepare/Restore methods.
type Executable interface {
	Prepare() error
	Restore() error
}

// DoAction takes an Executable, and calls either Prepare() or Restore() according
// to the action parameter.
func DoAction(executable Executable, action string) error {
	switch action {
	case PrepareAction:
		return executable.Prepare()
	case RestoreAction:
		return executable.Restore()
	default:
		return fmt.Errorf("unknown executor action: %s", action)
	}
}

// doServicePrePhase calls Prepare() or Disintegrate() according to the action parameter.
func doServicePrePhase(s services.Service, action string) error {
	switch action {
	case PrepareAction:
		return s.Prepare()
	case RestoreAction:
		return s.Disintegrate()
	default:
		return fmt.Errorf("unknown executor action: %s", action)
	}
}

// doServicePostPhase calls Integrate() or Restore() according to the action parameter.
func doServicePostPhase(s services.Service, action string) error {
	switch action {
	case PrepareAction:
		return s.Integrate()
	case RestoreAction:
		return s.Restore()
	default:
		return fmt.Errorf("unknown executor action: %s", action)
	}
}

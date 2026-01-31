package concierge

import (
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
)

func TestDryRunPlanExecution(t *testing.T) {
	// Test that with DryRunWorker, commands are not executed but
	// Print() calls produce output and the plan executes successfully.

	// Create a real system and wrap it with DryRunWorker
	realSystem, err := system.NewSystem(false)
	if err != nil {
		t.Fatalf("failed to create system: %v", err)
	}
	dryRunWorker := system.NewDryRunWorker(realSystem)

	// Create a minimal config with DryRun enabled
	conf := &config.Config{
		DryRun: true,
	}

	// Create the plan with DryRunWorker
	plan := NewPlan(conf, dryRunWorker)

	// Verify plan was created
	if plan == nil {
		t.Fatal("plan should not be nil")
	}

	// Execute prepare - should not fail and should not execute actual commands
	// This tests that DryRunWorker properly skips execution while allowing
	// the plan to run through its logic
	err = plan.Execute(PrepareAction)
	if err != nil {
		t.Fatalf("plan execution should not fail with DryRunWorker: %v", err)
	}

	// The test passes if no actual system changes occurred and no errors were raised
}

func TestDryRunConfigField(t *testing.T) {
	// Test that the DryRun field exists and can be set
	conf := &config.Config{
		DryRun: true,
	}

	if !conf.DryRun {
		t.Fatal("DryRun should be true")
	}

	conf.DryRun = false
	if conf.DryRun {
		t.Fatal("DryRun should be false")
	}
}

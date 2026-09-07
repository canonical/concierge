package system

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// countingWorker wraps MockSystem to record how many times a command was run.
type countingWorker struct {
	*MockSystem
	runs int
}

func (c *countingWorker) Run(cmd *Command) ([]byte, error) {
	c.runs++
	return c.MockSystem.Run(cmd)
}

func TestRunWithRetriesPermanentErrorIsNotRetried(t *testing.T) {
	output := "Bootstrap config verification failed: pre-init checks failed for node"

	worker := &countingWorker{MockSystem: NewMockSystem()}
	worker.MockCommandReturn("k8s bootstrap", []byte(output), fmt.Errorf("exit status 1"))

	cmd := NewCommand("k8s", []string{"bootstrap"})
	cmd.PermanentError = `pre-init checks failed`

	got, err := RunWithRetries(worker, cmd, 5*time.Second)
	if err == nil {
		t.Fatal("expected an error for a permanently failing command")
	}

	if worker.runs != 1 {
		t.Fatalf("expected the command to run once, got %d", worker.runs)
	}

	if !strings.Contains(string(got), "pre-init checks failed") {
		t.Fatalf("expected the failed command's output to be returned, got: %q", got)
	}
}

func TestRunWithRetriesRetriesOtherErrors(t *testing.T) {
	worker := &countingWorker{MockSystem: NewMockSystem()}
	worker.MockCommandReturn("k8s bootstrap", []byte("context deadline exceeded"), fmt.Errorf("exit status 1"))

	cmd := NewCommand("k8s", []string{"bootstrap"})
	cmd.PermanentError = `pre-init checks failed`

	_, err := RunWithRetries(worker, cmd, 2*time.Second)
	if err == nil {
		t.Fatal("expected an error for a failing command")
	}

	if worker.runs < 2 {
		t.Fatalf("expected the command to be retried, got %d run(s)", worker.runs)
	}
}

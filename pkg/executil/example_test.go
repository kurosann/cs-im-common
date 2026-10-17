package executil_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"vane-common/pkg/executil"

	"github.com/stretchr/testify/assert"
)

func TestExample(t *testing.T) {
	// 1. 基本使用
	result, err := executil.RunCommand("ls", "-l")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	fmt.Printf("Output: %s\n", result.Stdout)

	// 3. 使用流式输出
	cmd := executil.New("ping", "localhost")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ch, pid, err := cmd.Stream(ctx)
	if err != nil {
		fmt.Printf("Stream error: %v\n", err)
	}
	t.Logf("Process PID: %d", pid)

	for line := range ch {
		fmt.Printf("Line: %s\n", line)
	}
}

func TestError(t *testing.T) {
	result, err := executil.RunCommand("sadasd", "-l", "nonexistent")
	assert.Error(t, err)
	assert.Equal(t, 127, result.ExitCode)
	assert.Contains(t, err.Error(), "executable file not found")
	t.Logf("Error: %v", err)
	t.Logf("Stderr: %s", result.Stderr)
}

func TestStreamOutput(t *testing.T) {
	cmd := executil.New("echo", "test output")
	ctx := context.Background()

	ch, pid, err := cmd.Stream(ctx)
	if !assert.NoError(t, err) {
		return
	}
	assert.Greater(t, pid, 0)

	var received []string
	for line := range ch {
		received = append(received, line)
	}

	assert.NotEmpty(t, received)
	assert.Contains(t, received[0], "test output")
	t.Logf("Received lines: %v", received)
}

func TestKillAfterWait(t *testing.T) {
	cmd := executil.New("sleep", "10")
	ctx := context.Background()

	ch, pid, err := cmd.Stream(ctx)
	if !assert.NoError(t, err) {
		return
	}
	assert.Greater(t, pid, 0)
	go func() {
		for line := range ch {
			t.Logf("Line: %s", line)
		}
	}()

	cmd.Kill()
	code := cmd.GetState()
	if code != -1 {
		cmd.Wait()
	}
}

package executil

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"vane-common/pkg/safe"
)

// Command 表示一个要执行的命令
type Command struct {
	cancel      context.CancelFunc
	Name        string
	Args        []string
	Dir         string
	Env         []string
	In          io.Reader
	cmd         *exec.Cmd
	SysProcAttr *syscall.SysProcAttr
}

// Result 表示命令执行的结果
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Error    error
	Pid      int
}

// New 创建一个新的Command实例
func New(name string, args ...string) *Command {
	return &Command{
		Name: name,
		Args: args,
	}
}

// Run 执行命令并返回完整结果
func (c *Command) Run(ctx context.Context) (*Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.cmd = exec.CommandContext(ctx, c.Name, c.Args...)
	if c.Dir != "" {
		c.cmd.Dir = c.Dir
	}
	if len(c.Env) > 0 {
		c.cmd.Env = c.Env
	}
	if c.In != nil {
		c.cmd.Stdin = c.In
	}
	if c.SysProcAttr != nil {
		c.cmd.SysProcAttr = c.SysProcAttr
	}

	var stdout, stderr bytes.Buffer
	c.cmd.Stdout = &stdout
	c.cmd.Stderr = &stderr

	err := c.cmd.Run()
	result := &Result{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}
	if c.cmd.Process != nil {
		result.Pid = c.cmd.Process.Pid
	}

	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			result.ExitCode = exitError.ExitCode()
		}
		result.Error = err
		// 如果 stderr 为空，使用 error 信息
		if result.Stderr == "" {
			result.Stderr = err.Error()
		}
		return result, fmt.Errorf("command '%s' failed: %w (stderr: %s)", c.Name, err, result.Stderr)
	}

	return result, nil
}

// Stream 执行命令并实时处理输出
func (c *Command) Stream(ctx context.Context) (<-chan string, int, error) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.cmd = exec.CommandContext(ctx, c.Name, c.Args...)
	if c.Dir != "" {
		c.cmd.Dir = c.Dir
	}
	if len(c.Env) > 0 {
		c.cmd.Env = c.Env
	}

	if c.In != nil {
		c.cmd.Stdin = c.In
	}
	if c.SysProcAttr != nil {
		c.cmd.SysProcAttr = c.SysProcAttr
	}

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return nil, 0, err
	}

	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return nil, 0, err
	}

	if err := c.cmd.Start(); err != nil {
		return nil, 0, err
	}

	ch := make(chan string)

	// 使用 WaitGroup 确保所有输出都被处理

	wg := safe.NewWaitGroup()
	wg.Go(func() { StreamOutput(stdout, ch) })
	wg.Go(func() { StreamOutput(stderr, ch) })

	// 监控命令执行和context
	safe.Go(func() {
		// 等待命令结束
		c.cmd.Wait()
		// 等待所有输出处理完成
		wg.Wait()
		// 关闭输出通道
		close(ch)
	})

	return ch, c.cmd.Process.Pid, nil
}
func (c *Command) StreamWithPipe(ctx context.Context) (io.ReadCloser, int, error) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.cmd = exec.CommandContext(ctx, c.Name, c.Args...)
	if c.Dir != "" {
		c.cmd.Dir = c.Dir
	}
	if len(c.Env) > 0 {
		c.cmd.Env = c.Env
	}
	if c.In != nil {
		c.cmd.Stdin = c.In
	}
	if c.SysProcAttr != nil {
		c.cmd.SysProcAttr = c.SysProcAttr
	}

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return nil, 0, err
	}

	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return nil, 0, err
	}
	if err := c.cmd.Start(); err != nil {
		return nil, 0, err
	}

	return newMultiReaderCloser(stdout, stderr), c.cmd.Process.Pid, nil
}

var _ io.ReadCloser = (*multiReaderCloser)(nil)

type multiReaderCloser struct {
	closers []io.Closer
	multi   io.Reader
}

func newMultiReaderCloser(readers ...io.ReadCloser) *multiReaderCloser {
	readers2 := make([]io.Reader, 0, len(readers))
	closers := make([]io.Closer, 0, len(readers))
	for _, reader := range readers {
		if reader != nil {
			readers2 = append(readers2, reader)
			closers = append(closers, reader)
		}
	}
	return &multiReaderCloser{closers: closers, multi: io.MultiReader(readers2...)}
}

func (c *multiReaderCloser) Read(p []byte) (n int, err error) {
	return c.multi.Read(p)
}
func (c *multiReaderCloser) Close() error {
	errs := make([]error, 0, len(c.closers))
	for _, closer := range c.closers {
		if err := closer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
func (c *Command) StreamWithOutErrPipe(ctx context.Context) (io.ReadCloser, io.ReadCloser, int, error) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.cmd = exec.CommandContext(ctx, c.Name, c.Args...)
	if c.Dir != "" {
		c.cmd.Dir = c.Dir
	}
	if len(c.Env) > 0 {
		c.cmd.Env = c.Env
	}
	if c.In != nil {
		c.cmd.Stdin = c.In
	}
	if c.SysProcAttr != nil {
		c.cmd.SysProcAttr = c.SysProcAttr
	}

	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return nil, nil, 0, err
	}

	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return nil, nil, 0, err
	}
	if err := c.cmd.Start(); err != nil {
		return nil, nil, 0, err
	}

	return stdout, stderr, c.cmd.Process.Pid, nil
}

// StreamOutput 处理流式输出
func StreamOutput(r io.Reader, ch chan string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		select {
		case ch <- line:
		default:
		}
	}
}

// RunCommand 快速执行命令的便捷方法
func RunCommand(name string, args ...string) (*Result, error) {
	return New(name, args...).Run(context.Background())
}

func (c *Command) Kill() {
	if c.cancel != nil {
		c.cancel()
		if c.cmd != nil && c.cmd.Process != nil {
			c.cmd.Process.Signal(os.Interrupt)
		}
	}
}
func (c *Command) SetWaitDelay(delay time.Duration) {
	c.cmd.WaitDelay = delay
}
func (c *Command) GetState() int {
	if c.cmd.ProcessState == nil {
		return -1
	}
	return c.cmd.ProcessState.ExitCode()
}

func (c *Command) Wait() error {
	if c.cmd == nil {
		return nil
	}
	return c.cmd.Wait()
}

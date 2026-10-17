package cmd

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
	"vane-common/server"

	"github.com/stretchr/testify/assert"
)

func TestNewServerCmd(t *testing.T) {
	mainServer := server.NewServer("test")
	cmd := NewServerCmd(mainServer)
	assert.NotNil(t, cmd)
	assert.Equal(t, mainServer, cmd.mainServer)
}

func TestWithReload(t *testing.T) {
	cmd := NewServerCmd(server.NewServer("test"))
	reloadFn := func() error { return nil }
	cmd = cmd.WithReload(reloadFn)
	assert.NotNil(t, cmd.reload)
}

func TestWithInit(t *testing.T) {
	cmd := NewServerCmd(server.NewServer("test"))
	initFn := func(configPath string) error { return nil }
	cmd = cmd.WithInit(initFn)
	assert.NotNil(t, cmd.init)
}

func TestBuild(t *testing.T) {
	cmd := NewServerCmd(server.NewServer("test"))
	cmd = cmd.WithInit(func(configPath string) error { return nil })

	builtCmd := cmd.Build()
	assert.NotNil(t, builtCmd)
	assert.Equal(t, "start", builtCmd.Use)
	assert.Equal(t, "start the server", builtCmd.Short)
}

func TestStart(t *testing.T) {
	t.Run("successful start", func(t *testing.T) {
		// 保存原始参数
		oldArgs := os.Args
		defer func() { os.Args = oldArgs }()

		// 设置测试参数
		os.Args = []string{"test", "start", "--config", "config.yaml"}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cmd := NewServerCmd(server.NewServer("test"))
		cmd = cmd.WithContext(ctx)

		cmd = cmd.WithInit(func(configPath string) error { return nil })

		cmd.Start()
	})

	t.Run("failed start", func(t *testing.T) {
		// 保存原始参数
		oldArgs := os.Args
		defer func() { os.Args = oldArgs }()

		// 设置测试参数
		os.Args = []string{"test", "start", "--config", "invalid_path"}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		cmd := NewServerCmd(server.NewServer("test"))
		cmd = cmd.WithContext(ctx)
		cmd = cmd.WithInit(func(configPath string) error { return errors.New("init error") })
		cmd.Start()
	})
}

func TestReload(t *testing.T) {
	t.Run("successful reload", func(t *testing.T) {
		cmd := NewServerCmd(server.NewServer("test"))
		cmd = cmd.WithReload(func() error { return nil })

		err := cmd.Reload()
		assert.NoError(t, err)
	})

	t.Run("failed reload", func(t *testing.T) {
		cmd := NewServerCmd(server.NewServer("test"))
		cmd = cmd.WithReload(func() error { return errors.New("reload error") })

		err := cmd.Reload()
		assert.Error(t, err)
	})
}

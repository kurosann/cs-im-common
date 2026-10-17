package node

import (
	"context"
	"errors"
	"os"
	"strings"
)

type IContainerEnv interface {
	GetContainerID(ctx context.Context) (string, error)
	GetContainerHostName() (string, error)
}

var _ IContainerEnv = (*DefaultContainerEnv)(nil)

type DefaultContainerEnv struct{}

func (d DefaultContainerEnv) GetContainerID(ctx context.Context) (string, error) {
	return d.GetContainerHostName()
}

func (d DefaultContainerEnv) GetContainerHostName() (string, error) {
	// 执行 hostname 命令
	hostname, err := os.Hostname()
	if err != nil {
		return "", err
	}
	// logger.Debugf(context.Background(), "[DefaultContainerEnv] GetContainerHostName %s", hostname)

	// 获取容器ID并去除空白字符
	hostname = strings.TrimSpace(hostname)
	if hostname == "" {
		return "", errors.New("empty hostname")
	}

	return hostname, nil
}

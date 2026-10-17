package node

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"vane-common/pkg/logger"
)

var _dockerSyncOnce sync.Once
var isInDocker bool

func IsInDocker() bool {
	_dockerSyncOnce.Do(func() {
		isInDocker = CheckInDocker()
		logger.Infof(context.Background(), "IsInDocker: %v", isInDocker)
	})
	return isInDocker
}

func CheckInDocker() bool {
	file, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return false
	}

	content, err := io.ReadAll(file)
	if err != nil {
		return false
	}

	isInDocker := strings.Contains(string(content), "docker")

	if err = file.Close(); err != nil {
		logger.Errorf(context.Background(), "[IsInDocker] close file error: %v", err)
	}

	return isInDocker
}

var _ IContainerEnv = (*DockerEvn)(nil)

type DockerEvn struct {
	DefaultContainerEnv
}

func NewDockerEvn() *DockerEvn {
	return &DockerEvn{}
}

func (e DockerEvn) GetContainerID(ctx context.Context) (string, error) {
	file, err := os.Open("/proc/self/cgroup")
	if err != nil {
		hostname, err := os.Hostname()
		if err != nil {
			return "", err
		}
		return hostname, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// 查找包含 docker
		if strings.Contains(line, "docker") {
			parts := strings.Split(line, "/")
			if len(parts) > 0 {
				// 获取最后一部分，通常是容器 ID
				containerID := parts[len(parts)-1]
				// 如果容器 ID 包含短 ID，只取前 12 位
				if len(containerID) > 12 {
					containerID = containerID[:12]
				}
				return containerID, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("读取 cgroup 文件失败: %v", err)
	}

	return "", fmt.Errorf("未找到容器 ID")
}

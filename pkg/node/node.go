package node

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"

	"vane-common/pkg/network"
)

type ContainerNode struct {
	// 容器环境
	env IContainerEnv

	// cache
	ContainerID       string
	ContainerHostname string
	ContainerIp       []string
	ContainerVip      string
	ContainerPort     int
	ServiceName       string
}

var (
	node *ContainerNode
	once sync.Once
)

func NewContainerNode(svcname string) *ContainerNode {
	once.Do(func() {
		node = &ContainerNode{
			ServiceName: svcname,
		}

		if IsInDocker() {
			node.env = NewDockerEvn()
		} else if IsInKubernetes() {
			node.env = NewKubernetesEnv()
		} else {
			node.env = DefaultContainerEnv{}
		}
	})
	return node
}

func (s *ContainerNode) GetContainerID(ctx context.Context) (string, error) {
	if s.ContainerID != "" {
		return s.ContainerID, nil
	}

	containerId, err := s.env.GetContainerID(ctx)
	if err != nil {
		return "", err
	}
	s.ContainerID = containerId

	return containerId, nil
}

func (s *ContainerNode) GetContainerHostName() (string, error) {
	if s.ContainerHostname != "" {
		return s.ContainerHostname, nil
	}

	hostname, err := s.env.GetContainerHostName()
	if err != nil {
		return "", err
	}
	s.ContainerHostname = hostname

	return hostname, nil
}

func (s *ContainerNode) GetContainerIp(ctx context.Context) ([]string, error) {
	if len(s.ContainerIp) > 0 {
		return s.ContainerIp, nil
	}

	ip, err := network.GetSelfIP(ctx)
	if err != nil {
		return nil, err
	}

	s.ContainerIp = []string{ip}
	return s.ContainerIp, nil
}

func (s *ContainerNode) GetContainerVip(ctx context.Context) (string, error) {
	if s.ContainerVip != "" {
		return s.ContainerVip, nil
	}

	// 从环境变量中获取
	vip := os.Getenv(strings.ReplaceAll(strings.ToUpper(s.ServiceName), "-", "_") + "_SVC_SERVICE_HOST")
	if vip == "" {
		return "", errors.New("service vip is not set")
	}

	s.ContainerVip = vip
	return s.ContainerVip, nil
}

func (s *ContainerNode) GetContainerPort(ctx context.Context) (int, error) {
	if s.ContainerPort != 0 {
		return s.ContainerPort, nil
	}

	// 从环境变量中获取
	port := os.Getenv(strings.ReplaceAll(strings.ToUpper(s.ServiceName), "-", "_") + "_SVC_SERVICE_PORT")
	if port == "" {
		return 0, errors.New("service port is not set")
	}

	portInt, err := strconv.Atoi(port)
	if err != nil {
		return 0, err
	}
	s.ContainerPort = portInt
	return s.ContainerPort, nil
}

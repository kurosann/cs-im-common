package node

import (
	"context"
	"net"
	"os"
	"sync"
	"time"

	"vane-common/pkg/logger"
)

var (
	_kubeSyncOnce  sync.Once
	isInKubernetes bool
)

func IsInKubernetes() bool {
	_kubeSyncOnce.Do(func() {
		isInKubernetes = CheckInKubernetes()
		logger.Infof(context.Background(), "isInKubernetes : %v", isInKubernetes)
	})
	return isInKubernetes
}

func CheckInKubernetes() bool {
	// 方法1：检查环境变量
	if _, exists := os.LookupEnv("KUBERNETES_SERVICE_HOST"); exists {
		return true
	}

	// 方法2：检查 Service Account 文件
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); !os.IsNotExist(err) {
		return true
	}

	// 方法3：尝试连接 API Server
	conn, err := net.DialTimeout("tcp", "kubernetes.default.svc:443", 1*time.Second)
	if err == nil {
		conn.Close()
		return true
	}

	return false
}

func KubeNamespace() (string, error) {
	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func KubeNamespaceSafe() string {
	namespace, err := KubeNamespace()
	if err != nil {
		return ""
	}
	logger.Debugf(context.Background(), "[KubernetesControllerService] KubeNamespaceSafe: %s", namespace)
	return namespace
}

var _ IContainerEnv = (*KubernetesEnv)(nil)

type KubernetesEnv struct {
	DefaultContainerEnv
}

func NewKubernetesEnv() *KubernetesEnv {
	return &KubernetesEnv{}
}

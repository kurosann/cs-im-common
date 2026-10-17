package node

import (
	"os"
	"testing"
)

func TestIsInKubernetes(t *testing.T) {
	t.Log(IsInKubernetes())

	os.Setenv("KUBERNETES_SERVICE_HOST", "127.0.0.1")
	t.Log(IsInKubernetes())
}

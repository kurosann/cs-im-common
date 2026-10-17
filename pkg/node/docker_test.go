package node

import (
	"context"
	"testing"
)

func TestIsInDocker(t *testing.T) {
	t.Logf("IsInDocker:%v", IsInDocker())
}

func TestDockerEvn_GetContainerID(t *testing.T) {
	id, err := NewDockerEvn().GetContainerID(context.Background())
	if err != nil {
		t.Error(err)
	}
	t.Logf("GetContainerID:%v", id)
}

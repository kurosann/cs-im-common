package node

import (
	"context"
	"testing"
)

func TestNewContainerNode(t *testing.T) {
	node := NewContainerNode("test")

	containerId, err := node.GetContainerID(context.Background())
	if err != nil {
		t.Errorf("GetContainerID error: %v", err)
		return
	}
	t.Logf("ContainerId: %v", containerId)

	containerIp, err := node.GetContainerIp(context.Background())
	if err != nil {
		t.Errorf("GetContainerIp error: %v", err)
		return
	}
	t.Logf("containerIp: %v", containerIp)

	name, err := node.GetContainerHostName()
	if err != nil {
		t.Errorf("GetContainerHostName error: %v", err)
		return
	}
	t.Logf("name: %v", name)

}

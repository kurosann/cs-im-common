package node

import (
	"testing"
)

func TestDefaultContainerEnv_GetContainerHostName(t *testing.T) {
	env := DefaultContainerEnv{}
	name, err := env.GetContainerHostName()
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("container hostname:%s", name)
}

func TestDefaultContainerEnv_GetContainerID(t *testing.T) {
	env := DefaultContainerEnv{}
	id, err := env.GetContainerID(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("container id:%s", id)
}

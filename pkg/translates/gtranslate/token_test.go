package gtranslate

import (
	"strings"
	"testing"
)

func TestGetSM(t *testing.T) {
	tk := sM("hello", "0")
	if tk == "" {
		t.Error("tk is empty")
	}
	if tk == "&tk=" {
		t.Error("tk has no value")
	}
}

func TestSMCalculation(t *testing.T) {
	tk := sM("test", "435102.0")
	if !strings.HasPrefix(tk, "&tk=") {
		t.Errorf("expected tk to start with '&tk=', got %s", tk)
	}
	parts := strings.Split(strings.TrimPrefix(tk, "&tk="), ".")
	if len(parts) != 2 {
		t.Errorf("expected tk format 'num.num', got %s", tk)
	}
}

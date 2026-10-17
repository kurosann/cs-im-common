package errs

import "testing"

func TestGetErrMsgByCode(t *testing.T) {

	errs := ParseErr("code =Internal desc = err: 10003")
	t.Log(errs)
}

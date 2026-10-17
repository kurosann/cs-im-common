package ift

import "reflect"

func Default[T any](t, def T) T {
	if reflect.ValueOf(t).IsZero() {
		return def
	}
	return t
}

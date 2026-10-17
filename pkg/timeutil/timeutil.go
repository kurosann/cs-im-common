package timeutil

import (
	"reflect"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func Time2PbTimestamp(t time.Time) *timestamppb.Timestamp {
	return timestamppb.New(t)
}

func PbTimestamp2Time(t *timestamppb.Timestamp) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.AsTime()
}
func PTime2PbTimestamp(t *time.Time) *timestamppb.Timestamp {
	if t == nil {
		return nil
	}
	return timestamppb.New(*t)
}

func PbTimestamp2PTime(t *timestamppb.Timestamp) *time.Time {
	if t == nil {
		return nil
	}
	time := t.AsTime()
	return &time
}

func ATime2PbTimestamp[T time.Time | *time.Time](t T) *timestamppb.Timestamp {
	ref := reflect.ValueOf(t)
	if ref.IsZero() {
		return nil
	}
	if ref.Kind() == reflect.Ptr {
		return PTime2PbTimestamp(ref.Interface().(*time.Time))
	}
	return Time2PbTimestamp(ref.Interface().(time.Time))
}

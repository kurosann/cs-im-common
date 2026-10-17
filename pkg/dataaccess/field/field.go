package field

import "fmt"

type IID interface {
	IsNil() bool
	String() string
	Val() any
}
type id struct {
	ID any
}
type idType interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~string | ~uint
}

// xxxByID 使用此处的ID
func ID[T idType](t T) IID {
	return id{ID: t}
}
func (d id) IsNil() bool {
	return d.ID == nil
}
func (d id) String() string {
	return fmt.Sprintf("%v", d.ID)
}
func (d id) Val() any {
	return d.ID
}

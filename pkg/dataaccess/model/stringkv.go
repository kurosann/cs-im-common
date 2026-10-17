package model

import (
	"fmt"
	"strings"
	"time"
	"vane-common/pkg/dataaccess/field"
)

var _ Table = (*StringKV)(nil)
var _ TableMarshaler = (*StringKV)(nil)

type StringKV struct {
	DS      DbSource
	Key     string `json:"key"`
	Content string `json:"content"`
}

func NewStringKV(ds DbSource, key, content string) *StringKV {
	return &StringKV{
		DS:      ds,
		Key:     key,
		Content: content,
	}
}
func (s *StringKV) String() string {
	return s.Content
}

func (s *StringKV) Marshal() ([]byte, error) {
	return []byte(fmt.Appendf([]byte{}, "%s:%s", s.Key, s.Content)), nil
}

func (s *StringKV) Unmarshal(data []byte) error {
	parts := strings.SplitN(string(data), ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid string format")
	}

	s.Key = parts[0]
	s.Content = parts[1]
	return nil
}

func (s *StringKV) UnmarshalMap(data []byte, m *map[string]any) error {
	parts := strings.SplitN(string(data), ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid string format")
	}

	(*m)["key"] = parts[0]
	(*m)["content"] = parts[1]
	return nil
}

func (s *StringKV) Create() {
}

func (s *StringKV) DbSource() DbSource {
	return s.DS
}

func (s *StringKV) DbType() DBType {
	return DBTypeKv
}

func (s *StringKV) GetExpireAt() *time.Time {
	return nil
}

func (s *StringKV) GetOwner() string {
	return ""
}

func (s *StringKV) GetRevision() int64 {
	return 0
}

func (s *StringKV) SetExpireAt(*time.Time) {

}

func (s *StringKV) Update() {
}

func (s *StringKV) TableName() string {
	return fmt.Sprintf("string_kv:%s", s.Key)
}

func (s *StringKV) GetID() field.IID {
	return field.ID(s.Key)
}

func (s *StringKV) SetID(id field.IID) {
	s.Key = id.String()
}
func (s *StringKV) IDColumn() string {
	return "key"
}

func (s *StringKV) HasExpireAt() bool {
	return false
}

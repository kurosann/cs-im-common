// Package jsontype provides utilities for determining and converting JSON strings
// to appropriate Go data structures.
package jsontype

import (
	"encoding/json"
	"strings"
)

// Type represents the JSON data type
type Type int

const (
	// Unknown represents an unknown or invalid JSON type
	Unknown Type = iota
	// Object represents a JSON object {}
	Object
	// Array represents a JSON array []
	Array
)

// String returns the string representation of the Type
func (t Type) String() string {
	switch t {
	case Object:
		return "Object"
	case Array:
		return "Array"
	default:
		return "Unknown"
	}
}

// Determine checks if a string is a JSON object or array
func Determine(jsonStr string) Type {
	// Trim whitespace
	jsonStr = strings.TrimSpace(jsonStr)

	if len(jsonStr) == 0 {
		return Unknown
	}

	// Check the first non-whitespace character
	switch jsonStr[0] {
	case '{':
		return Object
	case '[':
		return Array
	default:
		return Unknown
	}
}

// ToObject converts a JSON object string to map[string]interface{}
func ToObject(jsonStr string) (map[string]interface{}, error) {
	var obj map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &obj)
	if err != nil {
		return nil, err
	}
	return obj, nil
}

// UnmarshalToObject unmarshals a JSON object string into a provided object pointer
func UnmarshalToObject(jsonStr string, v interface{}) error {
	return json.Unmarshal([]byte(jsonStr), v)
}

// ToArray converts a JSON array string to []interface{}
func ToArray(jsonStr string) ([]interface{}, error) {
	var arr []interface{}
	err := json.Unmarshal([]byte(jsonStr), &arr)
	if err != nil {
		return nil, err
	}
	return arr, nil
}

// UnmarshalToArray unmarshals a JSON array string into a provided array pointer
func UnmarshalToArray(jsonStr string, v interface{}) error {
	return json.Unmarshal([]byte(jsonStr), v)
}

// Convert automatically determines the JSON type and converts it to the appropriate Go type
func Convert(jsonStr string) (interface{}, Type, error) {
	jsonType := Determine(jsonStr)

	switch jsonType {
	case Object:
		obj, err := ToObject(jsonStr)
		return obj, jsonType, err
	case Array:
		arr, err := ToArray(jsonStr)
		return arr, jsonType, err
	default:
		return nil, Unknown, &json.SyntaxError{}
	}
}

package jsontype

import (
	"reflect"
	"testing"
)

func TestDetermine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Type
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: Unknown,
		},
		{
			name:     "Object",
			input:    `{"key": "value"}`,
			expected: Object,
		},
		{
			name:     "Array",
			input:    `["item1", "item2"]`,
			expected: Array,
		},
		{
			name:     "Whitespace object",
			input:    `  {"key": "value"}  `,
			expected: Object,
		},
		{
			name:     "Whitespace array",
			input:    `  ["item1", "item2"]  `,
			expected: Array,
		},
		{
			name:     "Invalid JSON",
			input:    `invalid`,
			expected: Unknown,
		},
		{
			name:     "Number",
			input:    `123`,
			expected: Unknown,
		},
		{
			name:     "String",
			input:    `"string"`,
			expected: Unknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := Determine(tt.input)
			if actual != tt.expected {
				t.Errorf("Determine(%q) = %v, want %v", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestToObject(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    map[string]interface{}
		expectError bool
	}{
		{
			name:  "Valid object",
			input: `{"name": "John", "age": 30}`,
			expected: map[string]interface{}{
				"name": "John",
				"age":  float64(30),
			},
			expectError: false,
		},
		{
			name:        "Invalid object",
			input:       `["not", "an", "object"]`,
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ToObject(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("ToObject(%q) error = %v, expectError %v", tt.input, err, tt.expectError)
				return
			}
			if !tt.expectError && !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("ToObject(%q) = %v, want %v", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestToArray(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    []interface{}
		expectError bool
	}{
		{
			name:  "Valid array",
			input: `["item1", "item2", 123]`,
			expected: []interface{}{
				"item1",
				"item2",
				float64(123),
			},
			expectError: false,
		},
		{
			name:        "Invalid array",
			input:       `{"not": "an", "array": "object"}`,
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ToArray(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("ToArray(%q) error = %v, expectError %v", tt.input, err, tt.expectError)
				return
			}
			if !tt.expectError && !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("ToArray(%q) = %v, want %v", tt.input, actual, tt.expected)
			}
		})
	}
}

func TestConvert(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedData interface{}
		expectedType Type
		expectError  bool
	}{
		{
			name:  "Valid object",
			input: `{"name": "John", "age": 30}`,
			expectedData: map[string]interface{}{
				"name": "John",
				"age":  float64(30),
			},
			expectedType: Object,
			expectError:  false,
		},
		{
			name:  "Valid array",
			input: `[1, 2, 3]`,
			expectedData: []interface{}{
				float64(1),
				float64(2),
				float64(3),
			},
			expectedType: Array,
			expectError:  false,
		},
		{
			name:         "Invalid JSON",
			input:        `invalid`,
			expectedData: nil,
			expectedType: Unknown,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualData, actualType, err := Convert(tt.input)
			if (err != nil) != tt.expectError {
				t.Errorf("Convert(%q) error = %v, expectError %v", tt.input, err, tt.expectError)
				return
			}
			if actualType != tt.expectedType {
				t.Errorf("Convert(%q) type = %v, want %v", tt.input, actualType, tt.expectedType)
			}
			if !tt.expectError && !reflect.DeepEqual(actualData, tt.expectedData) {
				t.Errorf("Convert(%q) data = %v, want %v", tt.input, actualData, tt.expectedData)
			}
		})
	}
}

func TestUnmarshalToObject(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name        string
		jsonStr     string
		expectError bool
	}{
		{
			name:        "Valid object",
			jsonStr:     `{"name": "John", "age": 30}`,
			expectError: false,
		},
		{
			name:        "Invalid object",
			jsonStr:     `["not", "an", "object"]`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var person Person
			err := UnmarshalToObject(tt.jsonStr, &person)

			if (err != nil) != tt.expectError {
				t.Errorf("UnmarshalToObject(%q) error = %v, expectError %v", tt.jsonStr, err, tt.expectError)
				return
			}

			if !tt.expectError {
				if person.Name != "John" || person.Age != 30 {
					t.Errorf("UnmarshalToObject(%q) = %+v, want name=John, age=30", tt.jsonStr, person)
				}
			}
		})
	}
}

func TestUnmarshalToArray(t *testing.T) {
	type Person struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	tests := []struct {
		name        string
		jsonStr     string
		expectError bool
	}{
		{
			name:        "Valid array",
			jsonStr:     `[{"name": "John", "age": 30}, {"name": "Jane", "age": 25}]`,
			expectError: false,
		},
		{
			name:        "Invalid array",
			jsonStr:     `{"not": "an", "array": "object"}`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var people []Person
			err := UnmarshalToArray(tt.jsonStr, &people)

			if (err != nil) != tt.expectError {
				t.Errorf("UnmarshalToArray(%q) error = %v, expectError %v", tt.jsonStr, err, tt.expectError)
				return
			}

			if !tt.expectError {
				if len(people) != 2 || people[0].Name != "John" || people[1].Age != 25 {
					t.Errorf("UnmarshalToArray(%q) = %+v, want array with 2 people", tt.jsonStr, people)
				}
			}
		})
	}
}

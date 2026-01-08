package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Value
	}{
		// Simple strings
		{"simple string", "+OK\r\n", Value{Type: SimpleString, Str: "OK"}},
		{"simple string empty", "+\r\n", Value{Type: SimpleString, Str: ""}},
		{"simple string with spaces", "+hello world\r\n", Value{Type: SimpleString, Str: "hello world"}},

		// Simple errors
		{"simple error", "-ERR unknown command\r\n", Value{Type: SimpleError, Str: "ERR unknown command"}},
		{"simple error short", "-ERR\r\n", Value{Type: SimpleError, Str: "ERR"}},

		// Integers
		{"integer positive", ":1000\r\n", Value{Type: Integer, Integer: 1000}},
		{"integer negative", ":-42\r\n", Value{Type: Integer, Integer: -42}},
		{"integer zero", ":0\r\n", Value{Type: Integer, Integer: 0}},

		// Bulk strings
		{"bulk string", "$5\r\nhello\r\n", Value{Type: BulkString, Str: "hello"}},
		{"bulk string empty", "$0\r\n\r\n", Value{Type: BulkString, Str: ""}},
		{"bulk string null", "$-1\r\n", Value{Type: BulkString, Null: true}},
		{"bulk string with crlf", "$7\r\nhi\r\nbye\r\n", Value{Type: BulkString, Str: "hi\r\nbye"}},
		{"bulk string with spaces", "$11\r\nhello world\r\n", Value{Type: BulkString, Str: "hello world"}},

		// Null
		{"null", "_\r\n", Value{Type: Null, Null: true}},

		// Arrays
		{"array empty", "*0\r\n", Value{Type: Array, Array: []Value{}}},
		{"array null", "*-1\r\n", Value{Type: Array, Null: true}},
		{"array single", "*1\r\n+OK\r\n", Value{Type: Array, Array: []Value{
			{Type: SimpleString, Str: "OK"},
		}}},
		{"array two strings", "*2\r\n+foo\r\n+bar\r\n", Value{Type: Array, Array: []Value{
			{Type: SimpleString, Str: "foo"},
			{Type: SimpleString, Str: "bar"},
		}}},
		{"array mixed types", "*3\r\n+hello\r\n:42\r\n$5\r\nworld\r\n", Value{Type: Array, Array: []Value{
			{Type: SimpleString, Str: "hello"},
			{Type: Integer, Integer: 42},
			{Type: BulkString, Str: "world"},
		}}},
		{"array nested", "*2\r\n*2\r\n:1\r\n:2\r\n*2\r\n:3\r\n:4\r\n", Value{Type: Array, Array: []Value{
			{Type: Array, Array: []Value{
				{Type: Integer, Integer: 1},
				{Type: Integer, Integer: 2},
			}},
			{Type: Array, Array: []Value{
				{Type: Integer, Integer: 3},
				{Type: Integer, Integer: 4},
			}},
		}}},

		// Real Redis commands (what redis-cli sends)
		{"ping command", "*1\r\n$4\r\nPING\r\n", Value{Type: Array, Array: []Value{
			{Type: BulkString, Str: "PING"},
		}}},
		{"set command", "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n", Value{Type: Array, Array: []Value{
			{Type: BulkString, Str: "SET"},
			{Type: BulkString, Str: "foo"},
			{Type: BulkString, Str: "bar"},
		}}},
		{"get command", "*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n", Value{Type: Array, Array: []Value{
			{Type: BulkString, Str: "GET"},
			{Type: BulkString, Str: "foo"},
		}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewReader(strings.NewReader(tt.input))
			got, err := reader.ReadValue()

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReadValueErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unknown type", "X\r\n"},
		{"invalid integer", ":abc\r\n"},
		{"invalid bulk length", "$abc\r\n"},
		{"invalid array length", "*abc\r\n"},
		{"incomplete bulk string", "$5\r\nhel"},
		{"empty input", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := NewReader(strings.NewReader(tt.input))
			_, err := reader.ReadValue()

			assert.Error(t, err, "expected error for input: %q", tt.input)
		})
	}
}

func TestWriteValue(t *testing.T) {
	tests := []struct {
		name  string
		value Value
		want  string
	}{
		// Simple strings
		{"simple string", Value{Type: SimpleString, Str: "OK"}, "+OK\r\n"},
		{"simple string empty", Value{Type: SimpleString, Str: ""}, "+\r\n"},
		{"simple string pong", Value{Type: SimpleString, Str: "PONG"}, "+PONG\r\n"},

		// Simple errors
		{"simple error", Value{Type: SimpleError, Str: "ERR unknown"}, "-ERR unknown\r\n"},

		// Integers
		{"integer positive", Value{Type: Integer, Integer: 123}, ":123\r\n"},
		{"integer negative", Value{Type: Integer, Integer: -456}, ":-456\r\n"},
		{"integer zero", Value{Type: Integer, Integer: 0}, ":0\r\n"},

		// Bulk strings
		{"bulk string", Value{Type: BulkString, Str: "hello"}, "$5\r\nhello\r\n"},
		{"bulk string empty", Value{Type: BulkString, Str: ""}, "$0\r\n\r\n"},
		{"bulk string null", Value{Type: BulkString, Null: true}, "$-1\r\n"},

		// Null
		{"null", Value{Type: Null, Null: true}, "_\r\n"},

		// Arrays
		{"array empty", Value{Type: Array, Array: []Value{}}, "*0\r\n"},
		{"array null", Value{Type: Array, Null: true}, "*-1\r\n"},
		{"array single", Value{Type: Array, Array: []Value{
			{Type: SimpleString, Str: "OK"},
		}}, "*1\r\n+OK\r\n"},
		{"array multiple", Value{Type: Array, Array: []Value{
			{Type: SimpleString, Str: "foo"},
			{Type: Integer, Integer: 42},
		}}, "*2\r\n+foo\r\n:42\r\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			writer := NewWriter(&buf)

			err := writer.WriteValue(tt.value)
			require.NoError(t, err)

			err = writer.Flush()
			require.NoError(t, err)

			assert.Equal(t, tt.want, buf.String())
		})
	}
}

func TestRoundtrip(t *testing.T) {
	tests := []struct {
		name  string
		value Value
	}{
		{"simple string", Value{Type: SimpleString, Str: "hello"}},
		{"error", Value{Type: SimpleError, Str: "ERR something"}},
		{"integer", Value{Type: Integer, Integer: 12345}},
		{"bulk string", Value{Type: BulkString, Str: "binary\r\ndata"}},
		{"null", Value{Type: Null, Null: true}},
		{"array", Value{Type: Array, Array: []Value{
			{Type: BulkString, Str: "SET"},
			{Type: BulkString, Str: "key"},
			{Type: BulkString, Str: "value"},
		}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Write
			var buf bytes.Buffer
			writer := NewWriter(&buf)
			err := writer.WriteValue(tt.value)
			require.NoError(t, err)
			err = writer.Flush()
			require.NoError(t, err)

			// Read back
			reader := NewReader(&buf)
			got, err := reader.ReadValue()
			require.NoError(t, err)

			assert.Equal(t, tt.value, got)
		})
	}
}

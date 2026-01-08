package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// from https://github.com/redis/redis-specifications/blob/master/protocol/RESP3.md
const (
	SimpleString   = '+'
	SimpleError    = '-'
	Integer        = ':'
	BulkString     = '$'
	Array          = '*'
	Null           = '_'
	Boolean        = '#'
	Double         = ','
	BigNumber      = '('
	BulkError      = '!'
	VerbatimString = '='
	Map            = '%'
	Set            = '~'
	Push           = '>'
)

// holds a single RESP3 value
// the spec is not specific here so there might be room for improvement
type Value struct {
	Type    byte    // type of the value we are holding
	Str     string  // for simple strings, errors and bulk strings
	Integer int64   // for integers
	Float   float64 // for floats
	Bool    bool    // for boolean values
	Array   []Value // for recursive values
	Null    bool    // bulk strings and arrays can be null
}

type Reader struct {
	rd *bufio.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{rd: bufio.NewReader(r)}
}

func (r *Reader) ReadValue() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}

	if len(line) == 0 {
		// invalid RESP!
		return Value{}, errors.New("invalid RESP: empty line")
	}

	// the first byte tells us the type of what's ahead
	typeByte := line[0]
	lineContent := line[1:]

	// dispatch to the appropriate handler based on type
	switch typeByte {

	case SimpleString:
		return Value{Type: SimpleString, Str: lineContent}, nil

	case SimpleError:
		return Value{Type: SimpleError, Str: lineContent}, nil

	case Integer:
		integerValue, err := strconv.ParseInt(lineContent, 10, 64)
		if err != nil {
			return Value{}, fmt.Errorf("invalid RESP: could not parse base 10 integer (%v)", err)
		}
		return Value{Type: Integer, Integer: integerValue}, nil

	case Null:
		return Value{Type: Null, Null: true}, nil
	}

	// nothing matched? invalid
	return Value{}, errors.New("invalid RESP: unknown type")
}

func (r *Reader) readLine() (string, error) {
	line, err := r.rd.ReadString('\n')
	if err != nil {
		return "", err
	}

	line = strings.TrimSuffix(line, "\r\n")
	return line, nil
}

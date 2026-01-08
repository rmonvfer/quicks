package main

import (
	"bufio"
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

type Writer struct {
	wr *bufio.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{wr: bufio.NewWriter(w)}
}

func (w *Writer) WriteValue(v Value) error {
	switch v.Type {
	case SimpleString:
		_, err := fmt.Fprintf(w.wr, "+%s\r\n", v.Str)
		return err

	case SimpleError:
		_, err := fmt.Fprintf(w.wr, "-%s\r\n", v.Str)
		return err

	case Integer:
		_, err := fmt.Fprintf(w.wr, ":%d\r\n", v.Integer)
		return err

	case BulkString:
		if v.Null {
			_, err := w.wr.WriteString("$-1\r\n")
			return err
		}
		_, err := fmt.Fprintf(w.wr, "$%d\r\n%s\r\n", len(v.Str), v.Str)
		return err

	case Array:
		if v.Null {
			_, err := w.wr.WriteString("*-1\r\n")
			return err
		}
		if _, err := fmt.Fprintf(w.wr, "*%d\r\n", len(v.Array)); err != nil {
			return err
		}
		for _, elem := range v.Array {
			if err := w.WriteValue(elem); err != nil { // recursive!
				return err
			}
		}
		return nil

	case Null:
		_, err := w.wr.WriteString("_\r\n")
		return err

	default:
		return fmt.Errorf("unknown type: %c", v.Type)
	}
}

func (w *Writer) Flush() error {
	return w.wr.Flush()
}

type Reader struct {
	rd *bufio.Reader
}

func NewReader(r io.Reader) *Reader {
	return &Reader{rd: bufio.NewReader(r)}
}

func (r *Reader) ReadValue() (Value, error) {
	typeByte, err := r.rd.ReadByte()
	if err != nil {
		return Value{}, err
	}

	// dispatch to the appropriate handler based on type
	switch typeByte {
	case SimpleString:
		return r.readSimpleString()
	case SimpleError:
		return r.readSimpleError()
	case Integer:
		return r.readInteger()
	case BulkString:
		return r.readBulkString()
	case Array:
		return r.readArray()
	case Null:
		return Value{Type: Null, Null: true}, nil
	default:
		return Value{}, fmt.Errorf("invalid RESP: unknown type (%c)", typeByte)
	}
}

func (r *Reader) readLine() (string, error) {
	line, err := r.rd.ReadString('\n')
	if err != nil {
		return "", err
	}

	line = strings.TrimSuffix(line, "\r\n")
	return line, nil
}

func (r *Reader) readSimpleString() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}
	return Value{Type: SimpleString, Str: line}, nil
}

func (r *Reader) readSimpleError() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}
	return Value{Type: SimpleError, Str: line}, nil
}

func (r *Reader) readInteger() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}
	integerValue, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return Value{}, fmt.Errorf("could not parse base 10 integer: %v", err)
	}
	return Value{Type: Integer, Integer: integerValue}, nil
}

func (r *Reader) readBulkString() (Value, error) {
	// format is $<length>\r\n<data>\r\n
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}

	length, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return Value{}, fmt.Errorf("invalid bulk string length: %v", err)
	}

	// RESP2 clients might send -1 length as null
	if length == -1 {
		return Value{Type: BulkString, Null: true}, nil
	}

	// read the rest of the string as per the provided length
	buf := make([]byte, length)
	if _, err := io.ReadFull(r.rd, buf); err != nil {
		return Value{}, err
	}

	// consume the remining line termination
	// this also ensures the line is correctly delimited
	if _, err := r.readLine(); err != nil {
		return Value{}, err
	}

	return Value{Type: BulkString, Str: string(buf)}, nil
}

func (r *Reader) readArray() (Value, error) {
	// format is *<count>\r\n<element1><element2>...<elementN>
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}

	length, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return Value{}, fmt.Errorf("invalid array length: %v", err)
	}

	// RESP2 clients might send -1 length as null
	if length == -1 {
		return Value{Type: Array, Null: true}, nil
	}

	// now each entry can be any Value
	values := make([]Value, length)
	for i := range length {
		value, err := r.ReadValue()
		if err != nil {
			return Value{}, fmt.Errorf("couldn't parse array value at position %d: %v", i, err)
		}
		values[i] = value
	}

	return Value{Type: Array, Array: values}, nil
}

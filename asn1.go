package asn1

// TODO package documentation
// TODO proper log messages
// TODO add a mechanism for extendability
// TODO proper checking of the constructed flag
// TODO support for constructed encoding and decoding of string types in BER

import (
	"fmt"
	"reflect"
)

// Internal consts
const (
	tagKey = "asn1"
)

// Encode an object using the default context and without options.
func Encode(obj interface{}) (data []byte, err error) {
	ctx := NewContext()
	return ctx.EncodeWithOptions(obj, "")
}

// Encode an object using the default context and with options.
func EncodeWithOptions(obj interface{}, options string) (data []byte, err error) {
	ctx := NewContext()
	return ctx.EncodeWithOptions(obj, options)
}

// Encode an object using the given context and without options.
func Decode(data []byte, obj interface{}) (rest []byte, err error) {
	ctx := NewContext()
	return ctx.DecodeWithOptions(data, obj, "")
}

// Encode an object using the given context and with options.
func DecodeWithOptions(data []byte, obj interface{}, options string) (rest []byte, err error) {
	ctx := NewContext()
	return ctx.DecodeWithOptions(data, obj, options)
}

// This error is caused by invalid data.
type ParseError struct {
	Msg string
}

// Return the error message.
func (e *ParseError) Error() string {
	return e.Msg
}

// Allocate a new ParseError.
func parseError(ctx *Context, msg string, args ...interface{}) *ParseError {
	return &ParseError{fmt.Sprintf(msg, args...)}
}

// This error is caused by invalid structure.
type SyntaxError struct {
	Msg string
}

// Return the error message.
func (e *SyntaxError) Error() string {
	return e.Msg
}

// Allocate a new ParseError,
func syntaxError(ctx *Context, msg string, args ...interface{}) *SyntaxError {
	return &SyntaxError{fmt.Sprintf(msg, args...)}
}

// setDefaultValue sets a reflected value to its default value based on the
// field options.
func (ctx *Context) setDefaultValue(value reflect.Value, opts *fieldOptions) error {
	switch value.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value.SetInt(int64(*opts.defaultValue))

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		value.SetUint(uint64(*opts.defaultValue))

	default:
		return syntaxError(ctx, "Default value is only allow to integers")
	}
	return nil
}

// newDefaultValue creates a new reflected value and sets it to its default value.
func (ctx *Context) newDefaultValue(objType reflect.Type, opts *fieldOptions) (reflect.Value, error) {
	value := reflect.New(objType).Elem()
	if opts.defaultValue == nil {
		return value, nil
	}
	return value, ctx.setDefaultValue(value, opts)
}

package asn1

import (
	"bytes"
	"io"
	"reflect"
	"sort"
)

// Expected values
type expectedElement struct {
	class   uint
	tag     uint
	decoder decoderFunction
}

// Expected values for fields
type expectedFieldElement struct {
	expectedElement
	value reflect.Value
	opts  *fieldOptions
}

// Decode BER or DER data without an option string.
// See (*Context) DecodeWithOptions() for further details.
func (ctx *Context) Decode(data []byte, obj interface{}) (rest []byte, err error) {
	return ctx.DecodeWithOptions(data, obj, "")
}

// Decode BER or DER data using an option string that are handled the same way
// as struct tags.
// Since the given object will be filled with the parsed data, it should be a
// reference.
func (ctx *Context) DecodeWithOptions(data []byte, obj interface{}, options string) (rest []byte, err error) {

	opts, err := parseOptions(ctx, options)
	if err != nil {
		return nil, err
	}

	value := reflect.ValueOf(obj)
	switch value.Kind() {
	case reflect.Ptr, reflect.Interface:
		value = value.Elem()
	}

	if !value.CanSet() {
		return nil, syntaxError(ctx, "Go type \"%s\" is read-only", value.Type())
	}

	reader := bytes.NewBuffer(data)
	err = ctx.decode(reader, value, opts)
	if err != nil {
		return nil, err
	}

	return reader.Bytes(), nil
}

// Main decode function
func (ctx *Context) decode(reader io.Reader, value reflect.Value, opts *fieldOptions) error {

	// Parse an Asn.1 element
	raw, err := decodeRawValue(reader)
	if err != nil {
		return err
	}
	if ctx.der.decoding && raw.Indefinite {
		return parseError(ctx, "Indefinite length form is not supported by DER mode")
	}

	elem, err := ctx.getExpectedElement(raw, value.Type(), opts)
	if err != nil {
		return err
	}

	// And tag must match
	if raw.Class != elem.class || raw.Tag != elem.tag {
		ctx.log.Printf("%#v\n", opts)
		return parseError(ctx, "Expected tag (%d,%d) but found (%d,%d)",
			elem.class, elem.tag, raw.Class, raw.Tag)
	}

	return elem.decoder(raw.Content, value)
}

// getExpectedElement returns the expected element for a given type. raw is only
// used as hint when decoding choices.
// TODO: consider replacing raw for class and tag number.
func (ctx *Context) getExpectedElement(raw *RawValue, elemType reflect.Type, opts *fieldOptions) (elem expectedElement, err error) {

	// Get the expected universal tag and its decoder for the given Go type
	elem, err = ctx.getUniversalTag(elemType, opts)
	if err != nil {
		return
	}

	// Modify the expected tag and decoder function based on the given options
	if opts.tag != nil {
		elem.class = ClassContextSpecific
		elem.tag = uint(*opts.tag)
	}
	if opts.universal {
		elem.class = ClassUniversal
	}
	if opts.application {
		elem.class = ClassApplication
	}

	if opts.explicit {
		elem.decoder = func(data []byte, value reflect.Value) error {
			// Unset previous flags
			opts.explicit = false
			opts.tag = nil
			opts.application = false
			// Parse child
			reader := bytes.NewBuffer(data)
			return ctx.decode(reader, value, opts)
		}
		return
	}

	if opts.choice != nil {
		// Get the registered choices
		var entry choiceEntry
		entry, err = ctx.getChoiceByTag(*opts.choice, raw.Class, raw.Tag)
		if err != nil {
			return
		}

		// Get the decoder for the new value
		elem.class, elem.tag = raw.Class, raw.Tag
		elem.decoder = func(data []byte, value reflect.Value) error {
			// Allocate a new value and set to the current one
			nestedValue := reflect.New(entry.typ).Elem()
			err = entry.decoder(data, nestedValue)
			if err != nil {
				return err
			}
			value.Set(nestedValue)
			return nil
		}
	}

	// At this point a decoder function already be found
	if elem.decoder == nil {
		err = parseError(ctx, "Go type not supported \"%s\"", elemType)
	}
	return
}

// getUniversalTag maps an type to a Asn.1 universal type.
func (ctx *Context) getUniversalTag(objType reflect.Type, opts *fieldOptions) (elem expectedElement, err error) {

	elem.class = ClassUniversal

	// Special types:
	switch objType {
	case bigIntType:
		elem.tag = TagInteger
		elem.decoder = ctx.decodeBigInt
	case oidType:
		elem.tag = TagOid
		elem.decoder = ctx.decodeOid
	case nullType:
		elem.tag = TagNull
		elem.decoder = ctx.decodeNull
	}

	// Generic types:
	if elem.decoder == nil {
		switch objType.Kind() {
		case reflect.Bool:
			elem.tag = TagBoolean
			elem.decoder = ctx.decodeBool

		case reflect.String:
			elem.tag = TagOctetString
			elem.decoder = ctx.decodeString

		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			elem.tag = TagInteger
			elem.decoder = ctx.decodeInt

		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			elem.tag = TagInteger
			elem.decoder = ctx.decodeUint

		case reflect.Struct:
			elem.tag = TagSequence
			elem.decoder = ctx.decodeStruct
			if opts.set {
				elem.decoder = ctx.decodeStructAsSet
			}

		case reflect.Array:
			if objType.Elem().Kind() == reflect.Uint8 {
				elem.tag = TagOctetString
				elem.decoder = ctx.decodeOctetString
			} else {
				elem.tag = TagSequence
				elem.decoder = ctx.decodeArray
			}

		case reflect.Slice:
			if objType.Elem().Kind() == reflect.Uint8 {
				elem.tag = TagOctetString
				elem.decoder = ctx.decodeOctetString
			} else {
				elem.tag = TagSequence
				elem.decoder = ctx.decodeSlice
			}
		}
	}

	// Check options for universal types
	if opts.set {
		if elem.tag != TagSequence {
			err = syntaxError(ctx,
				"Flag \"set\" can't be set to Go type \"%s\"", objType)
		}
		elem.tag = TagSet
	}
	return
}

//
func (ctx *Context) getExpectedFieldElements(value reflect.Value) ([]expectedFieldElement, error) {
	expectedValues := []expectedFieldElement{}
	for i := 0; i < value.NumField(); i++ {
		if value.CanSet() {
			// Get field and options
			field := value.Field(i)
			opts, err := parseOptions(ctx, value.Type().Field(i).Tag.Get(tagKey))
			if err != nil {
				return nil, err
			}
			// Expand choices
			raw := &RawValue{}
			if opts.choice == nil {
				elem, err := ctx.getExpectedElement(raw, field.Type(), opts)
				if err != nil {
					return nil, err
				}
				expectedValues = append(expectedValues,
					expectedFieldElement{elem, field, opts})
			} else {
				entries, err := ctx.getChoices(*opts.choice)
				if err != nil {
					return nil, err
				}
				for _, entry := range entries {
					raw.Class = entry.class
					raw.Tag = entry.tag
					elem, err := ctx.getExpectedElement(raw, field.Type(), opts)
					if err != nil {
						return nil, err
					}
					expectedValues = append(expectedValues,
						expectedFieldElement{elem, field, opts})
				}
			}
		}
	}
	return expectedValues, nil
}

// getRawValuesFromBytes reads up to max values from the byte sequence.
func (ctx *Context) getRawValuesFromBytes(data []byte, max int) ([]*RawValue, error) {
	// Raw values
	rawValues := []*RawValue{}
	reader := bytes.NewBuffer(data)
	for i := 0; i < max; i++ {
		// Parse an Asn.1 element
		raw, err := decodeRawValue(reader)
		if err != nil {
			return nil, err
		}
		rawValues = append(rawValues, raw)
		if reader.Len() == 0 {
			return rawValues, nil
		}
	}
	return nil, parseError(ctx, "Too many items for Sequence.")
}

// matchExpectedValues tries to decode a sequence of raw values based on the
// expected elements.
func (ctx *Context) matchExpectedValues(eValues []expectedFieldElement, rValues []*RawValue) error {
	// Try to match expected and raw values
	rIndex := 0
	for eIndex := 0; eIndex < len(eValues); eIndex++ {
		e := eValues[eIndex]
		// Using nil decoder to skip matched choices
		if e.decoder == nil {
			continue
		}

		missing := true
		if rIndex < len(rValues) {
			raw := rValues[rIndex]
			if e.class == raw.Class && e.tag == raw.Tag {
				err := e.decoder(raw.Content, e.value)
				if err != nil {
					return err
				}
				// Mark as found and advance raw values index
				missing = false
				rIndex += 1
				// Remove other options for the matched choice
				if e.opts.choice != nil {
					for i := eIndex + 1; i < len(eValues); i++ {
						c := eValues[i].opts.choice
						if c != nil && *c == *e.opts.choice {
							eValues[i].decoder = nil
						}
					}
				}
			}
		}

		if missing {
			if e.opts.optional || e.opts.choice != nil {
				continue
			}
			if e.opts.defaultValue != nil {
				err := ctx.setDefaultValue(e.value, e.opts)
				if err != nil {
					return err
				}
				continue
			}
			return parseError(ctx, "Missing value for [%d %d]", e.class, e.tag)
		}
	}
	return nil
}

// decodeStruct decodes struct fields in order
func (ctx *Context) decodeStruct(data []byte, value reflect.Value) error {

	expectedValues, err := ctx.getExpectedFieldElements(value)
	if err != nil {
		return err
	}

	rawValues, err := ctx.getRawValuesFromBytes(data, len(expectedValues))
	if err != nil {
		return err
	}

	return ctx.matchExpectedValues(expectedValues, rawValues)
}

// Decode a struct as an Asn.1 Set.
//
// The order doesn't matter for set. However DER dictates that a Set should be
// encoded in the ascending order of the tags. So when decoding with DER, we
// simply do not sort the raw values and use them in their natural order.
func (ctx *Context) decodeStructAsSet(data []byte, value reflect.Value) error {

	// Get the expected values
	expectedElements, err := ctx.getExpectedFieldElements(value)
	if err != nil {
		return err
	}
	sort.Sort(expectedFieldElementSlice(expectedElements))

	// Check duplicated tags
	for i := 1; i < len(expectedElements); i++ {
		curr := expectedElements[i]
		prev := expectedElements[i-1]
		if curr.class == prev.class &&
			curr.tag == prev.tag {
			return syntaxError(ctx, "Duplicated tag (%d,%d)", curr.class, curr.tag)
		}
	}

	// Get the raw values
	rawValues, err := ctx.getRawValuesFromBytes(data, len(expectedElements))
	if err != nil {
		return err
	}
	if !ctx.der.decoding {
		sort.Sort(rawValueSlice(rawValues))
	}

	return ctx.matchExpectedValues(expectedElements, rawValues)
}

// decodeSlice decodes a SET(OF) as a slice
func (ctx *Context) decodeSlice(data []byte, value reflect.Value) error {
	slice := reflect.New(value.Type()).Elem()
	var err error
	for len(data) > 0 {
		elem := reflect.New(value.Type().Elem()).Elem()
		data, err = ctx.DecodeWithOptions(data, elem.Addr().Interface(), "")
		if err != nil {
			return err
		}
		slice.Set(reflect.Append(slice, elem))
	}
	value.Set(slice)
	return nil
}

// decodeArray decodes a SET(OF) as an array
func (ctx *Context) decodeArray(data []byte, value reflect.Value) error {
	var err error
	for i := 0; i < value.Len(); i++ {
		if len(data) == 0 {
			return parseError(ctx, "Missing elements.")
		}
		elem := reflect.New(value.Type().Elem()).Elem()
		data, err = ctx.DecodeWithOptions(data, elem.Addr().Interface(), "")
		if err != nil {
			return err
		}
		value.Index(i).Set(elem)
	}
	if len(data) > 0 {
		return parseError(ctx, "Too many elements.")
	}
	return nil
}

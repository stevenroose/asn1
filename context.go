package asn1

import (
	"fmt"
	"io/ioutil"
	"log"
	"reflect"
)

// Context keeps global options for Asn.1 encoding and decoding
//
// Use the NewContext() function to create a new Context instance:
//
//	ctx := ber.NewContext()
//	// Set options, ex:
//	ctx.SetDer(true)
//	// And call decode or encode functions
//	bytes, err := ctx.EncodeWithOptions(value, "explicit,application,tag:5")
//	...
//
type Context struct {
	log     *log.Logger
	choices map[string][]choiceEntry
	der     struct {
		encoding bool
		decoding bool
	}
}

// Choice represents one option available for a CHOICE element.
type Choice struct {
	Type    reflect.Type
	Options string
}

// Internal register with information about the each CHOICE.
type choiceEntry struct {
	expectedElement
	typ  reflect.Type
	opts *fieldOptions
}

// NewContext creates and initializes a new context.
func NewContext() *Context {
	ctx := &Context{}
	ctx.log = defaultLogger()
	ctx.choices = make(map[string][]choiceEntry)
	ctx.SetDer(true, false)
	return ctx
}

// getChoices returns a list of choices for a given name.
func (ctx *Context) getChoices(choice string) ([]choiceEntry, error) {
	entries := ctx.choices[choice]
	if entries == nil {
		return nil, syntaxError(ctx, "Invalid choice \"%s\"", choice)
	}
	return entries, nil
}

// getChoiceByType returns the choice associated to a given name and type.
func (ctx *Context) getChoiceByType(choice string, t reflect.Type) (entry choiceEntry, err error) {
	entries, err := ctx.getChoices(choice)
	if err != nil {
		return
	}

	for _, current := range entries {
		if current.typ == t {
			entry = current
			return
		}
	}
	err = syntaxError(ctx, "Invalid Go type \"%s\" for choice \"%s\"", t, choice)
	return
}

// getChoiceByTag returns the choice associated to a given tag.
func (ctx *Context) getChoiceByTag(choice string, class, tag uint) (entry choiceEntry, err error) {
	entries, err := ctx.getChoices(choice)
	if err != nil {
		return
	}

	for _, current := range entries {
		if current.class == class && current.tag == tag {
			entry = current
			return
		}
	}
	err = syntaxError(ctx, "Invalid tag [%d,%d] for choice \"%s\"", class, tag, choice) // TODO
	return
}

// addChoiceEntry adds a single choice to the list associated to a given name.
func (this *Context) addChoiceEntry(choice string, entry choiceEntry) error {
	for _, current := range this.choices[choice] {
		if current.class == entry.class && current.tag == entry.tag {
			return fmt.Errorf(
				"Choice already registered: %s{%d, %d}",
				choice, entry.class, entry.tag)
		}
	}
	this.choices[choice] = append(this.choices[choice], entry)
	return nil
}

func (ctx *Context) AddChoice(choice string, entries []Choice) error {
	for _, e := range entries {
		opts, err := parseOptions(ctx, e.Options)
		if err != nil {
			return err
		}
		if opts.choice != nil {
			// TODO Add support for nested choices.
			return syntaxError(ctx, "nested choices are not allowed: \"%s\" inside \"%s\".",
				*opts.choice, choice)
		}
		raw := RawValue{}
		elem, err := ctx.getExpectedElement(&raw, e.Type, opts)
		if err != nil {
			return err
		}
		err = ctx.addChoiceEntry(choice, choiceEntry{
			expectedElement: elem,
			typ:             e.Type,
			opts:            opts,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// defaultLogger returns the default Logger. It's used to initialize a new context
// or when the logger is set to nil.
func defaultLogger() *log.Logger {
	return log.New(ioutil.Discard, "", 0)
}

// SetLogger defines the loggers used by all functions.
func (this *Context) SetLogger(logger *log.Logger) {
	if logger == nil {
		logger = defaultLogger()
	}
	this.log = logger
}

// SetDer sets DER mode for encofing and decoding.
func (this *Context) SetDer(encoding bool, decoding bool) {
	this.der.encoding = encoding
	this.der.decoding = decoding
}

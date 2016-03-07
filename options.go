package asn1

import (
	"strconv"
	"strings"
)

type fieldOptions struct {
	universal    bool
	application  bool
	explicit     bool
	indefinite   bool
	optional     bool
	set          bool
	tag          *int
	defaultValue *int
	choice       *string
}

// validate returns an error if any option is invalid.
func (opts *fieldOptions) validate(ctx *Context) error {
	tagError := func(class string) error {
		return syntaxError(ctx,
			"'tag' must be specified when '%s' is used", class)
	}
	if opts.universal && opts.tag == nil {
		return tagError("universal")
	}
	if opts.application && opts.tag == nil {
		return tagError("application")
	}
	if opts.tag != nil && *opts.tag < 0 {
		return syntaxError(ctx, "'tag' cannot be negative: %d", *opts.tag)
	}
	if opts.choice != nil && *opts.choice == "" {
		return syntaxError(ctx, "'choice' cannot be empty")
	}
	return nil
}

// parseOption returns a parsed fieldOptions or an error.
func parseOptions(ctx *Context, s string) (*fieldOptions, error) {
	var opts fieldOptions
	for _, token := range strings.Split(s, ",") {
		args := strings.Split(strings.TrimSpace(token), ":")
		err := parseOption(ctx, &opts, args)
		if err != nil {
			return nil, err
		}
	}
	if err := opts.validate(ctx); err != nil {
		return nil, err
	}
	return &opts, nil
}

// parseOption parse a single option.
func parseOption(ctx *Context, opts *fieldOptions, args []string) error {
	var err error
	switch args[0] {
	case "":
		// ignore

	case "universal":
		opts.universal, err = parseBoolOption(ctx, args)

	case "application":
		opts.application, err = parseBoolOption(ctx, args)

	case "explicit":
		opts.explicit, err = parseBoolOption(ctx, args)

	case "indefinite":
		opts.indefinite, err = parseBoolOption(ctx, args)

	case "optional":
		opts.optional, err = parseBoolOption(ctx, args)

	case "set":
		opts.set, err = parseBoolOption(ctx, args)

	case "tag":
		opts.tag, err = parseIntOption(ctx, args)

	case "default":
		opts.defaultValue, err = parseIntOption(ctx, args)

	case "choice":
		opts.choice, err = parseStringOption(ctx, args)

	default:
		err = syntaxError(ctx, "Invalid option: %s", args[0])
	}
	return err
}

// parseBoolOption just checks if no arguments were given.
func parseBoolOption(ctx *Context, args []string) (bool, error) {
	if len(args) > 1 {
		return false, syntaxError(ctx, "option '%s' does not have arguments.",
			args[0])
	}
	return true, nil
}

// parseIntOption parses an integer argument.
func parseIntOption(ctx *Context, args []string) (*int, error) {
	if len(args) != 2 {
		return nil, syntaxError(ctx, "option '%s' does not have arguments.")
	}
	num, err := strconv.Atoi(args[1])
	if err != nil {
		return nil, syntaxError(ctx, "invalid value '%s' for option '%s'.",
			args[1], args[0])
	}
	return &num, nil
}

// parseStringOption parses a string argument.
func parseStringOption(ctx *Context, args []string) (*string, error) {
	if len(args) != 2 {
		return nil, syntaxError(ctx, "option '%s' does not have arguments.")
	}
	return &args[1], nil
}

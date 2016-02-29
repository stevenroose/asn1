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

func parseOptions(ctx *Context, s string) (*fieldOptions, error) {
	opts := fieldOptions{}
	defs := []struct {
		name string
		args int
		ptr  interface{}
	}{
		// Flags
		{"universal", 0, &opts.universal},
		{"application", 0, &opts.application},
		{"explicit", 0, &opts.explicit},
		{"indefinite", 0, &opts.indefinite},
		{"optional", 0, &opts.optional},
		// Values
		{"set", 0, &opts.set},
		{"tag", 1, &opts.tag},
		{"default", 1, &opts.defaultValue},
		{"choice", 1, &opts.choice},
	}

	for _, token := range strings.Split(s, ",") {
		invalid := true
		token = strings.TrimSpace(token)
		if len(token) == 0 {
			continue
		}
		for _, opt := range defs {
			if opt.args == 0 {
				if token == opt.name {
					switch ptr := opt.ptr.(type) {
					case *bool:
						*ptr = true
					default:
						panic("Invalid field option type.")
					}
					invalid = false
					break
				}
			} else {
				if strings.HasPrefix(token, opt.name+":") {
					value := token[len(opt.name)+1:]
					switch pptr := opt.ptr.(type) {
					case **int:
						i, err := strconv.Atoi(value)
						if err != nil {
							return nil, syntaxError(ctx,
								"Invalid value for option \"%s\": %s", opt.name, value)
						}
						*pptr = new(int)
						**pptr = i
					case **string:
						*pptr = new(string)
						**pptr = value
					default:
						panic("Invalid field option type.")
					}
					invalid = false
					break
				}
			}
		}
		if invalid {
			return nil, syntaxError(ctx, "Invalid option: %s", token)
		}
	}

	tagError := func(class string) error {
		return syntaxError(ctx,
			"A tag must be specified when \"%s\" is used.", class)
	}
	if opts.universal && opts.tag == nil {
		return nil, tagError("universal")
	}
	if opts.application && opts.tag == nil {
		return nil, tagError("application")
	}
	return &opts, nil
}

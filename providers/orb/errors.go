package orb

import (
	"fmt"

	"github.com/amp-labs/connectors/common/interpreter"
)

// Errors follow RFC 7807: {"type", "status", "title", "detail"}, plus `validation_errors` on 400s.
// https://docs.withorb.com/api-reference/error-responses
var errorFormats = interpreter.NewFormatSwitch( // nolint:gochecknoglobals
	interpreter.FormatTemplate{
		MustKeys: []string{"title"},
		Template: func() interpreter.ErrorDescriptor { return &ResponseError{} },
	},
)

type ResponseError struct {
	Type             string `json:"type"`
	Status           int    `json:"status"`
	Title            string `json:"title"`
	Detail           string `json:"detail,omitempty"`
	ValidationErrors []any  `json:"validation_errors,omitempty"`
}

func (r ResponseError) CombineErr(base error) error {
	message := r.Title
	if r.Detail != "" {
		message = fmt.Sprintf("%s: %s", r.Title, r.Detail)
	}

	if len(r.ValidationErrors) != 0 {
		message = fmt.Sprintf("%s %v", message, r.ValidationErrors)
	}

	return fmt.Errorf("%w: %s", base, message)
}

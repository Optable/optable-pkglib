// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package errors

import (
	"bytes"
	"fmt"
)

// PositionalError is an error paired with a position. This is useful for APIs
// that perform bulk operations that can partially fail and the caller must bind
// which input(s) failed. Use the `Position` method to extract the position.
type PositionalError struct {
	pos int
	err error
}

// NewPositionalError creates an error paired with a position.
func NewPositionalError(pos int, err error) error {
	return &PositionalError{pos, err}
}

func (e *PositionalError) Error() string {
	return fmt.Sprintf("Positional(%d): %s", e.pos, e.err.Error())
}

func (e *PositionalError) Position() int {
	return e.pos
}

func (e *PositionalError) Unwrap() error {
	return e.err
}

// Errors is an error that wrap two or more errors. The downside of batching
// many errors is that unwrap will only return the first error. Use the
// `Errors` method to extract all errors.
type Errors struct {
	errs []error
}

func (e *Errors) Error() string {
	buf := new(bytes.Buffer)

	buf.WriteString("Multiple errors: ")
	for i, err := range e.errs {
		fmt.Fprintf(buf, "(%d){%s}\t", i+1, err.Error())
	}

	return buf.String()
}

func (e *Errors) Errors() []error {
	return e.errs
}

func (e *Errors) Unwrap() error {
	// Only return the first error.
	return e.errs[0]
}

func NewErrors(errs ...error) error {
	var errors []error
	for _, err := range errs {
		if err != nil {
			errors = append(errors, err)
		}
	}

	switch len(errors) {
	case 0:
		return nil
	case 1:
		return errors[0]
	default:
		return &Errors{errors}
	}
}

// Copyright © 2021 Optable Technologies Inc. All rights reserved.
// See LICENSE for details.
package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockErr struct{}

func (m *mockErr) Error() string {
	return "mockErr"
}

var myErr = new(mockErr)

func TestPositionalError(t *testing.T) {
	pos := 42
	err := NewPositionalError(pos, myErr)

	var posErr *PositionalError
	assert.ErrorAs(t, err, &posErr)
	if errors.As(err, &posErr) {
		assert.Equal(t, pos, posErr.Position())
		assert.Equal(t, myErr, posErr.Unwrap())
	}
}

func TestErrors(t *testing.T) {
	assert.Nil(t, NewErrors(), "NewErrors should return nil on empty array")
	assert.Nil(t, NewErrors(nil, nil), "NewErrors should return nil when errors only contain nils")
	assert.Equal(t, myErr, NewErrors(myErr), "NewErrors should unwrap a single error")

	err := NewErrors(nil, myErr, nil, myErr, nil)
	var errs *Errors
	assert.ErrorAs(t, err, &errs)
	if errors.As(err, &errs) {
		assert.ElementsMatch(t, []error{myErr, myErr}, errs.Errors())
		assert.Equal(t, myErr, errs.Unwrap())
	}
}

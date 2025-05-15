package otlp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_NewServerHandler(t *testing.T) {
	handler := NewServerHandler()
	assert.NotNil(t, handler)
}

func Test_NewClientHandler(t *testing.T) {
	handler := NewClientHandler()
	assert.NotNil(t, handler)
}

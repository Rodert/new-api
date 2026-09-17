package kitutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskSensitiveInfoPreservesValidationFieldPath(t *testing.T) {
	assert.Equal(t, "extra.resolution is required", MaskSensitiveInfo("extra.resolution is required"))
}

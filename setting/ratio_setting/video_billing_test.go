package ratio_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVideoBillingModeConfiguration(t *testing.T) {
	require.NoError(t, UpdateVideoBillingModeByJSONString(`{}`))
	t.Cleanup(func() {
		require.NoError(t, UpdateVideoBillingModeByJSONString(`{}`))
	})

	assert.Equal(t, VideoBillingModePerRequest, GetVideoBillingMode("kling-video-v3"))

	require.NoError(t, UpdateVideoBillingModeByJSONString(`{"kling-video-v3":"per_second"}`))
	assert.Equal(t, VideoBillingModePerSecond, GetVideoBillingMode("kling-video-v3"))
	assert.Equal(t, VideoBillingModePerRequest, GetVideoBillingMode("video-ds-2.0"))
}

func TestVideoBillingModeRejectsUnsupportedValues(t *testing.T) {
	for _, config := range []string{
		`{"kling-video-v3":"per_request"}`,
		`{"":"per_second"}`,
	} {
		assert.Error(t, UpdateVideoBillingModeByJSONString(config))
	}
}

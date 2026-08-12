package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/stretchr/testify/assert"
)

func TestGeminiImageRequestAlwaysUsesProtocolConversion(t *testing.T) {
	original := model_setting.GetGlobalSettings().PassThroughRequestEnabled
	t.Cleanup(func() {
		model_setting.GetGlobalSettings().PassThroughRequestEnabled = original
	})
	model_setting.GetGlobalSettings().PassThroughRequestEnabled = true

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ApiType:           constant.APITypeGemini,
		UpstreamModelName: "gemini-3.1-flash-image",
		ChannelSetting:    dto.ChannelSettings{PassThroughBodyEnabled: true},
	}}

	assert.False(t, shouldPassThroughImageRequest(info))

	info.UpstreamModelName = "imagen-4.0-generate-001"
	assert.True(t, shouldPassThroughImageRequest(info))

	model_setting.GetGlobalSettings().PassThroughRequestEnabled = false
	info.ChannelSetting.PassThroughBodyEnabled = false
	assert.False(t, shouldPassThroughImageRequest(info))
}

package chongplusvideo

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestUsesChongPlusProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("task_request", relaycommon.TaskSubmitReq{Prompt: "A cat runs on grass", Seconds: "8"})
	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://api2.chongplus.plus"}})

	requestURL, err := adaptor.BuildRequestURL(nil)
	require.NoError(t, err)
	assert.Equal(t, "https://api2.chongplus.plus/v1/videos/generations", requestURL)
	body, err := adaptor.BuildRequestBody(ctx, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "grok-imagine-video"}})
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)
	var request requestPayload
	require.NoError(t, common.Unmarshal(data, &request))
	require.NotNil(t, request.Seconds)
	assert.Equal(t, 8, *request.Seconds)
}

func TestParseTaskResultUsesChongPlusResponse(t *testing.T) {
	result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{"model":"grok-imagine-video","progress":100,"status":"done","video":{"duration":8,"url":"/v1/videos/request-id/content"}}`))
	require.NoError(t, err)
	assert.Equal(t, "SUCCESS", string(result.Status))
	assert.Equal(t, "100%", result.Progress)
	assert.Empty(t, result.Url)
}

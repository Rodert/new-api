package chongplusvideo

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

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
	require.NotNil(t, request.Duration)
	assert.Equal(t, 8, *request.Duration)
}

func TestEstimateBillingRespectsVideoBillingMode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("task_request", relaycommon.TaskSubmitReq{Seconds: "8"})
	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{OriginModelName: "grok-imagine-video-1.5"}

	require.NoError(t, ratio_setting.UpdateVideoBillingModeByJSONString(`{}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateVideoBillingModeByJSONString(`{}`))
	})
	assert.Nil(t, adaptor.EstimateBilling(ctx, info))

	require.NoError(t, ratio_setting.UpdateVideoBillingModeByJSONString(`{"grok-imagine-video-1.5":"per_second"}`))
	assert.Equal(t, map[string]float64{"seconds": 8}, adaptor.EstimateBilling(ctx, info))
}

func TestBuildGrokImagineVideo15FirstFrameRequest(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt: "Animate the subject",
		Image:  "https://example.com/first-frame.png",
		Images: []string{"https://example.com/first-frame.png"},
	})

	body, err := (&TaskAdaptor{}).BuildRequestBody(ctx, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		UpstreamModelName: "grok-imagine-video-1.5",
	}})
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var request requestPayload
	require.NoError(t, common.Unmarshal(data, &request))
	require.NotNil(t, request.Image)
	assert.Equal(t, "https://example.com/first-frame.png", request.Image.URL)
	assert.Empty(t, request.ReferenceImages)
}

func TestBuildGrokImagineVideo15ReferenceImageRequest(t *testing.T) {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:          "Use <IMAGE_1> and <IMAGE_2>",
		ReferenceImages: []string{"https://example.com/person.png", "https://example.com/product.png"},
	})

	body, err := (&TaskAdaptor{}).BuildRequestBody(ctx, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		UpstreamModelName: "grok-imagine-video-1.5",
	}})
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var request requestPayload
	require.NoError(t, common.Unmarshal(data, &request))
	assert.Nil(t, request.Image)
	assert.Equal(t, []imageReference{
		{URL: "https://example.com/person.png"},
		{URL: "https://example.com/product.png"},
	}, request.ReferenceImages)
}

func TestParseTaskResultUsesChongPlusResponse(t *testing.T) {
	result, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{"model":"grok-imagine-video","progress":100,"status":"done","video":{"duration":8,"url":"/v1/videos/request-id/content"}}`))
	require.NoError(t, err)
	assert.Equal(t, "SUCCESS", string(result.Status))
	assert.Equal(t, "100%", result.Progress)
	assert.Empty(t, result.Url)
}

func TestValidateGrokImagineVideo15ImageModes(t *testing.T) {
	tests := []struct {
		name string
		body string
		code string
	}{
		{
			name: "rejects conflicting first frame and reference images",
			body: `{"prompt":"A cinematic scene","image":"https://example.com/first-frame.png","reference_images":["https://example.com/reference.png"]}`,
			code: "invalid_images",
		},
		{
			name: "rejects 1080p reference images",
			body: `{"prompt":"A cinematic scene","resolution":"1080p","reference_images":["https://example.com/reference.png"]}`,
			code: "invalid_resolution",
		},
		{
			name: "rejects unsupported duration",
			body: `{"prompt":"A cinematic scene","seconds":"5"}`,
			code: "invalid_seconds",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/v1/videos", strings.NewReader(tt.body))
			ctx.Request.Header.Set("Content-Type", "application/json")

			taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(ctx, &relaycommon.RelayInfo{
				ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "grok-imagine-video-1.5"},
			})
			require.NotNil(t, taskErr)
			assert.Equal(t, tt.code, taskErr.Code)
		})
	}
}

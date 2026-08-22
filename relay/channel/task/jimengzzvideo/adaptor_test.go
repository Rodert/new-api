package jimengzzvideo

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestBodyUsesJimengZZVideoProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:      "A cat on a skateboard",
		Model:       "video-ds-2.0-fast",
		Seconds:     "15",
		AspectRatio: "9:16",
		Resolution:  "720p",
		Images:      []string{"https://example.com/first.png"},
		Videos:      []string{"https://example.com/input.mp4"},
		Audios:      []string{"https://example.com/input.mp3"},
	})
	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://upstream.example", ApiKey: "upstream-key"}})

	body, err := adaptor.BuildRequestBody(ctx, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "video-ds-2.0-fast"}})
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var got requestPayload
	require.NoError(t, common.Unmarshal(data, &got))
	assert.Equal(t, requestPayload{
		Model:       "video-ds-2.0-fast",
		Prompt:      "A cat on a skateboard",
		Seconds:     "15",
		AspectRatio: "9:16",
		Resolution:  "720p",
		Images:      []string{"https://example.com/first.png"},
		Videos:      []string{"https://example.com/input.mp4"},
		Audios:      []string{"https://example.com/input.mp3"},
	}, got)
	assert.NotContains(t, string(data), "duration")
	assert.NotContains(t, string(data), "size")
	assert.Contains(t, string(data), `"resolution":"720p"`)
}

func TestDoResponseKeepsUpstreamIDPrivate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	adaptor := &TaskAdaptor{}
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(`{"id":"video_upstream","status":"queued"}`))}

	upstreamID, data, taskErr := adaptor.DoResponse(ctx, resp, &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"}})
	require.Nil(t, taskErr)
	assert.Equal(t, "video_upstream", upstreamID)
	assert.JSONEq(t, `{"id":"video_upstream","status":"queued"}`, string(data))
	assert.JSONEq(t, `{"id":"task_public","task_id":"task_public","status":"queued"}`, writer.Body.String())
}

func TestDoResponseKeepsNestedUpstreamIDPrivate(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	adaptor := &TaskAdaptor{}
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(`{"data":{"id":"video_upstream","status":"queued"}}`))}

	upstreamID, _, taskErr := adaptor.DoResponse(ctx, resp, &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"}})
	require.Nil(t, taskErr)
	assert.Equal(t, "video_upstream", upstreamID)
	assert.JSONEq(t, `{"id":"task_public","task_id":"task_public","data":{"id":"task_public","task_id":"task_public","status":"queued"}}`, writer.Body.String())
}

func TestParseTaskResultMapsUpstreamStatus(t *testing.T) {
	adaptor := &TaskAdaptor{}
	tests := []struct {
		name     string
		body     string
		status   model.TaskStatus
		progress string
		reason   string
		url      string
	}{
		{name: "processing", body: `{"id":"video_1","status":"processing","progress":42}`, status: model.TaskStatusInProgress, progress: "42%"},
		{name: "completed", body: `{"id":"video_1","status":"completed","result":{"video_url":"https://cdn.example/video.mp4"}}`, status: model.TaskStatusSuccess, progress: "100%", url: "https://cdn.example/video.mp4"},
		{name: "failed nested response", body: `{"data":{"task_id":"video_1","status":"failed","error":{"message":"upstream failed"}}}`, status: model.TaskStatusFailure, progress: "100%", reason: "upstream failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := adaptor.ParseTaskResult([]byte(tt.body))
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, string(tt.status), got.Status)
			assert.Equal(t, tt.progress, got.Progress)
			assert.Equal(t, tt.reason, got.Reason)
			assert.Equal(t, tt.url, got.Url)
		})
	}
}

func TestValidateRequestRejectsNonPublicMediaURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(`{"model":"video-ds-2.0-fast","prompt":"A cat","images":["data:image/png;base64,abc"]}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(ctx, &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}})
	require.NotNil(t, taskErr)
	assert.Equal(t, "invalid_media_url", taskErr.Code)
}

func TestValidateRequestAcceptsSeedance25Capabilities(t *testing.T) {
	gin.SetMode(gin.TestMode)
	images := make([]string, 30)
	videos := make([]string, 10)
	audios := make([]string, 10)
	for i := range images {
		images[i] = "https://example.com/image.png"
	}
	for i := range videos {
		videos[i] = "https://example.com/input.mp4"
	}
	for i := range audios {
		audios[i] = "https://example.com/input.mp3"
	}
	body, err := common.Marshal(map[string]any{
		"model":      "seedance2.5",
		"prompt":     "A cinematic scene",
		"seconds":    "30",
		"resolution": "720p",
		"images":     images,
		"videos":     videos,
		"audios":     audios,
	})
	require.NoError(t, err)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(string(body)))
	ctx.Request.Header.Set("Content-Type", "application/json")

	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(ctx, &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}})
	require.Nil(t, taskErr)
}

func TestValidateRequestRejectsUnsupportedSeedance25Parameters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name string
		body string
		code string
	}{
		{name: "short duration", body: `{"model":"seedance2.5","prompt":"A scene","seconds":"3"}`, code: "invalid_seconds"},
		{name: "long duration", body: `{"model":"seedance2.5","prompt":"A scene","seconds":"31"}`, code: "invalid_seconds"},
		{name: "unsupported resolution", body: `{"model":"seedance2.5","prompt":"A scene","seconds":"4","resolution":"1080p"}`, code: "invalid_resolution"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(tt.body))
			ctx.Request.Header.Set("Content-Type", "application/json")

			taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(ctx, &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{}})
			require.NotNil(t, taskErr)
			assert.Equal(t, tt.code, taskErr.Code)
		})
	}
}

func TestEstimateBillingUsesSecondsOnlyForPerSecondModels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("task_request", relaycommon.TaskSubmitReq{Seconds: "15"})
	adaptor := &TaskAdaptor{}

	require.NoError(t, ratio_setting.UpdateVideoBillingModeByJSONString(`{"kling-video-v3":"per_second"}`))
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateVideoBillingModeByJSONString(`{}`))
	})

	assert.Equal(t, map[string]float64{"seconds": 15}, adaptor.EstimateBilling(ctx, &relaycommon.RelayInfo{OriginModelName: "kling-video-v3"}))
	assert.Nil(t, adaptor.EstimateBilling(ctx, &relaycommon.RelayInfo{OriginModelName: "video-ds-2.0"}))
}

func TestConvertToOpenAIVideoHidesUpstreamFailureResponse(t *testing.T) {
	adaptor := &TaskAdaptor{}
	data, err := adaptor.ConvertToOpenAIVideo(&model.Task{
		TaskID:     "task_public",
		Status:     model.TaskStatusFailure,
		Progress:   "100%",
		CreatedAt:  100,
		FinishTime: 200,
		Properties: model.Properties{OriginModelName: "seedance2.5"},
		Data:       []byte(`{"error":{"message":"https://upstream.example/internal"}}`),
	})
	require.NoError(t, err)
	assert.NotContains(t, string(data), "upstream.example")
	assert.JSONEq(t, `{"id":"task_public","task_id":"task_public","object":"video","model":"seedance2.5","status":"failed","progress":100,"created_at":100,"completed_at":200,"error":{"message":"Upstream task failed. Please retry later or contact an administrator.","code":"upstream_task_failed"}}`, string(data))
}

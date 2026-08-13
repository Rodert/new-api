package grokvideo

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

func TestBuildRequestBodyUsesGrokVideoProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(writer)
	ctx.Set("task_request", relaycommon.TaskSubmitReq{
		Prompt:      "A cinematic product video",
		Seconds:     "15",
		AspectRatio: "16:9",
		Resolution:  "720p",
		ImageURLs:   []string{"https://example.com/one.png", "https://example.com/two.png"},
	})

	adaptor := &TaskAdaptor{}
	body, err := adaptor.BuildRequestBody(ctx, &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		UpstreamModelName: "grok-image-video",
	}})
	require.NoError(t, err)
	data, err := io.ReadAll(body)
	require.NoError(t, err)

	var got requestPayload
	require.NoError(t, common.Unmarshal(data, &got))
	require.NotNil(t, got.Seconds)
	assert.Equal(t, 10, *got.Seconds)
	assert.Equal(t, "720p", got.Resolution)
	assert.Equal(t, []string{"https://example.com/one.png", "https://example.com/two.png"}, got.ImageURLs)
}

func TestParseTaskResultUsesDocumentedResponseFormat(t *testing.T) {
	adaptor := &TaskAdaptor{}
	result, err := adaptor.ParseTaskResult([]byte(`{
		"code": "success",
		"data": {
			"task_id": "upstream-task",
			"status": "SUCCESS",
			"progress": "100%",
			"result_url": "https://cdn.example/video.mp4"
		}
	}`))
	require.NoError(t, err)
	assert.Equal(t, "SUCCESS", string(result.Status))
	assert.Equal(t, "https://cdn.example/video.mp4", result.Url)
	assert.Equal(t, "100%", result.Progress)
}

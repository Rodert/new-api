package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewriteJimengZZVideoTaskResponseKeepsUpstreamResult(t *testing.T) {
	response, err := rewriteJimengZZVideoTaskResponse([]byte(`{
        "id":"upstream_task",
        "task_id":"upstream_task",
        "status":"completed",
        "progress":100,
        "result":{"video_url":"https://cdn.example/video.mp4","resultUrls":["https://cdn.example/video.mp4"]},
        "video_url":"https://cdn.example/video.mp4"
    }`), "task_public")
	require.NoError(t, err)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(response, &payload))
	assert.Equal(t, "task_public", payload["id"])
	assert.NotContains(t, payload, "task_id")
	assert.Equal(t, "https://cdn.example/video.mp4", payload["video_url"])
	assert.Equal(t, map[string]any{
		"video_url":  "https://cdn.example/video.mp4",
		"resultUrls": []any{"https://cdn.example/video.mp4"},
	}, payload["result"])
}

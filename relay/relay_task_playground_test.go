package relay

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlaygroundVideoTaskResponseFlattensCompletedTask(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_public",
		Status:     model.TaskStatusSuccess,
		SubmitTime: 100,
		StartTime:  110,
		FinishTime: 145,
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://files.example/video.mp4",
		},
	}

	response, taskErr := playgroundVideoTaskResponse(task)
	require.Nil(t, taskErr)

	var payload map[string]any
	require.NoError(t, common.Unmarshal(response, &payload))
	assert.Equal(t, "task_public", payload["task_id"])
	assert.Equal(t, "succeeded", payload["status"])
	assert.Equal(t, "https://files.example/video.mp4", payload["url"])
	assert.Equal(t, float64(100), payload["created_at"])
	assert.Equal(t, float64(145), payload["completed_at"])
	assert.Equal(t, float64(45), payload["processing_time"])
	assert.NotContains(t, payload, "failed_at")
	assert.NotContains(t, payload, "data")
}

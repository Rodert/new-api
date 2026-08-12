package controller

import (
	"context"
	"io"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageTaskResponseMapsTerminalResult(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_image",
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		SubmitTime: 10,
		StartTime:  20,
		FinishTime: 30,
	}
	task.SetData(imageTaskResult{Created: 25, Data: []dto.ImageData{{Url: "https://file.example/result.png"}}})

	response := imageTaskResponse(task)
	assert.Equal(t, "completed", response["status"])
	assert.Equal(t, int64(25), response["created"])
	data, ok := response["data"].([]dto.ImageData)
	require.True(t, ok)
	require.Len(t, data, 1)
	assert.Equal(t, "https://file.example/result.png", data[0].Url)
}

func TestBuildAsyncImageGenerationRequest(t *testing.T) {
	payload := imageTaskPayload{Request: dto.ImageRequest{Model: "gpt-image-2", Prompt: "draw"}}
	body, contentType, path, err := buildAsyncImageRequest(context.Background(), constant.TaskActionImageGeneration, payload)
	require.NoError(t, err)
	assert.Equal(t, "application/json", contentType)
	assert.Equal(t, "/v1/images/generations", path)
	encoded, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"model":"gpt-image-2"`)
}

func TestBuildAsyncImageEditWithoutUploadedFileUsesJSON(t *testing.T) {
	payload := imageTaskPayload{Request: dto.ImageRequest{
		Model:  "gemini-3.1-flash-image",
		Prompt: "edit",
		Image:  []byte(`"https://file.example/reference.png"`),
	}}
	body, contentType, path, err := buildAsyncImageRequest(context.Background(), constant.TaskActionImageEdit, payload)
	require.NoError(t, err)
	assert.Equal(t, "application/json", contentType)
	assert.Equal(t, "/v1/images/edits", path)
	encoded, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), "https://file.example/reference.png")
}

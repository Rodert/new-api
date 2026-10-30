package controller

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAsyncImageReferencesEnforcesCanonicalContract(t *testing.T) {
	request := dto.ImageRequest{
		Images: []byte(`[{"image_url":" https://example.com/one.png "},{"image_url":"data:image/png;base64,iVBORw0KGgo="}]`),
	}

	err := normalizeAsyncImageReferences(&request)

	require.NoError(t, err)
	require.JSONEq(t, `[{"image_url":"https://example.com/one.png"},{"image_url":"data:image/png;base64,iVBORw0KGgo="}]`, string(request.Images))

	tests := []struct {
		name    string
		request dto.ImageRequest
		wantErr string
	}{
		{
			name:    "legacy image field",
			request: dto.ImageRequest{Image: []byte(`"https://example.com/input.png"`)},
			wantErr: "image is not supported for asynchronous JSON tasks; use images[].image_url",
		},
		{
			name:    "string array",
			request: dto.ImageRequest{Images: []byte(`["https://example.com/input.png"]`)},
			wantErr: "images must be an array of objects containing image_url",
		},
		{
			name: "image URL alias",
			request: dto.ImageRequest{Extra: map[string]json.RawMessage{
				"image_urls": json.RawMessage(`["https://example.com/input.png"]`),
			}},
			wantErr: "image_urls is not supported for asynchronous JSON tasks; use images[].image_url",
		},
		{
			name:    "unknown object field",
			request: dto.ImageRequest{Images: []byte(`[{"image_url":"https://example.com/input.png","role":"subject"}]`)},
			wantErr: "images[0] must contain only image_url",
		},
		{
			name:    "insecure URL",
			request: dto.ImageRequest{Images: []byte(`[{"image_url":"http://example.com/input.png"}]`)},
			wantErr: "images[0].image_url is invalid: value must be a public HTTPS URL or image data URL",
		},
		{
			name:    "data URL media type mismatch",
			request: dto.ImageRequest{Images: []byte(`[{"image_url":"data:image/jpeg;base64,iVBORw0KGgo="}]`)},
			wantErr: "images[0].image_url is invalid: data URL content does not match its image media type",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := normalizeAsyncImageReferences(&test.request)
			require.EqualError(t, err, test.wantErr)
		})
	}

	references := make([]imageTaskInputReference, maxAsyncImageReferences+1)
	for index := range references {
		references[index].ImageURL = "https://example.com/input.png"
	}
	images, err := common.Marshal(references)
	require.NoError(t, err)
	overLimit := dto.ImageRequest{Images: images}
	require.EqualError(t, normalizeAsyncImageReferences(&overLimit), "images supports at most 10 reference images")
}

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

func TestBuildAsyncImageEditWithJSONReferences(t *testing.T) {
	payload := imageTaskPayload{Request: dto.ImageRequest{
		Model:  "gemini-3.1-flash-image",
		Prompt: "edit",
		Images: []byte(`[{"image_url":"https://file.example/reference.png"}]`),
	}}
	body, contentType, path, err := buildAsyncImageRequest(context.Background(), constant.TaskActionImageEdit, payload)
	require.NoError(t, err)
	assert.Equal(t, "application/json", contentType)
	assert.Equal(t, "/v1/images/edits", path)
	encoded, err := io.ReadAll(body)
	require.NoError(t, err)
	assert.JSONEq(t, `{"model":"gemini-3.1-flash-image","prompt":"edit","images":[{"image_url":"https://file.example/reference.png"}]}`, string(encoded))
}

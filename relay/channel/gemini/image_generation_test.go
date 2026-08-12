package gemini

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertGeminiGenerateContentImageRequest(t *testing.T) {
	t.Parallel()

	request := dto.ImageRequest{
		Model:   "gemini-3.1-flash-image",
		Prompt:  "a glass greenhouse in the rain",
		Size:    "1792x1024",
		Quality: "4k",
		N:       common.GetPointer(uint(1)),
	}
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: request.Model,
		},
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	converted, err := (&Adaptor{}).ConvertImageRequest(c, info, request)
	require.NoError(t, err)
	payload, ok := converted.(dto.GeminiChatRequest)
	require.True(t, ok)
	require.Len(t, payload.Contents, 1)
	require.Len(t, payload.Contents[0].Parts, 1)
	assert.Equal(t, request.Prompt, payload.Contents[0].Parts[0].Text)
	assert.Equal(t, []string{"TEXT", "IMAGE"}, payload.GenerationConfig.ResponseModalities)
	assert.Nil(t, payload.GenerationConfig.CandidateCount)

	var imageConfig map[string]string
	require.NoError(t, common.Unmarshal(payload.GenerationConfig.ImageConfig, &imageConfig))
	assert.Equal(t, "16:9", imageConfig["aspectRatio"])
	assert.Equal(t, "4K", imageConfig["imageSize"])
}

func TestGetRequestURLUsesGenerateContentForGeminiImageModel(t *testing.T) {
	t.Parallel()

	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{
		ChannelBaseUrl:    "https://generativelanguage.googleapis.com",
		UpstreamModelName: "gemini-3.1-flash-image",
	}}

	requestURL, err := (&Adaptor{}).GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://generativelanguage.googleapis.com/v1beta/models/gemini-3.1-flash-image:generateContent", requestURL)
}

func TestConvertGeminiImageEditIncludesMultipartImage(t *testing.T) {
	t.Parallel()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("model", "gemini-3-pro-image"))
	require.NoError(t, writer.WriteField("prompt", "replace the sky"))
	filePart, err := writer.CreateFormFile("image[]", "reference.png")
	require.NoError(t, err)
	imageBytes := []byte("not-a-real-png")
	_, err = filePart.Write(imageBytes)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, c.Request.ParseMultipartForm(32<<20))

	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesEdits,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-3-pro-image",
		},
	}
	converted, err := (&Adaptor{}).ConvertImageRequest(c, info, dto.ImageRequest{Prompt: "replace the sky"})
	require.NoError(t, err)
	payload, ok := converted.(dto.GeminiChatRequest)
	require.True(t, ok)
	require.Len(t, payload.Contents[0].Parts, 2)
	inlineData := payload.Contents[0].Parts[1].InlineData
	require.NotNil(t, inlineData)
	assert.Equal(t, "application/octet-stream", inlineData.MimeType)
	assert.Equal(t, "bm90LWEtcmVhbC1wbmc=", inlineData.Data)
}

func TestConvertGeminiImageRequestRejectsUnsupportedCountAndMissingEditImage(t *testing.T) {
	t.Parallel()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		RelayMode: relayconstant.RelayModeImagesGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gemini-3.1-flash-image",
		},
	}
	_, err := (&Adaptor{}).ConvertImageRequest(c, info, dto.ImageRequest{
		Prompt: "two images",
		N:      common.GetPointer(uint(2)),
	})
	require.EqualError(t, err, "Gemini image generation supports exactly one image per request")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("prompt", "edit without image"))
	require.NoError(t, writer.Close())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	require.NoError(t, c.Request.ParseMultipartForm(32<<20))
	info.RelayMode = relayconstant.RelayModeImagesEdits
	_, err = (&Adaptor{}).ConvertImageRequest(c, info, dto.ImageRequest{Prompt: "edit without image"})
	require.EqualError(t, err, "Gemini image editing requires an image")
}

func TestGeminiGenerateContentImageHandlerReturnsOpenAIImageResponse(t *testing.T) {
	t.Setenv("R2_ACCOUNT_ID", "")
	t.Setenv("R2_ACCESS_KEY_ID", "")
	t.Setenv("R2_SECRET_ACCESS_KEY", "")
	t.Setenv("R2_BUCKET", "")
	t.Setenv("R2_PUBLIC_BASE_URL", "")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	responseBody := `{
		"candidates":[{"content":{"role":"model","parts":[
			{"text":"done"},
			{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}
		]}}],
		"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":258,"totalTokenCount":270}
	}`
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(responseBody)),
	}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{UpstreamModelName: "gemini-3.1-flash-image"}}

	usage, apiErr := GeminiGenerateContentImageHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)
	assert.Equal(t, 12, usage.PromptTokens)
	assert.Equal(t, 258, usage.CompletionTokens)
	assert.Equal(t, 270, usage.TotalTokens)
	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

	var imageResponse dto.ImageResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &imageResponse))
	require.Len(t, imageResponse.Data, 1)
	assert.Equal(t, "aW1hZ2U=", imageResponse.Data[0].B64Json)
	assert.Empty(t, imageResponse.Data[0].Url)
}

func TestRewriteGeminiInlineImagesToURLsPreservesNativeResponseShape(t *testing.T) {
	t.Parallel()

	response := dto.GeminiChatResponse{Candidates: []dto.GeminiChatCandidate{{
		Content: dto.GeminiChatContent{Parts: []dto.GeminiPart{
			{Text: "generated image"},
			{InlineData: &dto.GeminiInlineData{MimeType: "image/png", Data: "aW1hZ2U="}},
		}},
	}}}

	changed := rewriteGeminiInlineImagesToURLs(&response, func(encoded, contentType string) (string, bool) {
		assert.Equal(t, "aW1hZ2U=", encoded)
		assert.Equal(t, "image/png", contentType)
		return "https://file.lunadownload.com/temporary/2026/08/12/generated.png", true
	})

	require.True(t, changed)
	parts := response.Candidates[0].Content.Parts
	assert.Equal(t, "generated image", parts[0].Text)
	assert.Nil(t, parts[1].InlineData)
	require.NotNil(t, parts[1].FileData)
	assert.Equal(t, "image/png", parts[1].FileData.MimeType)
	assert.Equal(t, "https://file.lunadownload.com/temporary/2026/08/12/generated.png", parts[1].FileData.FileUri)
}

func TestRewriteGeminiInlineImagesToURLsKeepsInlineDataWhenUploadFails(t *testing.T) {
	t.Parallel()

	inlineData := &dto.GeminiInlineData{MimeType: "image/png", Data: "aW1hZ2U="}
	response := dto.GeminiChatResponse{Candidates: []dto.GeminiChatCandidate{{
		Content: dto.GeminiChatContent{Parts: []dto.GeminiPart{{InlineData: inlineData}}},
	}}}

	changed := rewriteGeminiInlineImagesToURLs(&response, func(string, string) (string, bool) {
		return "", false
	})

	assert.False(t, changed)
	assert.Same(t, inlineData, response.Candidates[0].Content.Parts[0].InlineData)
	assert.Nil(t, response.Candidates[0].Content.Parts[0].FileData)
}

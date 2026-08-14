package gemini

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/QuantumNous/new-api/setting/reasoning"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

type Adaptor struct {
}

const maxGeminiReferenceImageSize = 10 << 20

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	if len(request.Contents) > 0 {
		for i, content := range request.Contents {
			if i == 0 {
				if request.Contents[0].Role == "" {
					request.Contents[0].Role = "user"
				}
			}
			for _, part := range content.Parts {
				if part.FileData != nil {
					if part.FileData.MimeType == "" && strings.Contains(part.FileData.FileUri, "www.youtube.com") {
						part.FileData.MimeType = "video/webm"
					}
				}
			}
		}
	}
	return request, nil
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, req *dto.ClaudeRequest) (any, error) {
	result, err := relayconvert.ConvertRequest(c, info, types.RelayFormatGemini, req)
	if err != nil {
		return nil, err
	}
	geminiRequest, ok := result.Value.(*dto.GeminiChatRequest)
	if !ok {
		return nil, fmt.Errorf("expected Gemini generateContent request, got %T", result.Value)
	}
	return geminiRequest, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	// convert size to aspect ratio but allow user to specify aspect ratio
	aspectRatio := "1:1" // default aspect ratio
	size := strings.TrimSpace(request.Size)
	if size != "" {
		if strings.Contains(size, ":") {
			aspectRatio = size
		} else {
			switch size {
			case "256x256", "512x512", "1024x1024":
				aspectRatio = "1:1"
			case "1536x1024":
				aspectRatio = "3:2"
			case "1024x1536":
				aspectRatio = "2:3"
			case "1024x1792":
				aspectRatio = "9:16"
			case "1792x1024":
				aspectRatio = "16:9"
			}
		}
	}

	if model_setting.IsGeminiModelSupportImagine(info.UpstreamModelName) {
		if lo.FromPtrOr(request.N, uint(1)) != 1 {
			return nil, errors.New("Gemini image generation supports exactly one image per request")
		}

		imageConfig := map[string]string{"aspectRatio": aspectRatio}
		switch strings.ToLower(request.Quality) {
		case "hd", "high", "2k":
			imageConfig["imageSize"] = "2K"
		case "4k":
			imageConfig["imageSize"] = "4K"
		case "fast", "standard", "medium", "low", "auto", "1k", "":
			imageConfig["imageSize"] = "1K"
		default:
			imageConfig["imageSize"] = "1K"
		}
		imageConfigBytes, err := common.Marshal(imageConfig)
		if err != nil {
			return nil, fmt.Errorf("marshal Gemini image config: %w", err)
		}

		parts := []dto.GeminiPart{{Text: request.Prompt}}
		if info.RelayMode == constant.RelayModeImagesEdits {
			imageParts, imageErr := geminiEditImageParts(c, request)
			if imageErr != nil {
				return nil, imageErr
			}
			parts = append(parts, imageParts...)
			if len(parts) == 1 {
				return nil, errors.New("Gemini image editing requires an image")
			}
		}

		return dto.GeminiChatRequest{
			Contents: []dto.GeminiChatContent{{Role: "user", Parts: parts}},
			GenerationConfig: dto.GeminiChatGenerationConfig{
				ResponseModalities: []string{"TEXT", "IMAGE"},
				ImageConfig:        imageConfigBytes,
			},
		}, nil
	}

	if !strings.HasPrefix(info.UpstreamModelName, "imagen") {
		return nil, errors.New("model does not support image generation")
	}

	// build gemini imagen request
	geminiRequest := dto.GeminiImageRequest{
		Instances: []dto.GeminiImageInstance{
			{
				Prompt: request.Prompt,
			},
		},
		Parameters: dto.GeminiImageParameters{
			SampleCount:      int(lo.FromPtrOr(request.N, uint(1))),
			AspectRatio:      aspectRatio,
			PersonGeneration: "allow_adult", // default allow adult
		},
	}

	// Set imageSize when quality parameter is specified
	// Map quality parameter to imageSize (only supported by Standard and Ultra models)
	// quality values: auto, high, medium, low (for gpt-image-1), hd, standard (for dall-e-3)
	// imageSize values: 1K (default), 2K
	// https://ai.google.dev/gemini-api/docs/imagen
	// https://platform.openai.com/docs/api-reference/images/create
	if request.Quality != "" {
		imageSize := "1K" // default
		switch request.Quality {
		case "hd", "high":
			imageSize = "2K"
		case "2K":
			imageSize = "2K"
		case "standard", "medium", "low", "auto", "1K":
			imageSize = "1K"
		default:
			// unknown quality value, default to 1K
			imageSize = "1K"
		}
		geminiRequest.Parameters.ImageSize = imageSize
	}

	return geminiRequest, nil
}

// geminiEditImageParts normalizes OpenAI image edit references into Gemini inlineData.
// Multipart files remain supported; JSON requests accept HTTPS URLs and data URLs.
func geminiEditImageParts(c *gin.Context, request dto.ImageRequest) ([]dto.GeminiPart, error) {
	if len(request.Mask) > 0 {
		return nil, errors.New("Gemini image editing does not support mask")
	}
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		return geminiJSONImageParts(c, request)
	}

	form := c.Request.MultipartForm
	if form == nil {
		var err error
		form, err = common.ParseMultipartFormReusable(c)
		if err != nil {
			return nil, fmt.Errorf("parse Gemini image edit form: %w", err)
		}
	}
	parts := make([]dto.GeminiPart, 0)
	for fieldName, files := range form.File {
		if fieldName != "image" && fieldName != "image[]" && !strings.HasPrefix(fieldName, "image[") {
			continue
		}
		for _, fileHeader := range files {
			file, err := fileHeader.Open()
			if err != nil {
				return nil, fmt.Errorf("open Gemini reference image: %w", err)
			}
			data, readErr := io.ReadAll(io.LimitReader(file, maxGeminiReferenceImageSize+1))
			closeErr := file.Close()
			if readErr != nil {
				return nil, fmt.Errorf("read Gemini reference image: %w", readErr)
			}
			if closeErr != nil {
				return nil, fmt.Errorf("close Gemini reference image: %w", closeErr)
			}
			part, err := geminiInlineImagePart(data, fileHeader.Header.Get("Content-Type"))
			if err != nil {
				return nil, err
			}
			parts = append(parts, part)
		}
	}
	return parts, nil
}

func geminiJSONImageParts(c *gin.Context, request dto.ImageRequest) ([]dto.GeminiPart, error) {
	sources := make([]string, 0)
	for _, raw := range []json.RawMessage{request.Image, request.Images} {
		if len(raw) == 0 {
			continue
		}
		var one string
		if err := common.Unmarshal(raw, &one); err == nil {
			sources = append(sources, one)
			continue
		}
		var many []string
		if err := common.Unmarshal(raw, &many); err != nil {
			return nil, errors.New("Gemini image editing requires image references as URL or data URL strings")
		}
		sources = append(sources, many...)
	}

	parts := make([]dto.GeminiPart, 0, len(sources))
	for _, source := range sources {
		part, err := geminiImageSourcePart(c, source)
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	return parts, nil
}

func geminiImageSourcePart(c *gin.Context, source string) (dto.GeminiPart, error) {
	source = strings.TrimSpace(source)
	if strings.HasPrefix(source, "data:") {
		meta, encoded, ok := strings.Cut(source[5:], ",")
		if !ok || !strings.HasSuffix(strings.ToLower(meta), ";base64") {
			return dto.GeminiPart{}, errors.New("invalid image data URL")
		}
		if len(encoded) > base64.StdEncoding.EncodedLen(maxGeminiReferenceImageSize) {
			return dto.GeminiPart{}, errors.New("Gemini image reference must be no larger than 10 MB")
		}
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return dto.GeminiPart{}, fmt.Errorf("decode image data URL: %w", err)
		}
		return geminiInlineImagePart(data, meta[:len(meta)-len(";base64")])
	}

	parsed, err := url.Parse(source)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return dto.GeminiPart{}, errors.New("Gemini image references must be HTTPS URLs or data URLs")
	}
	if err := service.ValidateSSRFProtectedFetchURL(source); err != nil {
		return dto.GeminiPart{}, fmt.Errorf("invalid Gemini image reference URL: %w", err)
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, source, nil)
	if err != nil {
		return dto.GeminiPart{}, err
	}
	resp, err := service.GetSSRFProtectedHTTPClient().Do(req)
	if err != nil {
		return dto.GeminiPart{}, fmt.Errorf("download Gemini image reference: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return dto.GeminiPart{}, fmt.Errorf("download Gemini image reference: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxGeminiReferenceImageSize+1))
	if err != nil {
		return dto.GeminiPart{}, fmt.Errorf("read Gemini image reference: %w", err)
	}
	return geminiInlineImagePart(data, resp.Header.Get("Content-Type"))
}

func geminiInlineImagePart(data []byte, contentType string) (dto.GeminiPart, error) {
	if len(data) == 0 || len(data) > maxGeminiReferenceImageSize {
		return dto.GeminiPart{}, errors.New("Gemini image reference must be no larger than 10 MB")
	}
	detectedContentType := http.DetectContentType(data)
	if detectedContentType != "image/jpeg" && detectedContentType != "image/png" && detectedContentType != "image/webp" {
		return dto.GeminiPart{}, errors.New("Gemini image reference content type is not supported")
	}
	if parsedContentType, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = parsedContentType
	}
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = detectedContentType
	}
	contentType = strings.ToLower(contentType)
	if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
		return dto.GeminiPart{}, errors.New("Gemini image reference content type is not supported")
	}
	if contentType != detectedContentType {
		return dto.GeminiPart{}, errors.New("Gemini image reference content type does not match file content")
	}
	return dto.GeminiPart{InlineData: &dto.GeminiInlineData{
		MimeType: contentType,
		Data:     base64.StdEncoding.EncodeToString(data),
	}}, nil
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {

}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {

	if model_setting.GetGeminiSettings().ThinkingAdapterEnabled &&
		!model_setting.ShouldPreserveThinkingSuffix(info.OriginModelName) {
		// 新增逻辑：处理 -thinking-<budget> 格式
		if strings.Contains(info.UpstreamModelName, "-thinking-") {
			parts := strings.Split(info.UpstreamModelName, "-thinking-")
			info.UpstreamModelName = parts[0]
		} else if strings.HasSuffix(info.UpstreamModelName, "-thinking") { // 旧的适配
			info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-thinking")
		} else if strings.HasSuffix(info.UpstreamModelName, "-nothinking") {
			info.UpstreamModelName = strings.TrimSuffix(info.UpstreamModelName, "-nothinking")
		} else if baseModel, level, ok := reasoning.TrimEffortSuffix(info.UpstreamModelName); ok && level != "" {
			info.UpstreamModelName = baseModel
		}
	}

	version := model_setting.GetGeminiVersionSetting(info.UpstreamModelName)

	if strings.HasPrefix(info.UpstreamModelName, "imagen") {
		return fmt.Sprintf("%s/%s/models/%s:predict", info.ChannelBaseUrl, version, info.UpstreamModelName), nil
	}

	if strings.HasPrefix(info.UpstreamModelName, "text-embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "gemini-embedding") {
		action := "embedContent"
		if info.IsGeminiBatchEmbedding {
			action = "batchEmbedContents"
		}
		return fmt.Sprintf("%s/%s/models/%s:%s", info.ChannelBaseUrl, version, info.UpstreamModelName, action), nil
	}

	action := "generateContent"
	if info.IsStream {
		action = "streamGenerateContent?alt=sse"
		if info.RelayMode == constant.RelayModeGemini {
			info.DisablePing = true
		}
	}
	return fmt.Sprintf("%s/%s/models/%s:%s", info.ChannelBaseUrl, version, info.UpstreamModelName, action), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	if info.RelayMode == constant.RelayModeImagesGenerations || info.RelayMode == constant.RelayModeImagesEdits {
		req.Set("Content-Type", "application/json")
	}
	req.Set("x-goog-api-key", info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	result, err := relayconvert.ConvertRequest(c, info, types.RelayFormatGemini, request)
	if err != nil {
		return nil, err
	}
	return result.Value, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	if request.Input == nil {
		return nil, errors.New("input is required")
	}

	inputs := request.ParseInput()
	if len(inputs) == 0 {
		return nil, errors.New("input is empty")
	}
	// We always build a batch-style payload with `requests`, so ensure we call the
	// batch endpoint upstream to avoid payload/endpoint mismatches.
	info.IsGeminiBatchEmbedding = true
	// process all inputs
	geminiRequests := make([]map[string]interface{}, 0, len(inputs))
	for _, input := range inputs {
		geminiRequest := map[string]interface{}{
			"model": fmt.Sprintf("models/%s", info.UpstreamModelName),
			"content": dto.GeminiChatContent{
				Parts: []dto.GeminiPart{
					{
						Text: input,
					},
				},
			},
		}

		// set specific parameters for different models
		// https://ai.google.dev/api/embeddings?hl=zh-cn#method:-models.embedcontent
		switch info.UpstreamModelName {
		case "text-embedding-004", "gemini-embedding-exp-03-07", "gemini-embedding-001":
			// Only newer models introduced after 2024 support OutputDimensionality
			dimensions := lo.FromPtrOr(request.Dimensions, 0)
			if dimensions > 0 {
				geminiRequest["outputDimensionality"] = dimensions
			}
		}
		geminiRequests = append(geminiRequests, geminiRequest)
	}

	return map[string]interface{}{
		"requests": geminiRequests,
	}, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	result, err := relayconvert.ConvertRequest(c, info, types.RelayFormatGemini, &request)
	if err != nil {
		return nil, err
	}
	geminiRequest, ok := result.Value.(*dto.GeminiChatRequest)
	if !ok {
		return nil, fmt.Errorf("expected Gemini generateContent request, got %T", result.Value)
	}
	return geminiRequest, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if info.RelayMode == constant.RelayModeResponses {
		if info.IsStream {
			return GeminiResponsesStreamHandler(c, info, resp)
		}
		return GeminiResponsesHandler(c, info, resp)
	}

	if info.RelayMode == constant.RelayModeGemini {
		if strings.Contains(info.RequestURLPath, ":embedContent") ||
			strings.Contains(info.RequestURLPath, ":batchEmbedContents") {
			return NativeGeminiEmbeddingHandler(c, resp, info)
		}
		if info.IsStream {
			return GeminiTextGenerationStreamHandler(c, info, resp)
		} else {
			return GeminiTextGenerationHandler(c, info, resp)
		}
	}

	if info.RelayMode == constant.RelayModeImagesGenerations || info.RelayMode == constant.RelayModeImagesEdits {
		if model_setting.IsGeminiModelSupportImagine(info.UpstreamModelName) {
			return GeminiGenerateContentImageHandler(c, info, resp)
		}
	}

	if strings.HasPrefix(info.UpstreamModelName, "imagen") {
		return GeminiImageHandler(c, info, resp)
	}

	// check if the model is an embedding model
	if strings.HasPrefix(info.UpstreamModelName, "text-embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "embedding") ||
		strings.HasPrefix(info.UpstreamModelName, "gemini-embedding") {
		return GeminiEmbeddingHandler(c, info, resp)
	}

	if info.IsStream {
		return GeminiChatStreamHandler(c, info, resp)
	} else {
		return GeminiChatHandler(c, info, resp)
	}

}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

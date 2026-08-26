package chongplusvideo

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/tidwall/sjson"
)

var ModelList = []string{"grok-imagine-video", "grok-imagine-video-1.5"}

const ChannelName = "chongplus-video"

var grokImagineVideo15SupportedSeconds = map[int]struct{}{
	4: {}, 6: {}, 8: {}, 10: {}, 12: {}, 15: {},
}

type imageReference struct {
	URL string `json:"url"`
}

type requestPayload struct {
	Model           string           `json:"model"`
	Prompt          string           `json:"prompt"`
	Duration        *int             `json:"duration,omitempty"`
	AspectRatio     string           `json:"aspect_ratio,omitempty"`
	Resolution      string           `json:"resolution,omitempty"`
	Image           *imageReference  `json:"image,omitempty"`
	ReferenceImages []imageReference `json:"reference_images,omitempty"`
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.apiKey = info.ApiKey
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	if taskErr := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionTextGenerate); taskErr != nil {
		return taskErr
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if info.UpstreamModelName != "grok-imagine-video-1.5" {
		images := append(append([]string{}, req.ImageURLs...), req.Images...)
		if len(images) == 0 && len(req.ReferenceImages) == 0 {
			return nil
		}
		return service.TaskErrorWrapperLocal(errors.New("ChongPlus video does not support reference images"), "invalid_images", http.StatusBadRequest)
	}
	seconds := req.Duration
	if req.Seconds != "" {
		parsedSeconds, parseErr := strconv.Atoi(req.Seconds)
		if parseErr != nil {
			return service.TaskErrorWrapperLocal(errors.New("seconds must be one of: 4, 6, 8, 10, 12, 15"), "invalid_seconds", http.StatusBadRequest)
		}
		seconds = parsedSeconds
	}
	if seconds != 0 {
		if _, ok := grokImagineVideo15SupportedSeconds[seconds]; !ok {
			return service.TaskErrorWrapperLocal(errors.New("seconds must be one of: 4, 6, 8, 10, 12, 15"), "invalid_seconds", http.StatusBadRequest)
		}
	}
	firstFrameImages := firstFrameImages(req)
	if len(firstFrameImages) > 0 && len(req.ReferenceImages) > 0 {
		return service.TaskErrorWrapperLocal(errors.New("do not send both a first-frame image and reference_images"), "invalid_images", http.StatusBadRequest)
	}
	if len(firstFrameImages) > 1 {
		return service.TaskErrorWrapperLocal(errors.New("first-frame image supports exactly one item"), "invalid_images", http.StatusBadRequest)
	}
	if len(req.ReferenceImages) > 7 {
		return service.TaskErrorWrapperLocal(errors.New("reference_images supports at most 7 items"), "invalid_images", http.StatusBadRequest)
	}
	if len(req.ReferenceImages) > 0 && strings.EqualFold(strings.TrimSpace(requestResolution(req)), "1080p") {
		return service.TaskErrorWrapperLocal(errors.New("reference_images supports up to 720p resolution"), "invalid_resolution", http.StatusBadRequest)
	}
	for _, image := range append(firstFrameImages, req.ReferenceImages...) {
		if strings.HasPrefix(image, "data:") {
			continue
		}
		parsed, parseErr := url.ParseRequestURI(image)
		if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return service.TaskErrorWrapperLocal(errors.New("images must contain public HTTP(S) URLs or data URLs"), "invalid_images", http.StatusBadRequest)
		}
	}
	return nil
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, _ *relaycommon.RelayInfo) map[string]float64 {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}
	seconds, _ := strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	if seconds <= 0 {
		seconds = 8
	}
	return map[string]float64{"seconds": float64(seconds)}
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return a.baseURL + "/v1/videos/generations", nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}
	body := requestPayload{Model: info.UpstreamModelName, Prompt: req.Prompt, AspectRatio: req.AspectRatio, Resolution: requestResolution(req)}
	if info.UpstreamModelName == "grok-imagine-video-1.5" {
		images := firstFrameImages(req)
		if len(images) == 1 {
			body.Image = &imageReference{URL: images[0]}
		}
		for _, image := range req.ReferenceImages {
			body.ReferenceImages = append(body.ReferenceImages, imageReference{URL: image})
		}
	}
	if seconds, err := strconv.Atoi(req.Seconds); err == nil && seconds > 0 {
		body.Duration = &seconds
	} else if req.Duration > 0 {
		body.Duration = &req.Duration
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, body io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, body)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *taskdto.TaskError) {
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	var response struct { RequestID string `json:"request_id"` }
	if err := common.Unmarshal(body, &response); err != nil {
		return "", nil, service.TaskErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	if response.RequestID == "" {
		return "", nil, service.TaskErrorWrapper(errors.New("request_id is empty"), "invalid_response", http.StatusInternalServerError)
	}
	public, err := sjson.SetBytes(body, "id", info.PublicTaskID)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "rewrite_response_failed", http.StatusInternalServerError)
	}
	public, _ = sjson.SetBytes(public, "task_id", info.PublicTaskID)
	c.Data(http.StatusOK, "application/json", public)
	return response.RequestID, body, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, _ := body["task_id"].(string)
	if taskID == "" {
		return nil, errors.New("invalid task_id")
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/videos/"+url.PathEscape(taskID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	var response struct {
		Status   string `json:"status"`
		Progress int    `json:"progress"`
		Video    struct { URL string `json:"url"` } `json:"video"`
	}
	if err := common.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	result := &relaycommon.TaskInfo{Progress: fmt.Sprintf("%d%%", response.Progress)}
	switch strings.ToLower(response.Status) {
	case "done", "success", "completed":
		result.Status = model.TaskStatusSuccess
		result.Progress = "100%"
	case "failed", "error", "cancelled", "canceled":
		result.Status = model.TaskStatusFailure
		result.Progress = "100%"
	default:
		result.Status = model.TaskStatusInProgress
	}
	return result, nil
}

func (a *TaskAdaptor) GetModelList() []string { return ModelList }
func (a *TaskAdaptor) GetChannelName() string { return ChannelName }
func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	return common.Marshal(task.ToOpenAIVideo())
}

func firstFrameImages(req relaycommon.TaskSubmitReq) []string {
	images := make([]string, 0, 1+len(req.Images)+len(req.ImageURLs))
	if image := strings.TrimSpace(req.Image); image != "" {
		images = append(images, image)
	}
	images = append(images, req.Images...)
	images = append(images, req.ImageURLs...)
	if image := req.GetInputReferenceURL(); image != "" {
		images = append(images, image)
	}
	uniqueImages := make([]string, 0, len(images))
	seen := make(map[string]struct{}, len(images))
	for _, image := range images {
		image = strings.TrimSpace(image)
		if image == "" {
			continue
		}
		if _, exists := seen[image]; exists {
			continue
		}
		seen[image] = struct{}{}
		uniqueImages = append(uniqueImages, image)
	}
	return uniqueImages
}

func requestResolution(req relaycommon.TaskSubmitReq) string {
	if req.Resolution != "" {
		return req.Resolution
	}
	return req.Size
}

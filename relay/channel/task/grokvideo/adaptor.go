package grokvideo

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

type requestPayload struct {
	Model       string   `json:"model"`
	Prompt      string   `json:"prompt"`
	Seconds     *int     `json:"seconds,omitempty"`
	AspectRatio string   `json:"aspect_ratio,omitempty"`
	Resolution  string   `json:"resolution,omitempty"`
	ImageURLs   []string `json:"image_urls,omitempty"`
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
	if err := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionTextGenerate); err != nil {
		return err
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if len(req.ImageURLs) > 0 && len(req.Images) > 0 {
		return service.TaskErrorWrapperLocal(errors.New("do not send both image_urls and images"), "invalid_images", http.StatusBadRequest)
	}
	if req.GetInputReferenceURL() != "" && len(req.ReferenceImages) > 0 {
		return service.TaskErrorWrapperLocal(errors.New("do not send both input_reference and reference_images"), "invalid_images", http.StatusBadRequest)
	}
	images := append([]string{}, req.ImageURLs...)
	images = append(images, req.Images...)
	images = append(images, req.ReferenceImages...)
	if ref := req.GetInputReferenceURL(); ref != "" {
		images = append(images, ref)
	}
	if len(images) > 7 {
		return service.TaskErrorWrapperLocal(errors.New("image_urls supports at most 7 items"), "invalid_images", http.StatusBadRequest)
	}
	for _, image := range images {
		if strings.HasPrefix(image, "data:") {
			continue
		}
		parsed, parseErr := url.ParseRequestURI(image)
		if parseErr != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return service.TaskErrorWrapperLocal(errors.New("image_urls must contain public HTTP(S) URLs or data URLs"), "invalid_images", http.StatusBadRequest)
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
	if seconds == 0 {
		seconds = 4
	}
	return map[string]float64{"seconds": float64(seconds)}
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return a.baseURL + "/v1/video/generations", nil
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
	images := append([]string{}, req.ImageURLs...)
	images = append(images, req.Images...)
	if ref := req.GetInputReferenceURL(); ref != "" {
		images = append(images, ref)
	}
	resolution := req.Resolution
	if resolution == "" {
		resolution = req.Size
	}
	body := requestPayload{Model: info.UpstreamModelName, Prompt: req.Prompt, AspectRatio: req.AspectRatio, Resolution: resolution, ImageURLs: images}
	if seconds, parseErr := strconv.Atoi(req.Seconds); parseErr == nil && seconds > 0 {
		body.Seconds = &seconds
	} else if req.Duration > 0 {
		body.Seconds = &req.Duration
	}
	if len(images) >= 2 && body.Model == "grok-image-video" && body.Seconds != nil && *body.Seconds > 10 {
		*body.Seconds = 10
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
	var envelope map[string]any
	if err := common.Unmarshal(body, &envelope); err != nil {
		return "", nil, service.TaskErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	taskID := stringValue(envelope["task_id"])
	if taskID == "" {
		taskID = stringValue(envelope["id"])
	}
	if data, ok := envelope["data"].(map[string]any); ok && taskID == "" {
		taskID = stringValue(data["task_id"])
		if taskID == "" {
			taskID = stringValue(data["id"])
		}
	}
	if taskID == "" {
		return "", nil, service.TaskErrorWrapper(errors.New("task_id is empty"), "invalid_response", http.StatusInternalServerError)
	}
	public, err := sjson.SetBytes(body, "id", info.PublicTaskID)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "rewrite_response_failed", http.StatusInternalServerError)
	}
	public, _ = sjson.SetBytes(public, "task_id", info.PublicTaskID)
	if _, ok := envelope["data"].(map[string]any); ok {
		public, _ = sjson.SetBytes(public, "data.id", info.PublicTaskID)
		public, _ = sjson.SetBytes(public, "data.task_id", info.PublicTaskID)
	}
	c.Data(http.StatusOK, "application/json", public)
	return taskID, body, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID := stringValue(body["task_id"])
	if taskID == "" {
		return nil, errors.New("invalid task_id")
	}
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/video/generations/"+url.PathEscape(taskID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error) {
	var envelope map[string]any
	if err := common.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}
	data, _ := envelope["data"].(map[string]any)
	if data == nil {
		data = envelope
	}
	status := strings.ToUpper(stringValue(data["status"]))
	result := &relaycommon.TaskInfo{TaskID: stringValue(data["task_id"]), Progress: stringValue(data["progress"]), Url: stringValue(data["result_url"]), Reason: stringValue(data["fail_reason"])}
	if result.Progress == "" {
		result.Progress = "20%"
	}
	if _, err := strconv.Atoi(strings.TrimSuffix(result.Progress, "%")); err == nil && !strings.HasSuffix(result.Progress, "%") {
		result.Progress += "%"
	}
	switch status {
	case "SUCCESS", "COMPLETED", "SUCCEEDED":
		result.Status = model.TaskStatusSuccess
		result.Progress = "100%"
	case "FAILURE", "FAILED", "CANCELLED", "CANCELED":
		result.Status = model.TaskStatusFailure
		result.Progress = "100%"
	case "SUBMITTED", "QUEUED", "NOT_START":
		result.Status = model.TaskStatusQueued
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

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

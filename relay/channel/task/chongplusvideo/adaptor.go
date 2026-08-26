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

var ModelList = []string{"grok-imagine-video"}

const ChannelName = "chongplus-video"

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
	if taskErr := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionTextGenerate); taskErr != nil {
		return taskErr
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	images := append(append([]string{}, req.ImageURLs...), req.Images...)
	if len(images) > 0 {
		return service.TaskErrorWrapperLocal(errors.New("ChongPlus video does not support reference images"), "invalid_images", http.StatusBadRequest)
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
	body := requestPayload{Model: info.UpstreamModelName, Prompt: req.Prompt, AspectRatio: req.AspectRatio, Resolution: req.Resolution}
	if seconds, err := strconv.Atoi(req.Seconds); err == nil && seconds > 0 {
		body.Seconds = &seconds
	} else if req.Duration > 0 {
		body.Seconds = &req.Duration
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

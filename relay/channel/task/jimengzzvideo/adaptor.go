package jimengzzvideo

import (
	"bytes"
	"encoding/json"
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

const (
	maxImages = 4
	maxVideos = 3
	maxAudios = 1
)

type requestPayload struct {
	Model       string   `json:"model"`
	Prompt      string   `json:"prompt"`
	Seconds     string   `json:"seconds,omitempty"`
	AspectRatio string   `json:"aspect_ratio,omitempty"`
	Images      []string `json:"images,omitempty"`
	Videos      []string `json:"videos,omitempty"`
	Audios      []string `json:"audios,omitempty"`
}

type responseError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

type responseResult struct {
	VideoURL   string   `json:"video_url"`
	ResultURLs []string `json:"resultUrls"`
}

type responseTask struct {
	ID       string          `json:"id"`
	TaskID   string          `json:"task_id"`
	Status   string          `json:"status"`
	Progress json.RawMessage `json:"progress"`
	VideoURL string          `json:"video_url"`
	Result   *responseResult `json:"result"`
	Error    *responseError  `json:"error"`
	Data     *responseTask   `json:"data"`
}

type TaskAdaptor struct {
	taskcommon.BaseBilling
	apiKey  string
	baseURL string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	if taskErr := relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate); taskErr != nil {
		return taskErr
	}

	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Model) == "" {
		return service.TaskErrorWrapperLocal(errors.New("model is required"), "missing_model", http.StatusBadRequest)
	}
	if len(req.Images) > maxImages {
		return service.TaskErrorWrapperLocal(fmt.Errorf("images supports at most %d items", maxImages), "invalid_images", http.StatusBadRequest)
	}
	if len(req.Videos) > maxVideos {
		return service.TaskErrorWrapperLocal(fmt.Errorf("videos supports at most %d items", maxVideos), "invalid_videos", http.StatusBadRequest)
	}
	if len(req.Audios) > maxAudios {
		return service.TaskErrorWrapperLocal(fmt.Errorf("audios supports at most %d item", maxAudios), "invalid_audios", http.StatusBadRequest)
	}
	for field, values := range map[string][]string{
		"images": req.Images,
		"videos": req.Videos,
		"audios": req.Audios,
	} {
		for _, value := range values {
			mediaURL, err := url.ParseRequestURI(value)
			if err != nil || (mediaURL.Scheme != "http" && mediaURL.Scheme != "https") || mediaURL.Host == "" {
				return service.TaskErrorWrapperLocal(fmt.Errorf("%s must contain public HTTP(S) URLs", field), "invalid_media_url", http.StatusBadRequest)
			}
		}
	}
	if req.Seconds != "" {
		seconds, err := strconv.Atoi(req.Seconds)
		if err != nil || seconds < 1 || seconds > relaycommon.MaxTaskDurationSeconds {
			return service.TaskErrorWrapperLocal(fmt.Errorf("seconds must be between 1 and %d", relaycommon.MaxTaskDurationSeconds), "invalid_seconds", http.StatusBadRequest)
		}
	}
	return nil
}

func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return a.baseURL + "/v1/videos", nil
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

	body := requestPayload{
		Model:       info.UpstreamModelName,
		Prompt:      req.Prompt,
		Seconds:     req.Seconds,
		AspectRatio: req.AspectRatio,
		Images:      req.Images,
		Videos:      req.Videos,
		Audios:      req.Audios,
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (string, []byte, *taskdto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	result, err := parseResponse(responseBody)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	var responseEnvelope responseTask
	if err := common.Unmarshal(responseBody, &responseEnvelope); err != nil {
		return "", nil, service.TaskErrorWrapper(err, "unmarshal_response_body_failed", http.StatusInternalServerError)
	}
	upstreamTaskID := result.ID
	if upstreamTaskID == "" {
		upstreamTaskID = result.TaskID
	}
	if upstreamTaskID == "" {
		return "", nil, service.TaskErrorWrapper(errors.New("task_id is empty"), "invalid_response", http.StatusInternalServerError)
	}

	clientResponse, err := sjson.SetBytes(responseBody, "id", info.PublicTaskID)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "rewrite_response_failed", http.StatusInternalServerError)
	}
	clientResponse, err = sjson.SetBytes(clientResponse, "task_id", info.PublicTaskID)
	if err != nil {
		return "", nil, service.TaskErrorWrapper(err, "rewrite_response_failed", http.StatusInternalServerError)
	}
	if responseEnvelope.ID == "" && responseEnvelope.TaskID == "" && responseEnvelope.Data != nil {
		clientResponse, err = sjson.SetBytes(clientResponse, "data.id", info.PublicTaskID)
		if err != nil {
			return "", nil, service.TaskErrorWrapper(err, "rewrite_response_failed", http.StatusInternalServerError)
		}
		clientResponse, err = sjson.SetBytes(clientResponse, "data.task_id", info.PublicTaskID)
		if err != nil {
			return "", nil, service.TaskErrorWrapper(err, "rewrite_response_failed", http.StatusInternalServerError)
		}
	}
	c.Data(http.StatusOK, "application/json", clientResponse)
	return upstreamTaskID, responseBody, nil
}

func (a *TaskAdaptor) FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, errors.New("invalid task_id")
	}

	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/v1/videos/"+taskID, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	result, err := parseResponse(respBody)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := &relaycommon.TaskInfo{}
	switch strings.ToLower(result.Status) {
	case "queued", "pending", "submitted":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = taskcommon.ProgressQueued
	case "processing", "in_progress", "running":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = taskcommon.ProgressInProgress
	case "completed", "success", "succeeded":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = taskcommon.ProgressComplete
	case "failed", "cancelled", "canceled":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Progress = taskcommon.ProgressComplete
		if result.Error != nil {
			taskResult.Reason = result.Error.Message
		}
		if taskResult.Reason == "" {
			taskResult.Reason = "task failed"
		}
	default:
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = taskcommon.ProgressInProgress
	}
	if progress := parseProgress(result.Progress); progress != "" {
		taskResult.Progress = progress
	}
	if result.VideoURL != "" {
		taskResult.Url = result.VideoURL
	} else if result.Result != nil {
		if result.Result.VideoURL != "" {
			taskResult.Url = result.Result.VideoURL
		} else if len(result.Result.ResultURLs) > 0 {
			taskResult.Url = result.Result.ResultURLs[0]
		}
	}
	return taskResult, nil
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	data, err := sjson.SetBytes(originTask.Data, "id", originTask.TaskID)
	if err != nil {
		return nil, errors.Wrap(err, "set id failed")
	}
	data, err = sjson.SetBytes(data, "task_id", originTask.TaskID)
	if err != nil {
		return nil, errors.Wrap(err, "set task_id failed")
	}
	var responseEnvelope responseTask
	if err := common.Unmarshal(originTask.Data, &responseEnvelope); err != nil {
		return nil, errors.Wrap(err, "parse task response failed")
	}
	if responseEnvelope.ID == "" && responseEnvelope.TaskID == "" && responseEnvelope.Data != nil {
		data, err = sjson.SetBytes(data, "data.id", originTask.TaskID)
		if err != nil {
			return nil, errors.Wrap(err, "set nested id failed")
		}
		data, err = sjson.SetBytes(data, "data.task_id", originTask.TaskID)
		if err != nil {
			return nil, errors.Wrap(err, "set nested task_id failed")
		}
	}
	return data, nil
}

func parseResponse(body []byte) (*responseTask, error) {
	result := &responseTask{}
	if err := common.Unmarshal(body, result); err != nil {
		return nil, err
	}
	if result.ID == "" && result.TaskID == "" && result.Data != nil {
		return result.Data, nil
	}
	return result, nil
}

func parseProgress(value json.RawMessage) string {
	if len(value) == 0 {
		return ""
	}
	var progress string
	if err := common.Unmarshal(value, &progress); err == nil && progress != "" {
		if strings.HasSuffix(progress, "%") {
			return progress
		}
		if _, err := strconv.Atoi(progress); err == nil {
			return progress + "%"
		}
	}
	var percent int
	if err := common.Unmarshal(value, &percent); err == nil && percent >= 0 && percent <= 100 {
		return strconv.Itoa(percent) + "%"
	}
	return ""
}

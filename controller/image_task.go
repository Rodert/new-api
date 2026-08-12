package controller

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	hosttypes "github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const asyncImageTaskBatchSize = 100
const maxAsyncGeneratedImageSize = 50 << 20

type imageTaskPayload struct {
	Request    dto.ImageRequest     `json:"request"`
	References []imageTaskReference `json:"references,omitempty"`
	FormValues map[string][]string  `json:"form_values,omitempty"`
}

type imageTaskReference struct {
	Field       string `json:"field"`
	URL         string `json:"url"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
}

type imageTaskResult struct {
	Data     []dto.ImageData `json:"data"`
	Created  int64           `json:"created"`
	Metadata any             `json:"metadata,omitempty"`
}

type imageTaskRunSummary struct {
	Processed int `json:"processed"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
}

func CreateImageTask(c *gin.Context) {
	if store, err := common.NewR2StoreFromEnv(); err != nil || store == nil {
		message := "R2 storage is required for asynchronous image tasks"
		if err != nil {
			message = err.Error()
		}
		respondImageTaskError(c, http.StatusServiceUnavailable, message, "storage_unavailable")
		return
	}
	action := constant.TaskActionImageGeneration
	contentType := c.GetHeader("Content-Type")
	if strings.Contains(contentType, "multipart/form-data") {
		action = constant.TaskActionImageEdit
	}

	// The validator determines edit/generation behavior from the path.
	originalPath := c.Request.URL.Path
	if action == constant.TaskActionImageEdit {
		c.Request.URL.Path = "/v1/images/edits"
	} else {
		c.Request.URL.Path = "/v1/images/generations"
	}
	request, err := helper.GetAndValidOpenAIImageRequest(c, relayconstant.Path2RelayMode(c.Request.URL.Path))
	c.Request.URL.Path = originalPath
	if err != nil {
		respondImageTaskError(c, http.StatusBadRequest, err.Error(), "invalid_request_error")
		return
	}
	if request.Stream != nil && *request.Stream {
		respondImageTaskError(c, http.StatusBadRequest, "stream is not supported for asynchronous image tasks", "invalid_request_error")
		return
	}
	if action == constant.TaskActionImageGeneration && (len(request.Image) > 0 || len(request.Images) > 0 || len(request.Mask) > 0) {
		action = constant.TaskActionImageEdit
	}

	payload := imageTaskPayload{Request: *request}
	if action == constant.TaskActionImageEdit {
		if c.Request.MultipartForm != nil {
			payload.FormValues = c.Request.MultipartForm.Value
			payload.References, err = persistAsyncImageReferences(c)
		}
		if err != nil {
			status := http.StatusBadRequest
			if !errors.Is(err, common.ErrRequestBodyTooLarge) && !strings.Contains(err.Error(), "required") && !strings.Contains(err.Error(), "supported") {
				status = http.StatusInternalServerError
			}
			respondImageTaskError(c, status, err.Error(), "invalid_request_error")
			return
		}
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAIImage, request, nil)
	if err != nil {
		respondImageTaskError(c, http.StatusInternalServerError, err.Error(), "internal_error")
		return
	}
	relayInfo.RelayMode = relayconstant.Path2RelayMode(c.Request.URL.Path)
	if action == constant.TaskActionImageEdit {
		relayInfo.RelayMode = relayconstant.RelayModeImagesEdits
	} else {
		relayInfo.RelayMode = relayconstant.RelayModeImagesGenerations
	}
	relayInfo.InitChannelMeta(c)

	meta := request.GetTokenCountMeta()
	tokens, err := service.EstimateRequestToken(c, meta, relayInfo)
	if err != nil {
		respondImageTaskError(c, http.StatusBadRequest, err.Error(), "count_token_failed")
		return
	}
	relayInfo.SetEstimatePromptTokens(tokens)
	priceData, err := helper.ModelPriceHelper(c, relayInfo, tokens, meta)
	if err != nil {
		respondImageTaskError(c, http.StatusBadRequest, err.Error(), "model_price_error")
		return
	}
	relayInfo.ForcePreConsume = true
	if !priceData.FreeModel {
		if billingErr := service.PreConsumeBilling(c, priceData.QuotaToPreConsume, relayInfo); billingErr != nil {
			respondImageTaskError(c, billingErr.StatusCode, billingErr.Error(), string(billingErr.GetErrorCode()))
			return
		}
	}
	if billingErr := service.PrepareTieredBillingForSelectedGroup(c, relayInfo); billingErr != nil {
		if relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		respondImageTaskError(c, billingErr.StatusCode, billingErr.Error(), string(billingErr.GetErrorCode()))
		return
	}

	task := model.InitTask(constant.TaskPlatformImage, relayInfo)
	task.Status = model.TaskStatusQueued
	task.Action = action
	task.Quota = relayInfo.FinalPreConsumedQuota
	task.PrivateData.BillingSource = relayInfo.BillingSource
	task.PrivateData.SubscriptionId = relayInfo.SubscriptionId
	task.PrivateData.TokenId = relayInfo.TokenId
	task.PrivateData.NodeName = common.NodeName
	task.PrivateData.BillingContext = &model.TaskBillingContext{
		ModelPrice:      relayInfo.PriceData.ModelPrice,
		GroupRatio:      relayInfo.PriceData.GroupRatioInfo.GroupRatio,
		ModelRatio:      relayInfo.PriceData.ModelRatio,
		OtherRatios:     relayInfo.PriceData.OtherRatios(),
		OriginModelName: relayInfo.OriginModelName,
		UsePrice:        relayInfo.PriceData.UsePrice,
	}
	task.SetData(payload)
	if err := task.Insert(); err != nil {
		if relayInfo.Billing != nil {
			relayInfo.Billing.Refund(c)
		}
		respondImageTaskError(c, http.StatusInternalServerError, "failed to create image task", "internal_error")
		return
	}

	c.JSON(http.StatusAccepted, imageTaskResponse(task))
}

func GetImageTask(c *gin.Context) {
	task, exists, err := model.GetByTaskId(c.GetInt("id"), c.Param("task_id"))
	if err != nil {
		respondImageTaskError(c, http.StatusInternalServerError, "failed to query image task", "internal_error")
		return
	}
	if !exists || task.Platform != constant.TaskPlatformImage {
		respondImageTaskError(c, http.StatusNotFound, "image task not found", "not_found")
		return
	}
	c.JSON(http.StatusOK, imageTaskResponse(task))
}

func imageTaskResponse(task *model.Task) gin.H {
	status := "queued"
	switch task.Status {
	case model.TaskStatusInProgress:
		status = "processing"
	case model.TaskStatusSuccess:
		status = "completed"
	case model.TaskStatusFailure:
		status = "failed"
	}
	response := gin.H{
		"id":           task.TaskID,
		"object":       "image.task",
		"status":       status,
		"progress":     task.Progress,
		"created_at":   task.SubmitTime,
		"started_at":   task.StartTime,
		"completed_at": task.FinishTime,
	}
	if task.Status == model.TaskStatusSuccess {
		var result imageTaskResult
		if task.GetData(&result) == nil {
			response["data"] = result.Data
			response["created"] = result.Created
		}
	}
	if task.Status == model.TaskStatusFailure {
		response["error"] = gin.H{"message": task.FailReason, "type": "image_task_failed"}
	}
	return response
}

func persistAsyncImageReferences(c *gin.Context) ([]imageTaskReference, error) {
	store, err := common.NewR2StoreFromEnv()
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, errors.New("R2 storage is required for asynchronous image edits")
	}
	form := c.Request.MultipartForm
	if form == nil {
		return nil, errors.New("image is required")
	}
	var references []imageTaskReference
	for _, field := range []string{"image", "image[]", "mask"} {
		for _, file := range form.File[field] {
			if file.Size <= 0 || file.Size > maxImageAssetSize {
				return nil, errors.New("image file must be no larger than 10 MB")
			}
			contentType := strings.ToLower(strings.TrimSpace(file.Header.Get("Content-Type")))
			if contentType != "image/jpeg" && contentType != "image/png" && contentType != "image/webp" {
				return nil, errors.New("image content type is not supported")
			}
			source, openErr := file.Open()
			if openErr != nil {
				return nil, openErr
			}
			filename := filepath.Base(file.Filename)
			key := common.R2DatePrefix(time.Now()) + uuid.NewString() + safePlaygroundAssetExtension(filename)
			putErr := store.Put(c.Request.Context(), key, contentType, source, file.Size)
			_ = source.Close()
			if putErr != nil {
				return nil, fmt.Errorf("failed to store image reference: %w", putErr)
			}
			references = append(references, imageTaskReference{Field: field, URL: store.URL(key), Filename: filename, ContentType: contentType})
		}
	}
	if len(references) == 0 {
		return nil, errors.New("image is required")
	}
	return references, nil
}

func runPendingImageTasks(ctx context.Context) (imageTaskRunSummary, error) {
	summary := imageTaskRunSummary{}
	timeoutMinutes := common.GetEnvOrDefault("ASYNC_IMAGE_TASK_TIMEOUT_MINUTES", 10)
	for _, task := range model.GetTimedOutImageTasks(time.Now().Unix()-int64(timeoutMinutes)*60, asyncImageTaskBatchSize) {
		finishImageTaskFailure(ctx, task, fmt.Errorf("image task timed out after %d minutes", timeoutMinutes))
		summary.Processed++
		summary.Failed++
	}
	concurrency := common.GetEnvOrDefault("ASYNC_IMAGE_TASK_CONCURRENCY", 2)
	if concurrency < 1 {
		concurrency = 1
	} else if concurrency > 16 {
		concurrency = 16
	}
	jobs := make(chan *model.Task)
	results := make(chan bool, asyncImageTaskBatchSize)
	var workers sync.WaitGroup
	for range concurrency {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for task := range jobs {
				if err := executeImageTask(ctx, task); err != nil {
					finishImageTaskFailure(ctx, task, err)
					results <- false
					continue
				}
				results <- true
			}
		}()
	}
	claimedCount := 0
	queuedTasks := make([]*model.Task, 0, asyncImageTaskBatchSize)
	for _, task := range model.GetPendingImageTasks(asyncImageTaskBatchSize) {
		if ctx.Err() != nil {
			break
		}
		fromStatus := task.Status
		task.Status = model.TaskStatusInProgress
		task.Progress = "10%"
		task.StartTime = time.Now().Unix()
		claimed, err := task.UpdateWithStatus(fromStatus)
		if err != nil {
			return summary, err
		}
		if !claimed {
			continue
		}
		claimedCount++
		queuedTasks = append(queuedTasks, task)
	}
	for _, task := range queuedTasks {
		jobs <- task
	}
	close(jobs)
	go func() {
		workers.Wait()
		close(results)
	}()
	for succeeded := range results {
		summary.Processed++
		if succeeded {
			summary.Succeeded++
		} else {
			summary.Failed++
		}
	}
	if claimedCount == 0 && ctx.Err() != nil {
		return summary, ctx.Err()
	}
	return summary, ctx.Err()
}

func executeImageTask(parent context.Context, task *model.Task) error {
	var payload imageTaskPayload
	if err := task.GetData(&payload); err != nil {
		return fmt.Errorf("decode task payload: %w", err)
	}
	body, contentType, path, err := buildAsyncImageRequest(parent, task.Action, payload)
	if err != nil {
		return err
	}
	requestCtx, cancel := context.WithTimeout(parent, time.Duration(common.GetEnvOrDefault("ASYNC_IMAGE_TASK_TIMEOUT_MINUTES", 10))*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req
	common.SetContextKey(c, common.RequestIdKey, common.NewRequestId())
	common.SetContextKey(c, constant.ContextKeyRequestStartTime, time.Now())

	user, err := model.GetUserCache(task.UserId)
	if err != nil {
		return fmt.Errorf("load task user: %w", err)
	}
	user.WriteContext(c)
	common.SetContextKey(c, constant.ContextKeyUsingGroup, task.Group)
	if task.PrivateData.TokenId > 0 {
		token, tokenErr := model.GetTokenById(task.PrivateData.TokenId)
		if tokenErr == nil {
			tokenErr = middleware.SetupContextForToken(c, token)
		}
		if tokenErr != nil {
			return tokenErr
		}
	}
	channel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		return fmt.Errorf("load task channel: %w", err)
	}
	if apiErr := middleware.SetupContextForSelectedChannel(c, channel, payload.Request.Model); apiErr != nil {
		return apiErr
	}

	info, err := relaycommon.GenRelayInfo(c, types.RelayFormatOpenAIImage, &payload.Request, nil)
	if err != nil {
		return err
	}
	info.RelayMode = relayconstant.Path2RelayMode(path)
	info.InitChannelMeta(c)
	info.FinalPreConsumedQuota = task.Quota
	info.BillingSource = task.PrivateData.BillingSource
	info.SubscriptionId = task.PrivateData.SubscriptionId
	if bc := task.PrivateData.BillingContext; bc != nil {
		info.PriceData = hosttypes.PriceData{
			ModelPrice: bc.ModelPrice, ModelRatio: bc.ModelRatio, UsePrice: bc.UsePrice,
			GroupRatioInfo: hosttypes.GroupRatioInfo{GroupRatio: bc.GroupRatio},
		}
		info.PriceData.ReplaceOtherRatios(bc.OtherRatios)
	}
	usage, logContent, apiErr := relay.ExecuteImage(c, info, false)
	if apiErr != nil {
		return apiErr
	}

	var result imageTaskResult
	if err := common.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		return fmt.Errorf("decode image response: %w", err)
	}
	if len(result.Data) == 0 {
		return errors.New("upstream returned no images")
	}
	if err := persistAsyncImageResults(requestCtx, result.Data); err != nil {
		return err
	}
	fromStatus := task.Status
	task.Status = model.TaskStatusSuccess
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	task.FailReason = ""
	task.SetData(result)
	updated, err := task.UpdateWithStatus(fromStatus)
	if err != nil {
		return err
	}
	if !updated {
		return errors.New("image task status changed during execution")
	}
	service.PostTextConsumeQuota(c, info, usage, logContent)
	task.Quota = info.FinalPreConsumedQuota
	if info.PriceData.UsePrice {
		actualQuota := info.PriceData.ApplyOtherRatiosToFloat(info.PriceData.ModelPrice * common.QuotaPerUnit * info.PriceData.GroupRatioInfo.GroupRatio)
		task.Quota = common.QuotaFromFloat(actualQuota)
	}
	if err := task.UpdateQuota(); err != nil {
		common.SysError(fmt.Sprintf("failed to update completed image task quota %s: %v", task.TaskID, err))
	}
	return nil
}

func persistAsyncImageResults(ctx context.Context, images []dto.ImageData) error {
	store, err := common.NewR2StoreFromEnv()
	if err != nil {
		return err
	}
	if store == nil {
		return errors.New("R2 storage is required for asynchronous image tasks")
	}
	for index := range images {
		if images[index].Url == "" {
			return errors.New("image result has no persistent URL")
		}
		if strings.HasPrefix(images[index].Url, store.URL("")) {
			continue
		}
		if err := service.ValidateSSRFProtectedFetchURL(images[index].Url); err != nil {
			return fmt.Errorf("invalid generated image URL: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, images[index].Url, nil)
		if err != nil {
			return err
		}
		resp, err := service.GetSSRFProtectedHTTPClient().Do(req)
		if err != nil {
			return fmt.Errorf("download generated image: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return fmt.Errorf("download generated image: status %d", resp.StatusCode)
		}
		payload, readErr := io.ReadAll(io.LimitReader(resp.Body, maxAsyncGeneratedImageSize+1))
		_ = resp.Body.Close()
		if readErr != nil {
			return readErr
		}
		if len(payload) == 0 || len(payload) > maxAsyncGeneratedImageSize {
			return errors.New("generated image size is invalid")
		}
		contentType := http.DetectContentType(payload)
		extension := ""
		switch contentType {
		case "image/png":
			extension = ".png"
		case "image/jpeg":
			extension = ".jpg"
		case "image/webp":
			extension = ".webp"
		default:
			return errors.New("generated image content type is not supported")
		}
		key := common.R2DatePrefix(time.Now()) + uuid.NewString() + extension
		if err := store.Put(ctx, key, contentType, bytes.NewReader(payload), int64(len(payload))); err != nil {
			return fmt.Errorf("store generated image: %w", err)
		}
		images[index].Url = store.URL(key)
	}
	return nil
}

func buildAsyncImageRequest(ctx context.Context, action string, payload imageTaskPayload) (io.Reader, string, string, error) {
	if action != constant.TaskActionImageEdit || len(payload.References) == 0 {
		body, err := common.Marshal(payload.Request)
		path := "/v1/images/generations"
		if action == constant.TaskActionImageEdit {
			path = "/v1/images/edits"
		}
		return bytes.NewReader(body), "application/json", path, err
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, values := range payload.FormValues {
		for _, value := range values {
			if err := writer.WriteField(key, value); err != nil {
				return nil, "", "", err
			}
		}
	}
	for _, reference := range payload.References {
		if err := appendAsyncImageReference(ctx, writer, reference); err != nil {
			return nil, "", "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", "", err
	}
	return &body, writer.FormDataContentType(), "/v1/images/edits", nil
}

func appendAsyncImageReference(ctx context.Context, writer *multipart.Writer, reference imageTaskReference) error {
	if err := service.ValidateSSRFProtectedFetchURL(reference.URL); err != nil {
		return fmt.Errorf("invalid stored image URL: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reference.URL, nil)
	if err != nil {
		return err
	}
	resp, err := service.GetSSRFProtectedHTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("download image reference: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download image reference: status %d", resp.StatusCode)
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, reference.Field, filepath.Base(reference.Filename)))
	header.Set("Content-Type", reference.ContentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return err
	}
	written, err := io.Copy(part, io.LimitReader(resp.Body, maxImageAssetSize+1))
	if err != nil {
		return err
	}
	if written > maxImageAssetSize {
		return errors.New("stored image reference is larger than 10 MB")
	}
	return nil
}

func finishImageTaskFailure(ctx context.Context, task *model.Task, runErr error) {
	fromStatus := task.Status
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	task.FailReason = common.LocalLogPreview(runErr.Error())
	updated, err := task.UpdateWithStatus(fromStatus)
	if err != nil || !updated {
		common.SysError(fmt.Sprintf("failed to mark image task %s failed: %v", task.TaskID, err))
		return
	}
	service.RefundTaskQuota(ctx, task, task.FailReason)
}

func respondImageTaskError(c *gin.Context, status int, message, code string) {
	c.JSON(status, gin.H{"error": gin.H{"message": message, "type": code, "code": code}})
}

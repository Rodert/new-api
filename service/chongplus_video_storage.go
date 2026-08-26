package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/google/uuid"
)

const maxPersistedChongPlusVideoSize int64 = 2 << 30

// persistChongPlusVideoToR2 copies the protected ChongPlus content endpoint to R2.
// The returned URL is safe to expose after the task has completed.
func persistChongPlusVideoToR2(ctx context.Context, task *model.Task, baseURL, key, proxy string) (string, error) {
	store, err := common.NewR2StoreFromEnv()
	if err != nil {
		return "", err
	}
	if store == nil {
		return "", fmt.Errorf("R2 storage is not configured")
	}

	contentURL := strings.TrimRight(baseURL, "/") + "/v1/videos/" + url.PathEscape(task.GetUpstreamTaskID()) + "/content"
	if err := ValidateSSRFProtectedFetchURL(contentURL); err != nil {
		return "", fmt.Errorf("invalid ChongPlus content URL: %w", err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodGet, contentURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+key)

	client := GetSSRFProtectedHTTPClient()
	if proxy != "" {
		client, err = GetHttpClientWithProxy(proxy)
		if err != nil {
			return "", err
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download ChongPlus video: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return "", fmt.Errorf("download ChongPlus video: status %d", resp.StatusCode)
	}
	if resp.ContentLength > maxPersistedChongPlusVideoSize {
		return "", fmt.Errorf("ChongPlus video exceeds the %d byte storage limit", maxPersistedChongPlusVideoSize)
	}

	file, err := os.CreateTemp("", "new-api-chongplus-video-*.mp4")
	if err != nil {
		return "", fmt.Errorf("create temporary video file: %w", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	size, err := io.Copy(file, io.LimitReader(resp.Body, maxPersistedChongPlusVideoSize+1))
	if err != nil {
		return "", fmt.Errorf("save ChongPlus video: %w", err)
	}
	if size == 0 || size > maxPersistedChongPlusVideoSize {
		return "", fmt.Errorf("ChongPlus video size is invalid")
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind temporary video file: %w", err)
	}

	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if !strings.HasPrefix(contentType, "video/") {
		contentType = "video/mp4"
	}
	keyName := common.R2DatePrefix(time.Now()) + uuid.NewString() + ".mp4"
	if err := store.Put(requestCtx, keyName, contentType, file, size); err != nil {
		return "", fmt.Errorf("store ChongPlus video: %w", err)
	}
	return store.URL(keyName), nil
}

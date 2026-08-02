package controller

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	playgroundAssetDirectory       = "/data/playground-assets"
	playgroundAssetLifetime        = 24 * time.Hour
	playgroundAssetCleanupInterval = time.Hour
	maxImageAssetSize              = 20 << 20
	maxVideoAssetSize              = 100 << 20
	maxAudioAssetSize              = 20 << 20
)

type playgroundAssetMetadata struct {
	ContentType string `json:"content_type"`
	ExpiresAt   int64  `json:"expires_at"`
	Kind        string `json:"kind"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
}

// StartPlaygroundAssetCleanup keeps short-lived playground uploads from
// accumulating in the persistent data volume.
func StartPlaygroundAssetCleanup() {
	go func() {
		cleanupExpiredPlaygroundAssets(time.Now())
		ticker := time.NewTicker(playgroundAssetCleanupInterval)
		defer ticker.Stop()
		for now := range ticker.C {
			cleanupExpiredPlaygroundAssets(now)
		}
	}()
}

func PlaygroundAssetUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxVideoAssetSize+(1<<20))
	kind := c.PostForm("kind")
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "file is required"})
		return
	}

	maxSize, allowedContentTypes, ok := playgroundAssetConstraints(kind)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid asset kind"})
		return
	}
	if file.Size <= 0 || file.Size > maxSize {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "asset size is invalid"})
		return
	}

	contentType := strings.ToLower(strings.TrimSpace(file.Header.Get("Content-Type")))
	if !allowedContentTypes[contentType] {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "asset content type is not supported"})
		return
	}

	assetID := uuid.NewString()
	filename := filepath.Base(file.Filename)
	if err := savePlaygroundAsset(assetID, file, playgroundAssetMetadata{
		ContentType: contentType,
		ExpiresAt:   time.Now().Add(playgroundAssetLifetime).Unix(),
		Kind:        kind,
		Filename:    filename,
		Size:        file.Size,
	}); err != nil {
		common.SysError(fmt.Sprintf("failed to save playground asset: %s", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to save asset"})
		return
	}

	baseURL := playgroundAssetBaseURL(c.Request)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"kind":         kind,
			"url":          baseURL + "/pg/assets/" + assetID,
			"filename":     filename,
			"content_type": contentType,
			"size":         file.Size,
		},
	})
}

func PlaygroundAssetFetch(c *gin.Context) {
	assetID := c.Param("asset_id")
	if _, err := uuid.Parse(assetID); err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	metadata, err := loadPlaygroundAssetMetadata(assetID)
	if err != nil || metadata.ExpiresAt <= time.Now().Unix() {
		if metadata.ExpiresAt > 0 && metadata.ExpiresAt <= time.Now().Unix() {
			_ = os.Remove(playgroundAssetPath(assetID))
			_ = os.Remove(playgroundAssetMetadataPath(assetID))
		}
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Content-Type", metadata.ContentType)
	c.Header("Cache-Control", "public, max-age=3600")
	http.ServeFile(c.Writer, c.Request, playgroundAssetPath(assetID))
}

func playgroundAssetConstraints(kind string) (int64, map[string]bool, bool) {
	switch kind {
	case "image":
		return maxImageAssetSize, map[string]bool{
			"image/jpeg": true,
			"image/png":  true,
			"image/webp": true,
		}, true
	case "video":
		return maxVideoAssetSize, map[string]bool{
			"video/mp4":       true,
			"video/webm":      true,
			"video/quicktime": true,
		}, true
	case "audio":
		return maxAudioAssetSize, map[string]bool{
			"audio/aac":  true,
			"audio/mpeg": true,
			"audio/mp4":  true,
			"audio/ogg":  true,
			"audio/wav":  true,
			"audio/webm": true,
		}, true
	default:
		return 0, nil, false
	}
}

func savePlaygroundAsset(assetID string, file *multipart.FileHeader, metadata playgroundAssetMetadata) error {
	if err := os.MkdirAll(playgroundAssetDirectory, 0o750); err != nil {
		return err
	}

	source, err := file.Open()
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.OpenFile(playgroundAssetPath(assetID), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		_ = os.Remove(playgroundAssetPath(assetID))
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(playgroundAssetPath(assetID))
		return closeErr
	}

	data, err := common.Marshal(metadata)
	if err != nil {
		_ = os.Remove(playgroundAssetPath(assetID))
		return err
	}
	if err := os.WriteFile(playgroundAssetMetadataPath(assetID), data, 0o600); err != nil {
		_ = os.Remove(playgroundAssetPath(assetID))
		return err
	}
	return nil
}

func loadPlaygroundAssetMetadata(assetID string) (playgroundAssetMetadata, error) {
	data, err := os.ReadFile(playgroundAssetMetadataPath(assetID))
	if err != nil {
		return playgroundAssetMetadata{}, err
	}
	var metadata playgroundAssetMetadata
	if err := common.Unmarshal(data, &metadata); err != nil {
		return playgroundAssetMetadata{}, err
	}
	return metadata, nil
}

func playgroundAssetPath(assetID string) string {
	return filepath.Join(playgroundAssetDirectory, assetID)
}

func playgroundAssetMetadataPath(assetID string) string {
	return playgroundAssetPath(assetID) + ".json"
}

func playgroundAssetBaseURL(request *http.Request) string {
	scheme := "http"
	if forwardedProto := request.Header.Get("X-Forwarded-Proto"); forwardedProto != "" {
		scheme = strings.TrimSpace(strings.Split(forwardedProto, ",")[0])
	} else if request.TLS != nil {
		scheme = "https"
	}
	return (&url.URL{Scheme: scheme, Host: request.Host}).String()
}

func cleanupExpiredPlaygroundAssets(now time.Time) {
	entries, err := os.ReadDir(playgroundAssetDirectory)
	if err != nil {
		if !os.IsNotExist(err) {
			common.SysError(fmt.Sprintf("failed to read playground asset directory: %s", err.Error()))
		}
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		assetID := strings.TrimSuffix(entry.Name(), ".json")
		if _, err := uuid.Parse(assetID); err != nil {
			continue
		}

		metadata, err := loadPlaygroundAssetMetadata(assetID)
		if err == nil && metadata.ExpiresAt > now.Unix() {
			continue
		}
		if err != nil {
			info, infoErr := entry.Info()
			if infoErr != nil || now.Sub(info.ModTime()) <= playgroundAssetLifetime {
				continue
			}
		}

		if err := os.Remove(playgroundAssetPath(assetID)); err != nil && !os.IsNotExist(err) {
			common.SysError(fmt.Sprintf("failed to remove expired playground asset: %s", err.Error()))
			continue
		}
		if err := os.Remove(playgroundAssetMetadataPath(assetID)); err != nil && !os.IsNotExist(err) {
			common.SysError(fmt.Sprintf("failed to remove expired playground asset metadata: %s", err.Error()))
		}
	}

	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		assetID := entry.Name()
		if _, err := uuid.Parse(assetID); err != nil {
			continue
		}
		if _, err := os.Stat(playgroundAssetMetadataPath(assetID)); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			continue
		}
		info, err := entry.Info()
		if err != nil || now.Sub(info.ModTime()) <= playgroundAssetLifetime {
			continue
		}
		if err := os.Remove(playgroundAssetPath(assetID)); err != nil && !os.IsNotExist(err) {
			common.SysError(fmt.Sprintf("failed to remove orphaned playground asset: %s", err.Error()))
		}
	}
}

package controller

import (
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	playgroundR2Store *common.R2Store
	playgroundR2Err   error
)

const (
	maxImageAssetSize = 10 << 20
	maxVideoAssetSize = 100 << 20
	maxAudioAssetSize = 20 << 20
)

type playgroundAssetMetadata struct {
	ContentType string `json:"content_type"`
	Kind        string `json:"kind"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
}

func InitPlaygroundAssetStorage() {
	store, err := common.NewR2StoreFromEnv()
	switch {
	case err != nil:
		playgroundR2Err = err
		common.SysError(fmt.Sprintf("R2 asset storage configuration error: %s", err.Error()))
	case store == nil:
		playgroundR2Err = fmt.Errorf("R2 asset storage is not configured")
		common.SysError("R2 asset storage is not configured; playground asset uploads are disabled")
	default:
		playgroundR2Store = store
	}
}

func PlaygroundAssetUpload(c *gin.Context) {
	if playgroundR2Err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": playgroundR2Err.Error()})
		return
	}
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
	metadata := playgroundAssetMetadata{
		ContentType: contentType,
		Kind:        kind,
		Filename:    filename,
		Size:        file.Size,
	}
	key := common.R2DatePrefix(time.Now()) + assetID + safePlaygroundAssetExtension(filename)
	if err := uploadPlaygroundAssetToR2(c.Request.Context(), file, metadata, key); err != nil {
		common.SysError(fmt.Sprintf("failed to upload playground asset to R2: %s", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": "failed to save asset"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"kind":         kind,
			"url":          playgroundR2Store.URL(key),
			"filename":     filename,
			"content_type": contentType,
			"size":         file.Size,
		},
	})
}

func uploadPlaygroundAssetToR2(ctx context.Context, file *multipart.FileHeader, metadata playgroundAssetMetadata, key string) error {
	source, err := file.Open()
	if err != nil {
		return err
	}
	defer source.Close()
	return playgroundR2Store.Put(ctx, key, metadata.ContentType, source, metadata.Size)
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

func safePlaygroundAssetExtension(filename string) string {
	ext := filepath.Ext(filepath.Base(filename))
	if len(ext) < 2 || len(ext) > 16 {
		return ""
	}
	for _, r := range ext[1:] {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return ""
		}
	}
	return strings.ToLower(ext)
}

package controller

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
)

func Playground(c *gin.Context) {
	if newAPIError := preparePlaygroundRelay(c, types.RelayFormatOpenAI); newAPIError != nil {
		c.JSON(newAPIError.StatusCode, gin.H{"error": newAPIError.ToOpenAIError()})
		return
	}

	Relay(c, types.RelayFormatOpenAI)
}

func PlaygroundImage(c *gin.Context) {
	if newAPIError := preparePlaygroundRelay(c, types.RelayFormatOpenAIImage); newAPIError != nil {
		c.JSON(newAPIError.StatusCode, gin.H{"error": newAPIError.ToOpenAIError()})
		return
	}

	Relay(c, types.RelayFormatOpenAIImage)
}

func PlaygroundCreateImageTask(c *gin.Context) {
	relayMode := relayconstant.RelayModeImagesGenerations
	if strings.Contains(c.GetHeader("Content-Type"), "multipart/form-data") {
		relayMode = relayconstant.RelayModeImagesEdits
	}
	c.Set("relay_mode", relayMode)

	if newAPIError := preparePlaygroundRelay(c, types.RelayFormatOpenAIImage); newAPIError != nil {
		c.JSON(newAPIError.StatusCode, gin.H{"error": newAPIError.ToOpenAIError()})
		return
	}

	CreateImageTask(c)
}

func PlaygroundTask(c *gin.Context) {
	if newAPIError := preparePlaygroundRelay(c, types.RelayFormatTask); newAPIError != nil {
		c.JSON(newAPIError.StatusCode, gin.H{"error": newAPIError.ToOpenAIError()})
		return
	}

	RelayTask(c)
}

func PlaygroundTaskFetch(c *gin.Context) {
	if newAPIError := preparePlaygroundRelay(c, types.RelayFormatTask); newAPIError != nil {
		c.JSON(newAPIError.StatusCode, gin.H{"error": newAPIError.ToOpenAIError()})
		return
	}

	// The playground stores and polls the video task object directly, unlike
	// public task APIs which use the generic TaskResponse envelope.
	c.Set("playground_flat_video_response", true)
	RelayTaskFetch(c)
}

func preparePlaygroundRelay(c *gin.Context, relayFormat types.RelayFormat) *types.NewAPIError {
	var newAPIError *types.NewAPIError

	useAccessToken := c.GetBool("use_access_token")
	if useAccessToken {
		newAPIError = types.NewError(errors.New("暂不支持使用 access token"), types.ErrorCodeAccessDenied, types.ErrOptionWithSkipRetry())
		return newAPIError
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, nil, nil)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		return newAPIError
	}

	userId := c.GetInt("id")

	// Write user context to ensure acceptUnsetRatio is available
	userCache, err := model.GetUserCache(userId)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeQueryDataError, types.ErrOptionWithSkipRetry())
		return newAPIError
	}
	userCache.WriteContext(c)

	tempToken := &model.Token{
		UserId: userId,
		Name:   fmt.Sprintf("playground-%s", relayInfo.UsingGroup),
		Group:  relayInfo.UsingGroup,
	}
	_ = middleware.SetupContextForToken(c, tempToken)

	return nil
}

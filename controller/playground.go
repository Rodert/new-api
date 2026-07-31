package controller

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
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

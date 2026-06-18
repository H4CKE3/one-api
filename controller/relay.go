package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common"
	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/middleware"
	dbmodel "github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/monitor"
	"github.com/songquanpeng/one-api/relay/controller"
	"github.com/songquanpeng/one-api/relay/model"
	"github.com/songquanpeng/one-api/relay/relaymode"
)

// https://platform.openai.com/docs/api-reference/chat

func relayHelper(c *gin.Context, relayMode int) *model.ErrorWithStatusCode {
	var err *model.ErrorWithStatusCode
	switch relayMode {
	case relaymode.ImagesGenerations:
		err = controller.RelayImageHelper(c, relayMode)
	case relaymode.AudioSpeech:
		fallthrough
	case relaymode.AudioTranslation:
		fallthrough
	case relaymode.AudioTranscription:
		err = controller.RelayAudioHelper(c, relayMode)
	case relaymode.Proxy:
		err = controller.RelayProxyHelper(c, relayMode)
	default:
		err = controller.RelayTextHelper(c)
	}
	return err
}

func Relay(c *gin.Context) {
	ctx := c.Request.Context()
	relayMode := relaymode.GetByPath(c.Request.URL.Path)
	if config.DebugEnabled {
		requestBody, _ := common.GetRequestBody(c)
		logger.Debugf(ctx, "request body: %s", string(requestBody))
	}
	channelId := c.GetInt(ctxkey.ChannelId)
	userId := c.GetInt(ctxkey.Id)
	bizErr := relayHelper(c, relayMode)
	if bizErr == nil {
		monitor.Emit(channelId, true)
		// 更新渠道统计：成功请求
		go dbmodel.UpdateChannelRequestCount(channelId, false)
		return
	}
	lastFailedChannelId := channelId
	failedChannelHistory := []int{channelId}
	channelName := c.GetString(ctxkey.ChannelName)
	group := c.GetString(ctxkey.Group)
	originalModel := c.GetString(ctxkey.OriginalModel)
	if bizErr.StatusCode == http.StatusTooManyRequests {
		dbmodel.MarkChannelRateLimited(channelId, getRateLimitCooldown(*bizErr))
	}
	go processChannelRelayError(ctx, userId, channelId, channelName, *bizErr)
	requestId := c.GetString(helper.RequestIdKey)
	retryTimes := config.RetryTimes
	if !shouldRetry(c, bizErr.StatusCode) {
		logger.Errorf(ctx, "relay error happen, status code is %d, won't retry in this case", bizErr.StatusCode)
		retryTimes = 0
	}
	for i := retryTimes; i > 0; i-- {
		channel, err := dbmodel.CacheGetRandomSatisfiedChannel(group, originalModel, i != retryTimes)
		if err != nil {
			logger.Errorf(ctx, "CacheGetRandomSatisfiedChannel failed: %+v", err)
		}
		candidateID := 0
		if channel != nil {
			candidateID = channel.Id
		}
		selectedChannelID, err := chooseRetryChannelID(candidateID, err, lastFailedChannelId, failedChannelHistory)
		if err != nil {
			logger.Errorf(ctx, "chooseRetryChannelID failed: %+v", err)
			break
		}
		if channel == nil || channel.Id != selectedChannelID {
			channel, err = dbmodel.GetChannelById(selectedChannelID, true)
			if err != nil {
				logger.Errorf(ctx, "GetChannelById failed: %+v", err)
				break
			}
		}
		logger.Infof(ctx, "using channel #%d to retry (remain times %d)", channel.Id, i)
		attempt := retryTimes - i + 1
		delay := calculateRetryDelay(bizErr.StatusCode, attempt, rand.Intn)
		if delay > 0 {
			logger.Infof(ctx, "waiting %s before retry attempt %d", delay, attempt)
			time.Sleep(delay)
		}
		middleware.SetupContextForSelectedChannel(c, channel, originalModel)
		requestBody, err := common.GetRequestBody(c)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		bizErr = relayHelper(c, relayMode)
		if bizErr == nil {
			return
		}
		channelId := c.GetInt(ctxkey.ChannelId)
		lastFailedChannelId = channelId
		failedChannelHistory = append(failedChannelHistory, channelId)
		channelName := c.GetString(ctxkey.ChannelName)
		if bizErr.StatusCode == http.StatusTooManyRequests {
			dbmodel.MarkChannelRateLimited(channelId, getRateLimitCooldown(*bizErr))
		}
		go processChannelRelayError(ctx, userId, channelId, channelName, *bizErr)
	}
	if bizErr != nil {
		if bizErr.StatusCode == http.StatusTooManyRequests {
			bizErr.Error.Message = "当前分组上游负载已饱和，请稍后再试"
		}

		// BUG: bizErr is in race condition
		bizErr.Error.Message = helper.MessageWithRequestId(bizErr.Error.Message, requestId)
		c.JSON(bizErr.StatusCode, gin.H{
			"error": bizErr.Error,
		})
	}
}

func chooseRetryChannelID(candidateID int, selectionErr error, lastFailedChannelID int, failedChannelHistory []int) (int, error) {
	if candidateID != 0 && candidateID != lastFailedChannelID {
		return candidateID, nil
	}

	for i := len(failedChannelHistory) - 1; i >= 0; i-- {
		channelID := failedChannelHistory[i]
		if channelID == 0 || channelID == lastFailedChannelID {
			continue
		}
		return channelID, nil
	}

	if candidateID != 0 {
		return candidateID, nil
	}
	return 0, selectionErr
}

func shouldRetry(c *gin.Context, statusCode int) bool {
	if _, ok := c.Get(ctxkey.SpecificChannelId); ok {
		return false
	}
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	if statusCode/100 == 5 {
		return true
	}
	if statusCode == http.StatusBadRequest {
		return false
	}
	if statusCode/100 == 2 {
		return false
	}
	return true
}

func processChannelRelayError(ctx context.Context, userId int, channelId int, channelName string, err model.ErrorWithStatusCode) {
	logger.Errorf(ctx, "relay error (channel id %d, user id: %d): %s", channelId, userId, err.Message)
	// 更新渠道统计：错误请求
	dbmodel.UpdateChannelRequestCount(channelId, true)
	if err.StatusCode == http.StatusTooManyRequests {
		cooldown := getRateLimitCooldown(err)
		dbmodel.MarkChannelRateLimited(channelId, cooldown)
		logger.Infof(ctx, "channel #%d entered rate-limit cooldown for %s", channelId, cooldown)
	}
	// https://platform.openai.com/docs/guides/error-codes/api-errors
	if monitor.ShouldDisableChannel(&err.Error, err.StatusCode) {
		monitor.DisableChannel(channelId, channelName, err.Message)
	} else {
		monitor.Emit(channelId, false)
	}
}

func calculateRetryDelay(statusCode int, attempt int, randomIntn func(n int) int) time.Duration {
	if statusCode != http.StatusTooManyRequests && statusCode/100 != 5 {
		return 0
	}
	if attempt < 1 {
		attempt = 1
	}
	base := config.RetryBackoffBaseMilliseconds
	if base <= 0 {
		return 0
	}
	maxDelay := config.RetryBackoffMaxMilliseconds
	if maxDelay <= 0 {
		maxDelay = base
	}
	delayMs := int(math.Round(float64(base) * math.Pow(2, float64(attempt-1))))
	if delayMs > maxDelay {
		delayMs = maxDelay
	}
	jitter := config.RetryJitterMilliseconds
	if jitter > 0 {
		if randomIntn == nil {
			randomIntn = rand.Intn
		}
		delayMs += randomIntn(jitter)
	}
	return time.Duration(delayMs) * time.Millisecond
}

func getRateLimitCooldown(err model.ErrorWithStatusCode) time.Duration {
	defaultCooldown := time.Duration(config.ChannelRateLimitCooldownSeconds) * time.Second
	if defaultCooldown <= 0 {
		return 0
	}
	if seconds := findRetryAfterSeconds(err.Message); seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return defaultCooldown
}

func findRetryAfterSeconds(message string) int {
	lower := strings.ToLower(message)
	prefixes := []string{
		"retry after ",
		"retry_after:",
		"retry-after:",
	}
	for _, prefix := range prefixes {
		idx := strings.Index(lower, prefix)
		if idx < 0 {
			continue
		}
		start := idx + len(prefix)
		end := start
		for end < len(lower) && lower[end] >= '0' && lower[end] <= '9' {
			end++
		}
		if end == start {
			continue
		}
		seconds, err := strconv.Atoi(lower[start:end])
		if err == nil && seconds > 0 {
			return seconds
		}
	}
	return 0
}

func RelayNotImplemented(c *gin.Context) {
	err := model.Error{
		Message: "API not implemented",
		Type:    "one_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	err := model.Error{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

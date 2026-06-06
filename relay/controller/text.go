package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/songquanpeng/one-api/common/config"
	"github.com/songquanpeng/one-api/common/ctxkey"
	"github.com/songquanpeng/one-api/common/helper"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/model"
	"github.com/songquanpeng/one-api/relay"
	"github.com/songquanpeng/one-api/relay/adaptor"
	"github.com/songquanpeng/one-api/relay/adaptor/openai"
	"github.com/songquanpeng/one-api/relay/apitype"
	"github.com/songquanpeng/one-api/relay/billing"
	billingratio "github.com/songquanpeng/one-api/relay/billing/ratio"
	"github.com/songquanpeng/one-api/relay/channeltype"
	"github.com/songquanpeng/one-api/relay/meta"
	relaymodel "github.com/songquanpeng/one-api/relay/model"
	"github.com/songquanpeng/one-api/relay/relaymode"
)

func RelayTextHelper(c *gin.Context) *relaymodel.ErrorWithStatusCode {
	ctx := c.Request.Context()
	meta := meta.GetByContext(c)
	// get & validate textRequest
	textRequest, err := getAndValidateTextRequest(c, meta.Mode)
	if err != nil {
		logger.Errorf(ctx, "getAndValidateTextRequest failed: %s", err.Error())
		return openai.ErrorWrapper(err, "invalid_text_request", http.StatusBadRequest)
	}
	meta.IsStream = textRequest.Stream

	// map model name
	meta.OriginModelName = textRequest.Model
	textRequest.Model, _ = getMappedModelName(textRequest.Model, meta.ModelMapping)
	meta.ActualModelName = textRequest.Model

	// 创建聊天记录服务
	chatService := model.NewChatRecordService(
		meta.UserId,
		meta.TokenId,
		meta.ChannelId,
		c.GetString(ctxkey.ChannelName), // 从上下文获取渠道名称
		meta.APIType,
		meta.ActualModelName,
		c.GetString(helper.RequestIdKey), // 从上下文获取请求ID
	)

	// 保存用户消息到聊天记录
	if meta.Mode == relaymode.ChatCompletions && len(textRequest.Messages) > 0 {
		for _, msg := range textRequest.Messages {
			savePromptMessage(ctx, chatService, msg.Role, msg.StringContent())
		}
	}

	// set system prompt if not empty
	systemPromptReset := setSystemPrompt(ctx, textRequest, meta.ForcedSystemPrompt)
	// get model ratio & group ratio
	modelRatio := billingratio.GetModelRatio(textRequest.Model, meta.ChannelType)
	groupRatio := billingratio.GetGroupRatio(meta.Group)
	ratio := modelRatio * groupRatio
	// pre-consume quota
	promptTokens := getPromptTokens(textRequest, meta.Mode)
	meta.PromptTokens = promptTokens
	preConsumedQuota, bizErr := preConsumeQuota(ctx, textRequest, promptTokens, ratio, groupRatio, meta)
	if bizErr != nil {
		logger.Warnf(ctx, "preConsumeQuota failed: %+v", *bizErr)
		return bizErr
	}

	adaptor := relay.GetAdaptor(meta.APIType)
	if adaptor == nil {
		return openai.ErrorWrapper(fmt.Errorf("invalid api type: %d", meta.APIType), "invalid_api_type", http.StatusBadRequest)
	}
	adaptor.Init(meta)

	// get request body
	requestBody, err := getRequestBody(c, meta, textRequest, adaptor)
	if err != nil {
		return openai.ErrorWrapper(err, "convert_request_failed", http.StatusInternalServerError)
	}

	// do request
	resp, err := adaptor.DoRequest(c, meta, requestBody)
	if err != nil {
		logger.Errorf(ctx, "DoRequest failed: %s", err.Error())
		bizErr := openai.ErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
		billing.ReturnPreConsumedQuota(ctx, preConsumedQuota, meta.TokenId)
		recordFailedConsumeLog(ctx, bizErr, meta, textRequest, modelRatio, groupRatio, systemPromptReset)

		// 保存失败的聊天记录
		if meta.Mode == relaymode.ChatCompletions {
			errorMsg := fmt.Sprintf("StatusCode: %d, Type: %s, Code: %s, Param: %s, Message: %s",
				bizErr.StatusCode, bizErr.Error.Type, bizErr.Error.Code, bizErr.Error.Param, bizErr.Error.Message)
			model.SaveChatRecordAsync(chatService, "", model.ChatRoleAssistant, 0, 0, 0, model.ChatRecordStatusFailed, errorMsg)
		}

		return bizErr
	}
	if isErrorHappened(meta, resp) {
		billing.ReturnPreConsumedQuota(ctx, preConsumedQuota, meta.TokenId)
		relayErr := RelayErrorHandler(resp)
		recordFailedConsumeLog(ctx, relayErr, meta, textRequest, modelRatio, groupRatio, systemPromptReset)

		// 保存失败的聊天记录
		if meta.Mode == relaymode.ChatCompletions {
			errorMsg := fmt.Sprintf("StatusCode: %d, Type: %s, Code: %s, Param: %s, Message: %s",
				relayErr.StatusCode, relayErr.Error.Type, relayErr.Error.Code, relayErr.Error.Param, relayErr.Error.Message)
			model.SaveChatRecordAsync(chatService, "", model.ChatRoleAssistant, 0, 0, 0, model.ChatRecordStatusFailed, errorMsg)
		}

		return relayErr
	}

	// do response
	usage, responseText, respErr := adaptor.DoResponse(c, resp, meta)
	if respErr != nil {
		logger.Errorf(ctx, "respErr is not nil: %+v", respErr)
		billing.ReturnPreConsumedQuota(ctx, preConsumedQuota, meta.TokenId)
		recordFailedConsumeLog(ctx, respErr, meta, textRequest, modelRatio, groupRatio, systemPromptReset)

		// 保存失败的聊天记录
		if meta.Mode == relaymode.ChatCompletions {
			errorMsg := ""
			if respErr != nil {
				// 将详细错误信息合并到 error_message
				errorMsg = fmt.Sprintf("StatusCode: %d, Type: %s, Code: %s, Param: %s, Message: %s",
					respErr.StatusCode, respErr.Error.Type, respErr.Error.Code, respErr.Error.Param, respErr.Error.Message)
			}
			model.SaveChatRecordAsync(chatService, "", model.ChatRoleAssistant, int(usage.PromptTokens), int(usage.CompletionTokens), int(usage.TotalTokens), model.ChatRecordStatusFailed, errorMsg)
		}

		return respErr
	}

	// 保存成功的助手回复到聊天记录
	if meta.Mode == relaymode.ChatCompletions {
		// 使用实际的AI回复内容
		model.SaveChatRecordAsync(chatService, responseText, model.ChatRoleAssistant, int(usage.PromptTokens), int(usage.CompletionTokens), int(usage.TotalTokens), model.ChatRecordStatusSuccess, "")
	}

	// post-consume quota
	go postConsumeQuota(ctx, usage, meta, textRequest, ratio, preConsumedQuota, modelRatio, groupRatio, systemPromptReset)
	return nil
}

func savePromptMessage(ctx context.Context, chatService *model.ChatRecordService, role string, content string) {
	content = strings.TrimSpace(content)
	if content == "" {
		return
	}

	var err error
	switch role {
	case model.ChatRoleSystem:
		err = chatService.SaveSystemMessage(content)
	case model.ChatRoleDeveloper:
		err = chatService.SaveDeveloperMessage(content)
	case model.ChatRoleAssistant:
		err = chatService.SaveAssistantMessage(content, 0, 0, 0, model.ChatRecordStatusSuccess, "")
	case model.ChatRoleUser:
		err = chatService.SaveUserMessage(content, model.ChatRoleUser)
	default:
		err = chatService.SaveUserMessage(content, role)
	}
	if err != nil {
		logger.Errorf(ctx, "save prompt chat record failed: %s", err.Error())
	}
}

func recordFailedConsumeLog(ctx context.Context, err *relaymodel.ErrorWithStatusCode, meta *meta.Meta, textRequest *relaymodel.GeneralOpenAIRequest, modelRatio float64, groupRatio float64, systemPromptReset bool) {
	if err == nil {
		return
	}
	logContent := fmt.Sprintf("请求失败：%s；倍率：%.2f × %.2f", err.Error.Message, modelRatio, groupRatio)
	model.RecordConsumeLog(ctx, &model.Log{
		UserId:            meta.UserId,
		ChannelId:         meta.ChannelId,
		PromptTokens:      meta.PromptTokens,
		CompletionTokens:  0,
		ModelName:         textRequest.Model,
		TokenName:         meta.TokenName,
		Quota:             0,
		Content:           logContent,
		IsStream:          meta.IsStream,
		ElapsedTime:       helper.CalcElapsedTime(meta.StartTime),
		SystemPromptReset: systemPromptReset,
	})
}

func getRequestBody(c *gin.Context, meta *meta.Meta, textRequest *relaymodel.GeneralOpenAIRequest, adaptor adaptor.Adaptor) (io.Reader, error) {
	if !config.EnforceIncludeUsage &&
		meta.APIType == apitype.OpenAI &&
		meta.OriginModelName == meta.ActualModelName &&
		meta.ChannelType != channeltype.Baichuan &&
		meta.ForcedSystemPrompt == "" {
		// no need to convert request for openai
		return c.Request.Body, nil
	}

	// get request body
	var requestBody io.Reader
	convertedRequest, err := adaptor.ConvertRequest(c, meta.Mode, textRequest)
	if err != nil {
		logger.Debugf(c.Request.Context(), "converted request failed: %s\n", err.Error())
		return nil, err
	}
	jsonData, err := json.Marshal(convertedRequest)
	if err != nil {
		logger.Debugf(c.Request.Context(), "converted request json_marshal_failed: %s\n", err.Error())
		return nil, err
	}
	logger.Debugf(c.Request.Context(), "converted request: \n%s", string(jsonData))
	requestBody = bytes.NewBuffer(jsonData)
	return requestBody, nil
}

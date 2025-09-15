package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/songquanpeng/one-api/common/config"
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
		"", // ChannelName 暂时为空，可以从渠道配置中获取
		meta.APIType,
		meta.ActualModelName,
		"", // RequestId 暂时为空
	)
	
	// 保存用户消息到聊天记录
	if meta.Mode == relaymode.ChatCompletions && len(textRequest.Messages) > 0 {
		// 解析用户消息和系统消息
		var userContent, systemContent strings.Builder
		for _, msg := range textRequest.Messages {
			switch msg.Role {
			case model.ChatRoleUser:
				userContent.WriteString(msg.StringContent())
				userContent.WriteString("\n")
			case model.ChatRoleSystem:
				systemContent.WriteString(msg.StringContent())
				systemContent.WriteString("\n")
			}
		}
		
		// 保存系统消息
		if systemContent.Len() > 0 {
			model.SaveChatRecordAsync(chatService, strings.TrimSpace(systemContent.String()), model.ChatRoleSystem, 0, 0, 0, model.ChatRecordStatusSuccess, "")
		}
		
		// 保存用户消息
		if userContent.Len() > 0 {
			model.SaveChatRecordAsync(chatService, strings.TrimSpace(userContent.String()), model.ChatRoleUser, 0, 0, 0, model.ChatRecordStatusSuccess, "")
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
	preConsumedQuota, bizErr := preConsumeQuota(ctx, textRequest, promptTokens, ratio, meta)
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
		return openai.ErrorWrapper(err, "do_request_failed", http.StatusInternalServerError)
	}
	if isErrorHappened(meta, resp) {
		billing.ReturnPreConsumedQuota(ctx, preConsumedQuota, meta.TokenId)
		return RelayErrorHandler(resp)
	}

	// do response
	usage, respErr := adaptor.DoResponse(c, resp, meta)
	if respErr != nil {
		logger.Errorf(ctx, "respErr is not nil: %+v", respErr)
		billing.ReturnPreConsumedQuota(ctx, preConsumedQuota, meta.TokenId)
		
		// 保存失败的聊天记录
		if meta.Mode == relaymode.ChatCompletions {
			errorMsg := ""
			if respErr != nil {
				errorMsg = respErr.Message
			}
			model.SaveChatRecordAsync(chatService, "", model.ChatRoleAssistant, int(usage.PromptTokens), int(usage.CompletionTokens), int(usage.TotalTokens), model.ChatRecordStatusFailed, errorMsg)
		}
		
		return respErr
	}
	
	// 保存成功的助手回复到聊天记录
	if meta.Mode == relaymode.ChatCompletions {
		// 注意：这里我们无法直接获取响应内容，因为DoResponse已经将内容发送给客户端
		// 在实际应用中，可能需要修改DoResponse方法来返回响应内容
		model.SaveChatRecordAsync(chatService, "[响应已发送]", model.ChatRoleAssistant, int(usage.PromptTokens), int(usage.CompletionTokens), int(usage.TotalTokens), model.ChatRecordStatusSuccess, "")
	}
	
	// post-consume quota
	go postConsumeQuota(ctx, usage, meta, textRequest, ratio, preConsumedQuota, modelRatio, groupRatio, systemPromptReset)
	return nil
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

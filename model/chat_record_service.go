package model

import (
	"crypto/md5"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/songquanpeng/one-api/common/logger"
)

// ChatRecordService 聊天记录服务
type ChatRecordService struct {
	conversationId string
	userId         int
	tokenId        int
	channelId      int
	channelName    string
	apiType        int
	model          string
	requestId      string
	startTime      int64
}

// NewChatRecordService 创建聊天记录服务
func NewChatRecordService(userId, tokenId, channelId int, channelName string, apiType int, model, requestId string) *ChatRecordService {
	// 生成会话ID，使用用户ID+时间戳+随机数
	conversationId := generateConversationId(userId)
	
	return &ChatRecordService{
		conversationId: conversationId,
		userId:         userId,
		tokenId:        tokenId,
		channelId:      channelId,
		channelName:    channelName,
		apiType:        apiType,
		model:          model,
		requestId:      requestId,
		startTime:      time.Now().Unix(),
	}
}

// generateConversationId 生成会话ID
func generateConversationId(userId int) string {
	timestamp := time.Now().Unix()
	randomNum := rand.Intn(9000) + 1000 // 生成1000-9999之间的随机数
	hash := md5.Sum([]byte(fmt.Sprintf("%d_%d_%d", userId, timestamp, randomNum)))
	return fmt.Sprintf("%x", hash)[:16]
}

// SaveUserMessage 保存用户消息
func (crs *ChatRecordService) SaveUserMessage(content string, role string) error {
	if content == "" {
		return nil
	}
	
	record := &ChatRecord{
		UserId:         crs.userId,
		TokenId:        crs.tokenId,
		ConversationId: crs.conversationId,
		Role:           role,
		Content:        strings.TrimSpace(content),
		Model:          crs.model,
		ChannelId:      crs.channelId,
		ChannelName:    crs.channelName,
		ApiType:        crs.apiType,
		RequestId:      crs.requestId,
		Status:         ChatRecordStatusSuccess,
	}
	
	return record.Insert()
}

// SaveAssistantMessage 保存助手回复
func (crs *ChatRecordService) SaveAssistantMessage(content string, promptTokens, completionTokens, totalTokens int, status int, errorMsg string) error {
	record := &ChatRecord{
		UserId:           crs.userId,
		TokenId:          crs.tokenId,
		ConversationId:   crs.conversationId,
		Role:             ChatRoleAssistant,
		Content:          content,
		Model:            crs.model,
		ChannelId:        crs.channelId,
		ChannelName:      crs.channelName,
		ApiType:          crs.apiType,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		TotalTokens:      totalTokens,
		RequestId:        crs.requestId,
		ResponseTime:     int((time.Now().Unix() - crs.startTime) * 1000), // 转换为毫秒
		Status:           status,
		ErrorMessage:     errorMsg,
		Cost:             int64(totalTokens), // 简化计算，1 token = 1 额度
	}
	
	return record.Insert()
}

// SaveSystemMessage 保存系统消息
func (crs *ChatRecordService) SaveSystemMessage(content string) error {
	record := &ChatRecord{
		UserId:         crs.userId,
		TokenId:        crs.tokenId,
		ConversationId: crs.conversationId,
		Role:           ChatRoleSystem,
		Content:        content,
		Model:          crs.model,
		ChannelId:      crs.channelId,
		ChannelName:    crs.channelName,
		ApiType:        crs.apiType,
		RequestId:      crs.requestId,
		Status:         ChatRecordStatusSuccess,
	}
	
	return record.Insert()
}

// SaveDeveloperMessage 保存开发者消息
func (crs *ChatRecordService) SaveDeveloperMessage(content string) error {
	record := &ChatRecord{
		UserId:         crs.userId,
		TokenId:        crs.tokenId,
		ConversationId: crs.conversationId,
		Role:           ChatRoleDeveloper,
		Content:        content,
		Model:          crs.model,
		ChannelId:      crs.channelId,
		ChannelName:    crs.channelName,
		ApiType:        crs.apiType,
		RequestId:      crs.requestId,
		Status:         ChatRecordStatusSuccess,
	}
	
	return record.Insert()
}

// GetConversationId 获取会话ID
func (crs *ChatRecordService) GetConversationId() string {
	return crs.conversationId
}

// SetConversationId 设置会话ID（用于继续之前的对话）
func (crs *ChatRecordService) SetConversationId(conversationId string) {
	crs.conversationId = conversationId
}

// SaveChatRecordAsync 异步保存聊天记录
func SaveChatRecordAsync(service *ChatRecordService, content string, role string, promptTokens, completionTokens, totalTokens int, status int, errorMsg string) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SysError(fmt.Sprintf("SaveChatRecordAsync panic: %v", r))
			}
		}()
		
		var err error
		switch role {
		case ChatRoleUser, ChatRoleSystem:
			err = service.SaveUserMessage(content, role)
		case ChatRoleAssistant:
			err = service.SaveAssistantMessage(content, promptTokens, completionTokens, totalTokens, status, errorMsg)
		case ChatRoleDeveloper:
			err = service.SaveDeveloperMessage(content)
		}
		
		if err != nil {
			logger.SysError(fmt.Sprintf("Failed to save chat record: %v", err))
		}
	}()
}

// ParseMessagesFromRequest 从请求中解析消息（简化版，返回字符串内容）
func ParseMessagesFromRequest(messages interface{}) (userContent, systemContent string) {
	// 这里需要根据实际的消息结构来解析
	// 暂时返回空字符串，具体实现需要在调用处处理
	return "", ""
}

// ExtractContentFromMessages 从消息列表中提取内容（简化版）
func ExtractContentFromMessages(messages interface{}) string {
	// 这里需要根据实际的消息结构来解析
	// 暂时返回空字符串，具体实现需要在调用处处理
	return ""
}

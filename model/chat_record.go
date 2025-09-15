package model

import (
	"time"
)

// ChatRecord 聊天记录表
type ChatRecord struct {
	Id               int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId           int    `json:"user_id" gorm:"not null;index"`                 // 用户ID
	TokenId          int    `json:"token_id" gorm:"not null;index"`                // 使用的Token ID
	ConversationId   string `json:"conversation_id" gorm:"type:varchar(64);index"` // 会话ID，用于关联同一轮对话
	Role             string `json:"role" gorm:"type:varchar(20);not null"`         // 角色：user, assistant, system, developer等
	Content          string `json:"content" gorm:"type:text;not null"`             // 消息内容
	Model            string `json:"model" gorm:"type:varchar(100);not null"`       // 使用的模型名称
	ChannelId        int    `json:"channel_id" gorm:"not null;index"`              // 渠道ID
	ChannelName      string `json:"channel_name" gorm:"type:varchar(100)"`         // 渠道名称
	ApiType          int    `json:"api_type" gorm:"not null"`                      // API类型
	PromptTokens     int    `json:"prompt_tokens" gorm:"default:0"`                // 输入token数
	CompletionTokens int    `json:"completion_tokens" gorm:"default:0"`            // 输出token数
	TotalTokens      int    `json:"total_tokens" gorm:"default:0"`                 // 总token数
	Cost             int64  `json:"cost" gorm:"default:0"`                         // 消耗的额度
	RequestId        string `json:"request_id" gorm:"type:varchar(128);index"`     // 请求ID，用于追踪
	ResponseTime     int    `json:"response_time" gorm:"default:0"`                // 响应时间(毫秒)
	Status           int    `json:"status" gorm:"default:1"`                       // 状态：1-成功，2-失败，3-部分成功
	ErrorMessage     string `json:"error_message" gorm:"type:text"`                // 错误信息
	CreatedTime      int64  `json:"created_time" gorm:"bigint;not null;index"`     // 创建时间
	UpdatedTime      int64  `json:"updated_time" gorm:"bigint;not null"`           // 更新时间
}

// 聊天记录状态常量
const (
	ChatRecordStatusSuccess = 1 // 成功
	ChatRecordStatusFailed  = 2 // 失败
)

// 角色常量
const (
	ChatRoleUser      = "user"
	ChatRoleAssistant = "assistant"
	ChatRoleSystem    = "system"
	ChatRoleDeveloper = "developer"
)

// Insert 插入聊天记录
func (cr *ChatRecord) Insert() error {
	cr.CreatedTime = time.Now().Unix()
	cr.UpdatedTime = cr.CreatedTime
	return DB.Create(cr).Error
}

// Update 更新聊天记录
func (cr *ChatRecord) Update() error {
	cr.UpdatedTime = time.Now().Unix()
	return DB.Model(cr).Updates(cr).Error
}

// GetChatRecordsByUserId 根据用户ID获取聊天记录
func GetChatRecordsByUserId(userId int, startIdx int, num int, conversationId string) ([]*ChatRecord, error) {
	var records []*ChatRecord
	query := DB.Where("user_id = ?", userId)

	if conversationId != "" {
		query = query.Where("conversation_id = ?", conversationId)
	}

	err := query.Order("created_time desc").Limit(num).Offset(startIdx).Find(&records).Error
	return records, err
}

// GetChatRecordsByConversationId 根据会话ID获取聊天记录
func GetChatRecordsByConversationId(conversationId string) ([]*ChatRecord, error) {
	var records []*ChatRecord
	err := DB.Where("conversation_id = ?", conversationId).Order("created_time asc").Find(&records).Error
	return records, err
}

// GetChatRecordById 根据ID获取聊天记录
func GetChatRecordById(id int) (*ChatRecord, error) {
	var record ChatRecord
	err := DB.Where("id = ?", id).First(&record).Error
	return &record, err
}

// DeleteChatRecordById 根据ID删除聊天记录
func DeleteChatRecordById(id int, userId int) error {
	return DB.Where("id = ? AND user_id = ?", id, userId).Delete(&ChatRecord{}).Error
}

// DeleteChatRecordsByConversationId 根据会话ID删除聊天记录
func DeleteChatRecordsByConversationId(conversationId string, userId int) error {
	return DB.Where("conversation_id = ? AND user_id = ?", conversationId, userId).Delete(&ChatRecord{}).Error
}

// GetChatRecordStats 获取聊天记录统计信息
func GetChatRecordStats(userId int, startTime int64, endTime int64) (map[string]interface{}, error) {
	var stats struct {
		TotalRecords       int64 `json:"total_records"`
		TotalTokens        int64 `json:"total_tokens"`
		TotalCost          int64 `json:"total_cost"`
		TotalConversations int64 `json:"total_conversations"`
	}

	query := DB.Model(&ChatRecord{}).Where("user_id = ?", userId)
	if startTime > 0 {
		query = query.Where("created_time >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("created_time <= ?", endTime)
	}

	err := query.Select("COUNT(*) as total_records, SUM(total_tokens) as total_tokens, SUM(cost) as total_cost").Scan(&stats).Error
	if err != nil {
		return nil, err
	}

	// 获取会话数量
	var conversationCount int64
	err = query.Select("COUNT(DISTINCT conversation_id)").Scan(&conversationCount).Error
	if err != nil {
		return nil, err
	}
	stats.TotalConversations = conversationCount

	return map[string]interface{}{
		"total_records":       stats.TotalRecords,
		"total_tokens":        stats.TotalTokens,
		"total_cost":          stats.TotalCost,
		"total_conversations": stats.TotalConversations,
	}, nil
}

// SearchChatRecords 搜索聊天记录
func SearchChatRecords(userId int, keyword string, startIdx int, num int) ([]*ChatRecord, error) {
	var records []*ChatRecord
	query := DB.Where("user_id = ?", userId)

	if keyword != "" {
		query = query.Where("content LIKE ? OR model LIKE ? OR channel_name LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}

	err := query.Order("created_time desc").Limit(num).Offset(startIdx).Find(&records).Error
	return records, err
}

// GetRecentConversations 获取用户最近的会话列表
func GetRecentConversations(userId int, limit int) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	err := DB.Model(&ChatRecord{}).
		Select("conversation_id, MAX(created_time) as last_time, COUNT(*) as message_count").
		Where("user_id = ?", userId).
		Group("conversation_id").
		Order("last_time desc").
		Limit(limit).
		Find(&results).Error

	return results, err
}

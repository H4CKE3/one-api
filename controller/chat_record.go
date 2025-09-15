package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/common/logger"
	"github.com/songquanpeng/one-api/model"
)

// GetChatRecords 获取聊天记录
func GetChatRecords(c *gin.Context) {
	userId := c.GetInt("id")
	startIdx, _ := strconv.Atoi(c.Query("start_idx"))
	num, _ := strconv.Atoi(c.Query("num"))
	conversationId := c.Query("conversation_id")

	if num <= 0 {
		num = 20
	}
	if startIdx < 0 {
		startIdx = 0
	}

	records, err := model.GetChatRecordsByUserId(userId, startIdx, num, conversationId)
	if err != nil {
		logger.Errorf(c.Request.Context(), "failed to get chat records: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取聊天记录失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    records,
	})
}

// GetChatRecordById 根据ID获取聊天记录
func GetChatRecordById(c *gin.Context) {
	userId := c.GetInt("id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的记录ID",
		})
		return
	}

	record, err := model.GetChatRecordById(id)
	if err != nil {
		logger.Errorf(c.Request.Context(), "failed to get chat record: %v", err)
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "聊天记录不存在",
		})
		return
	}

	// 检查权限
	if record.UserId != userId {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "无权访问此记录",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    record,
	})
}

// GetConversationRecords 获取会话的所有记录
func GetConversationRecords(c *gin.Context) {
	userId := c.GetInt("id")
	conversationId := c.Param("conversation_id")

	if conversationId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "会话ID不能为空",
		})
		return
	}

	records, err := model.GetChatRecordsByConversationId(conversationId)
	if err != nil {
		logger.Errorf(c.Request.Context(), "failed to get conversation records: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取会话记录失败",
		})
		return
	}

	// 检查权限 - 确保用户只能访问自己的记录
	var userRecords []*model.ChatRecord
	for _, record := range records {
		if record.UserId == userId {
			userRecords = append(userRecords, record)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    userRecords,
	})
}

// DeleteChatRecord 删除聊天记录
func DeleteChatRecord(c *gin.Context) {
	userId := c.GetInt("id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "无效的记录ID",
		})
		return
	}

	err = model.DeleteChatRecordById(id, userId)
	if err != nil {
		logger.Errorf(c.Request.Context(), "failed to delete chat record: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除聊天记录失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}

// DeleteConversation 删除整个会话
func DeleteConversation(c *gin.Context) {
	userId := c.GetInt("id")
	conversationId := c.Param("conversation_id")

	if conversationId == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "会话ID不能为空",
		})
		return
	}

	err := model.DeleteChatRecordsByConversationId(conversationId, userId)
	if err != nil {
		logger.Errorf(c.Request.Context(), "failed to delete conversation: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "删除会话失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "删除成功",
	})
}

// GetChatRecordStats 获取聊天记录统计
func GetChatRecordStats(c *gin.Context) {
	userId := c.GetInt("id")
	startTimeStr := c.Query("start_time")
	endTimeStr := c.Query("end_time")

	var startTime, endTime int64
	var err error

	if startTimeStr != "" {
		startTime, err = strconv.ParseInt(startTimeStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "无效的开始时间",
			})
			return
		}
	}

	if endTimeStr != "" {
		endTime, err = strconv.ParseInt(endTimeStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "无效的结束时间",
			})
			return
		}
	}

	stats, err := model.GetChatRecordStats(userId, startTime, endTime)
	if err != nil {
		logger.Errorf(c.Request.Context(), "failed to get chat record stats: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取统计信息失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    stats,
	})
}

// SearchChatRecords 搜索聊天记录
func SearchChatRecords(c *gin.Context) {
	userId := c.GetInt("id")
	keyword := c.Query("keyword")
	startIdx, _ := strconv.Atoi(c.Query("start_idx"))
	num, _ := strconv.Atoi(c.Query("num"))

	if num <= 0 {
		num = 20
	}
	if startIdx < 0 {
		startIdx = 0
	}

	records, err := model.SearchChatRecords(userId, keyword, startIdx, num)
	if err != nil {
		logger.Errorf(c.Request.Context(), "failed to search chat records: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "搜索聊天记录失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    records,
	})
}

// GetRecentConversations 获取最近的会话列表
func GetRecentConversations(c *gin.Context) {
	userId := c.GetInt("id")
	limit, _ := strconv.Atoi(c.Query("limit"))

	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	conversations, err := model.GetRecentConversations(userId, limit)
	if err != nil {
		logger.Errorf(c.Request.Context(), "failed to get recent conversations: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "获取最近会话失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    conversations,
	})
}

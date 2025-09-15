package router

import (
	"github.com/gin-gonic/gin"
	"github.com/songquanpeng/one-api/controller"
	"github.com/songquanpeng/one-api/middleware"
)

// SetChatRecordRouter 设置聊天记录相关路由
func SetChatRecordRouter(router *gin.RouterGroup) {
	// 聊天记录管理路由组
	chatRecordGroup := router.Group("/chat-records")
	chatRecordGroup.Use(middleware.UserAuth())
	{
		// 获取聊天记录列表
		chatRecordGroup.GET("", controller.GetChatRecords)
		
		// 根据ID获取聊天记录
		chatRecordGroup.GET("/:id", controller.GetChatRecordById)
		
		// 获取会话的所有记录
		chatRecordGroup.GET("/conversation/:conversation_id", controller.GetConversationRecords)
		
		// 删除聊天记录
		chatRecordGroup.DELETE("/:id", controller.DeleteChatRecord)
		
		// 删除整个会话
		chatRecordGroup.DELETE("/conversation/:conversation_id", controller.DeleteConversation)
		
		// 获取聊天记录统计
		chatRecordGroup.GET("/stats", controller.GetChatRecordStats)
		
		// 搜索聊天记录
		chatRecordGroup.GET("/search", controller.SearchChatRecords)
		
		// 获取最近的会话列表
		chatRecordGroup.GET("/conversations", controller.GetRecentConversations)
	}
}

package openapi

import "github.com/gin-gonic/gin"

// Module 聚合开放 API HTTP 处理器。
type Module struct {
	Handler *Handler
}

// NewModule 创建模块。
func NewModule(handler *Handler) *Module {
	return &Module{Handler: handler}
}

// RegisterRoutes 注册登录用户侧 API Key 管理接口。
func (m *Module) RegisterRoutes(authRequired *gin.RouterGroup) {
	if m == nil || m.Handler == nil {
		return
	}
	authRequired.GET("/user/openapi-key", m.Handler.GetAPIKey)
	authRequired.POST("/user/openapi-key", m.Handler.CreateAPIKey)
	authRequired.POST("/user/openapi-key/regenerate", m.Handler.RegenerateAPIKey)
	authRequired.DELETE("/user/openapi-key", m.Handler.DeleteAPIKey)
}

// RegisterCompatibleRoutes 注册根路径 /v1 兼容接口。
func (m *Module) RegisterCompatibleRoutes(v1 *gin.RouterGroup) {
	if m == nil || m.Handler == nil {
		return
	}
	v1.GET("/models", m.Handler.ListModels)
	v1.POST("/chat/completions", m.Handler.ChatCompletions)
}

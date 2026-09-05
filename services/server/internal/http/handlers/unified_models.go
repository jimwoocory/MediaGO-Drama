package handlers

import (
	"github.com/gin-gonic/gin"
	httpresponse "github.com/mediago-dev/mediago-drama/services/server/internal/http/response"
	service "github.com/mediago-dev/mediago-drama/services/server/internal/service/settings"
	"net/http"
)

// HandleUnifiedModels returns discovered output models sharing the unified credential.
// @Summary 获取统一接口生成模型
// @Tags Settings
// @Produce json
// @Success 200 {object} SwaggerEnvelope
// @Router /api/v1/settings/unified-models [get]
func (handler Settings) HandleUnifiedModels(ctx *gin.Context) {
	result, err := handler.service.ListUnifiedModels(ctx.Request.Context(), ctx.Query("refresh") == "true")
	if err != nil {
		writeSettingsError(ctx, err)
		return
	}
	httpresponse.OK(ctx, result)
}

// HandlePutUnifiedModel saves a model protocol override, never another API key.
// @Summary 保存统一接口模型协议
// @Tags Settings
// @Accept json
// @Produce json
// @Param body body SwaggerUnifiedModelRequest true "模型配置"
// @Success 200 {object} SwaggerEnvelope
// @Router /api/v1/settings/unified-models [put]
func (handler Settings) HandlePutUnifiedModel(ctx *gin.Context) {
	input, err := decodeJSON[service.UnifiedModel](ctx)
	if err != nil {
		httpresponse.ErrorFromStatus(ctx, http.StatusBadRequest, err)
		return
	}
	result, err := handler.service.SetUnifiedModel(ctx.Request.Context(), input)
	if err != nil {
		writeSettingsError(ctx, err)
		return
	}
	httpresponse.OK(ctx, result)
}

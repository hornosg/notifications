package controller

import (
	"fmt"
	"net/http"

	"notifications/src/notification/application/port"
	"notifications/src/notification/application/request"
	"notifications/src/notification/application/usecase"
	"notifications/src/shared/logger"
	"notifications/src/shared/middleware"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type NotificationHandler struct {
	sendNotificationUseCase  port.SendNotificationUseCase
	getNotificationUseCase   port.GetNotificationUseCase
	listNotificationsUseCase port.ListNotificationsUseCase
}

func NewNotificationHandler(
	sendNotificationUseCase port.SendNotificationUseCase,
	getNotificationUseCase port.GetNotificationUseCase,
	listNotificationsUseCase port.ListNotificationsUseCase,
) *NotificationHandler {
	return &NotificationHandler{
		sendNotificationUseCase:  sendNotificationUseCase,
		getNotificationUseCase:   getNotificationUseCase,
		listNotificationsUseCase: listNotificationsUseCase,
	}
}

func (handler *NotificationHandler) SendNotification(ctx *gin.Context) {
	log := logger.GetLogger()

	var req request.SendNotificationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		log.Error("Binding error details",
			zap.Error(err),
			zap.String("error_type", fmt.Sprintf("%T", err)),
			zap.String("error_string", err.Error()))
		middleware.AbortWithBusinessError(ctx, middleware.ErrInvalidRequestFormat)
		return
	}

	log.Info("Request bound successfully", zap.String("type", req.Type), zap.String("action", req.Action))

	// Namespace/tenant para embeber en la entidad — la conexión ya viene RLS-scoped desde
	// database.TenantSession; esto solo arma el appctx que usan los use cases (ver
	// shared/middleware/rls.go: namespace SIEMPRE del JWT, nunca de un header).
	reqCtx := middleware.ContextWithRLSFromGin(ctx)

	result := handler.sendNotificationUseCase.Execute(reqCtx, &req)
	if !result.Success {
		middleware.AbortWithBusinessError(ctx, result.ToBusinessError())
		return
	}

	ctx.JSON(http.StatusOK, result.Data)
}

func (handler *NotificationHandler) GetNotificationStatus(ctx *gin.Context) {
	notificationID := ctx.Param("id")
	if notificationID == "" {
		middleware.AbortWithBusinessError(ctx, middleware.BusinessError{
			Code:       "MISSING_NOTIFICATION_ID",
			Message:    "ID de notificación requerido",
			HTTPStatus: http.StatusBadRequest,
		})
		return
	}

	reqCtx := middleware.ContextWithRLSFromGin(ctx)
	response, err := handler.getNotificationUseCase.Execute(reqCtx, notificationID)
	if err != nil {
		switch err {
		case usecase.ErrNotificationNotFound:
			middleware.AbortWithBusinessError(ctx, middleware.ErrNotificationNotFound)
		default:
			middleware.AbortWithError(ctx, err)
		}
		return
	}

	ctx.JSON(http.StatusOK, response)
}

func (handler *NotificationHandler) ListNotifications(ctx *gin.Context) {
	log := logger.GetLogger()

	var listRequest request.ListNotificationsRequest
	if err := ctx.ShouldBindQuery(&listRequest); err != nil {
		log.Error("Error binding query parameters", zap.Error(err))
		middleware.AbortWithBusinessError(ctx, middleware.ErrInvalidRequestFormat)
		return
	}

	log.Info("List notifications request",
		zap.String("type", listRequest.Type),
		zap.String("action", listRequest.Action),
		zap.String("status", listRequest.Status),
		zap.Int("page", listRequest.Page),
		zap.Int("limit", listRequest.Limit))

	reqCtx := middleware.ContextWithRLSFromGin(ctx)
	response, err := handler.listNotificationsUseCase.Execute(reqCtx, &listRequest)
	if err != nil {
		log.Error("Error executing list notifications use case", zap.Error(err))

		if validationErr, ok := err.(request.ValidationError); ok {
			middleware.AbortWithBusinessError(ctx, middleware.BusinessError{
				Code:       "VALIDATION_ERROR",
				Message:    validationErr.Error(),
				HTTPStatus: http.StatusBadRequest,
			})
			return
		}

		middleware.AbortWithError(ctx, err)
		return
	}

	ctx.JSON(http.StatusOK, response)
}

// RegisterRoutes registra las rutas del módulo notifications.
func (handler *NotificationHandler) RegisterRoutes(router *gin.RouterGroup) {
	notificationsGroup := router.Group("/notifications")
	{
		notificationsGroup.POST("", handler.SendNotification)
		notificationsGroup.GET("", handler.ListNotifications)
		notificationsGroup.GET("/:id", handler.GetNotificationStatus)
	}
}

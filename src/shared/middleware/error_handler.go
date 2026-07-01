package middleware

import (
	"net/http"

	"notifications/src/shared/apperror"
	"notifications/src/shared/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Alias reexportados para no acoplar los controllers directamente a shared/apperror.
type ErrorResponse = apperror.ErrorResponse
type Error = apperror.Error
type BusinessError = apperror.BusinessError

var (
	ErrInvalidRequestFormat = apperror.ErrInvalidRequestFormat
	ErrInvalidEmail         = apperror.ErrInvalidEmail
	ErrTemplateNotFound     = apperror.ErrTemplateNotFound
	ErrNotificationNotFound = apperror.ErrNotificationNotFound
	ErrInternalServer       = apperror.ErrInternalServer
)

// ErrorHandlerMiddleware centraliza el mapeo de errores de negocio a respuesta HTTP.
func ErrorHandlerMiddleware() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()
		if len(ctx.Errors) > 0 {
			handleError(ctx, ctx.Errors.Last().Err)
		}
	}
}

func handleError(ctx *gin.Context, err error) {
	log := logger.GetLogger()

	switch e := err.(type) {
	case apperror.BusinessError:
		log.Warn("Business error occurred",
			zap.String("code", e.Code),
			zap.String("message", e.Message),
			zap.String("path", ctx.Request.URL.Path),
		)
		ctx.JSON(e.HTTPStatus, e.ToErrorResponse())
	default:
		log.Error("Unhandled error occurred",
			zap.Error(err),
			zap.String("path", ctx.Request.URL.Path),
			zap.String("method", ctx.Request.Method),
		)
		ctx.JSON(http.StatusInternalServerError, apperror.ErrorResponse{
			Success: false,
			Error: apperror.Error{
				Code:    "INTERNAL_SERVER_ERROR",
				Message: "Error interno del servidor",
				Details: err.Error(),
			},
		})
	}
}

// AbortWithBusinessError es un helper para abortar con error de negocio.
func AbortWithBusinessError(ctx *gin.Context, err apperror.BusinessError) {
	ctx.Error(err)
	ctx.Abort()
}

// AbortWithError es un helper para abortar con error genérico.
func AbortWithError(ctx *gin.Context, err error) {
	ctx.Error(err)
	ctx.Abort()
}

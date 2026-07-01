package apperror

import "net/http"

// BusinessError representa errores de negocio con su mapeo a status HTTP.
// Vive en shared/apperror para ser usado por application sin depender de infraestructura HTTP.
type BusinessError struct {
	Code       string
	Message    string
	Details    string
	HTTPStatus int
}

func (e BusinessError) Error() string {
	return e.Message
}

// Error expuesto para serialización JSON.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

type ErrorResponse struct {
	Success bool  `json:"success"`
	Error   Error `json:"error"`
}

// Errores de negocio predefinidos.
var (
	ErrInvalidRequestFormat = BusinessError{
		Code:       "INVALID_REQUEST_FORMAT",
		Message:    "Formato de request inválido",
		HTTPStatus: http.StatusBadRequest,
	}

	ErrInvalidEmail = BusinessError{
		Code:       "INVALID_EMAIL",
		Message:    "El formato del email es inválido",
		HTTPStatus: http.StatusBadRequest,
	}

	ErrTemplateNotFound = BusinessError{
		Code:       "TEMPLATE_NOT_FOUND",
		Message:    "El template especificado no existe",
		HTTPStatus: http.StatusBadRequest,
	}

	ErrNotificationNotFound = BusinessError{
		Code:       "NOTIFICATION_NOT_FOUND",
		Message:    "La notificación no fue encontrada",
		HTTPStatus: http.StatusNotFound,
	}

	ErrInternalServer = BusinessError{
		Code:       "INTERNAL_SERVER_ERROR",
		Message:    "Error interno del servidor",
		HTTPStatus: http.StatusInternalServerError,
	}
)

// ToErrorResponse convierte un BusinessError al DTO serializable.
func (e BusinessError) ToErrorResponse() ErrorResponse {
	return ErrorResponse{
		Success: false,
		Error: Error{
			Code:    e.Code,
			Message: e.Message,
			Details: e.Details,
		},
	}
}

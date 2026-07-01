package port

import (
	"context"

	"notifications/src/notification/domain"
)

// Queue define la interfaz para operaciones de cola.
type Queue interface {
	// Enqueue añade una notificación a la cola.
	Enqueue(ctx context.Context, notification *domain.Notification) error

	// Dequeue recibe una notificación de la cola como entidad de dominio.
	Dequeue(ctx context.Context) (*domain.Notification, error)

	// Size retorna el número aproximado de mensajes en la cola.
	Size(ctx context.Context) (int64, error)
}

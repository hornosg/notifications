package port

import "context"

// Deduplicator es el fast-path de idempotencia: un nonce de corta vida (patrón Xu Ch.10,
// "nonce de la última hora") que evita reprocesar el mismo trigger ante la entrega
// at-least-once del EventBus. Es complementario —no sustituto— del backstop durable en DB
// (ExistsByDedupKey + UNIQUE index parcial).
//
// Implementación nil-safe: si no hay backend Redis configurado, el use case simplemente
// no lo invoca y cae al backstop de DB.
type Deduplicator interface {
	// MarkIfNew marca la clave como vista con TTL de forma atómica (SET NX).
	// Devuelve true si la clave NO existía (procesar) y false si ya existía (skip).
	MarkIfNew(ctx context.Context, key string) (bool, error)
}

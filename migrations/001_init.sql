-- 001_init.sql — esquema base de notifications
-- Corre como CONTROL PLANE (superusuario) vía postgres-setup. El rol de app no tiene DDL.

CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid()

-- notifications es cross-project (E23): namespace identifica el proyecto/IDP ('mc', etc.) y
-- tenant_id el tenant DENTRO de ese proyecto. tenant_id es NULLABLE: notificaciones de
-- plataforma (sin tenant específico) tienen tenant_id NULL — solo visibles vía break-glass
-- de system_admin (ver 002_rls.sql). Tipos VARCHAR, no uuid: mismo esquema que el legacy
-- notification-service, y tenant_id no siempre es un uuid válido (algunos emisores S2S).
CREATE TABLE IF NOT EXISTS notifications (
    id            VARCHAR(36) PRIMARY KEY,
    namespace     VARCHAR(50) NOT NULL,
    tenant_id     VARCHAR(36),
    type          VARCHAR(50) NOT NULL,
    action        VARCHAR(100) NOT NULL,
    template_id   VARCHAR(36),
    recipient     VARCHAR(500) NOT NULL,
    data          JSONB,
    status        VARCHAR(50) NOT NULL DEFAULT 'pending',
    retry_count   INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    -- idempotencia: event_id (eventos) o Idempotency-Key/hash (API sync). NULL = sin dedup.
    dedup_key     VARCHAR(255),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS notifications_namespace_tenant_idx ON notifications (namespace, tenant_id);
CREATE INDEX IF NOT EXISTS notifications_ns_status_idx ON notifications (namespace, status);
CREATE INDEX IF NOT EXISTS notifications_status_created_idx ON notifications (status, created_at);
CREATE INDEX IF NOT EXISTS notifications_pending_idx ON notifications (status) WHERE status = 'pending';

-- COALESCE(tenant_id, '') porque tenant_id es NULLABLE y en un índice UNIQUE los NULL son
-- distintos entre sí (dos filas con tenant_id NULL no colisionarían) — normalizamos a ''.
CREATE UNIQUE INDEX IF NOT EXISTS notifications_dedup_uq
    ON notifications (namespace, COALESCE(tenant_id, ''), dedup_key)
    WHERE dedup_key IS NOT NULL;

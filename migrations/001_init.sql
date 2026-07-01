-- 001_init.sql — esquema base de notifications
-- Corre como CONTROL PLANE (superusuario) vía postgres-setup. El rol de app no tiene DDL.

CREATE EXTENSION IF NOT EXISTS pgcrypto;   -- gen_random_uuid()

-- Tabla de ejemplo. Toda tabla de negocio en Devy es tenant-scoped por defecto (P-11):
-- lleva tenant_id y se aísla por RLS (ver 002_rls.sql). Si el servicio es single-tenant,
-- generá con --single y podés dropear la columna tenant_id.
--
-- notifications es además cross-project (E23): namespace identifica el proyecto/IDP
-- (p. ej. "mc") y tenant_id el tenant DENTRO de ese proyecto. Son dos ejes de scope
-- distintos y ambos obligatorios — ver decisión E23 2026-07-01. namespace es texto
-- ("mc", etc.), no uuid.
CREATE TABLE IF NOT EXISTS example (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    namespace  text NOT NULL,
    tenant_id  uuid NOT NULL,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS example_namespace_tenant_idx ON example (namespace, tenant_id);

-- 004_project_config.sql — configuración por proyecto + RLS fail-closed (E23).
-- Corre como CONTROL PLANE (superusuario) vía postgres-setup. El rol de app no tiene DDL.

-- project_config es 1 fila por namespace (identidad de envío, proveedor, canales, cuotas).
-- provider_creds_ref es una REFERENCIA a un secret del cluster, nunca la credencial en claro.
CREATE TABLE IF NOT EXISTS project_config (
    namespace            VARCHAR(50) PRIMARY KEY,
    from_email           VARCHAR(255) NOT NULL,
    from_name            VARCHAR(255) NOT NULL,
    provider_creds_ref   VARCHAR(255),
    channels_enabled     JSONB NOT NULL DEFAULT '["email"]',
    default_template_set VARCHAR(100),
    quota                JSONB,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE project_config ENABLE ROW LEVEL SECURITY;
ALTER TABLE project_config FORCE  ROW LEVEL SECURITY;

-- Policy solo por namespace: la PK ya es namespace, no hay tenant_id (config es de proyecto).
DROP POLICY IF EXISTS namespace_isolation ON project_config;
CREATE POLICY namespace_isolation ON project_config
    USING      (namespace = current_setting('app.namespace', true))
    WITH CHECK (namespace = current_setting('app.namespace', true));

DROP POLICY IF EXISTS break_glass ON project_config;
CREATE POLICY break_glass ON project_config
    USING (current_setting('app.role', true) = 'system_admin');

-- Seed del proyecto actual (mercado-cercano). provider_creds_ref apunta al secret default de plataforma.
INSERT INTO project_config (namespace, from_email, from_name, provider_creds_ref, channels_enabled)
VALUES ('mc', 'noreply@mercadocercano.com', 'Mercado Cercano', 'RESEND_API_KEY', '["email"]')
ON CONFLICT (namespace) DO NOTHING;

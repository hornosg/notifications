-- 003_templates.sql — tabla de templates + RLS fail-closed (E23).
-- Corre como CONTROL PLANE (superusuario) vía postgres-setup. El rol de app no tiene DDL.

-- templates es scope de proyecto (namespace), no de tenant: un template le pertenece al
-- proyecto entero (branding compartido por todos los tenants de ese namespace). Por eso la
-- policy de abajo filtra solo por namespace, sin tenant_id — no hay columna tenant_id acá.
CREATE TABLE IF NOT EXISTS templates (
    id          VARCHAR(36) PRIMARY KEY,
    namespace   VARCHAR(50) NOT NULL,
    name        VARCHAR(255) NOT NULL,
    subject     VARCHAR(500) NOT NULL,
    file_path   VARCHAR(1000) NOT NULL,
    action      VARCHAR(100) NOT NULL,
    type        VARCHAR(50) NOT NULL,
    version     INTEGER NOT NULL DEFAULT 1,
    is_active   BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT templates_namespace_name_uq UNIQUE (namespace, name)
);

CREATE INDEX IF NOT EXISTS templates_ns_action_type_idx ON templates (namespace, action, type);
CREATE INDEX IF NOT EXISTS templates_ns_action_type_version_idx ON templates (namespace, action, type, version);
CREATE INDEX IF NOT EXISTS templates_active_idx ON templates (is_active);

ALTER TABLE templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE templates FORCE  ROW LEVEL SECURITY;

-- Policy solo por namespace (ver nota arriba: templates no tiene tenant_id).
DROP POLICY IF EXISTS namespace_isolation ON templates;
CREATE POLICY namespace_isolation ON templates
    USING      (namespace = current_setting('app.namespace', true))
    WITH CHECK (namespace = current_setting('app.namespace', true));

DROP POLICY IF EXISTS break_glass ON templates;
CREATE POLICY break_glass ON templates
    USING (current_setting('app.role', true) = 'system_admin');

-- Seed del proyecto mercado-cercano (namespace 'mc'), portado de notification-service.
INSERT INTO templates (id, namespace, name, subject, file_path, action, type, version, is_active) VALUES
('template-welcome-001', 'mc', 'welcome_email', 'Bienvenido a nuestra plataforma', './templates/email/welcome_email.html', 'WELCOME', 'email', 1, true),
('template-verification-001', 'mc', 'verification_email', 'Verificación de email', './templates/email/verification_email.html', 'EMAIL_VERIFICATION', 'email', 1, true),
('template-password-reset-001', 'mc', 'password_reset_email', 'Recuperación de contraseña', './templates/email/password_reset.html', 'PASSWORD_RESET', 'email', 1, true),
('template-order-confirmation-001', 'mc', 'order_confirmation_email', 'Confirmación de pedido', './templates/email/order_confirmation.html', 'ORDER_CONFIRMATION', 'email', 1, true),
('template-shipping-notification-001', 'mc', 'shipping_notification_email', 'Tu pedido ha sido enviado', './templates/email/shipping_notification.html', 'SHIPPING_NOTIFICATION', 'email', 1, true)
ON CONFLICT (id) DO NOTHING;

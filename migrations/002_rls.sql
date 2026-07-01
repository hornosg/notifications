-- 002_rls.sql — Aislamiento fail-closed (Devy RULE-10 / roadmap E07), única fuente de scope
-- (ver docs/adr/ADR-001-rls-como-unica-fuente-de-aislamiento.md).
--
-- El aislamiento NO depende del WHERE del código: si una query olvida filtrar, la base la
-- filtra igual. La app hace SET app.tenant_id / app.namespace por request (ver
-- src/shared/database/database.go, TenantSession), leídos SIEMPRE del JWT ya validado.

ALTER TABLE notifications ENABLE ROW LEVEL SECURITY;
ALTER TABLE notifications FORCE  ROW LEVEL SECURITY;   -- aplica incluso al dueño de la tabla

-- Política compuesta namespace + tenant: notifications es cross-project (E23), así que el
-- aislamiento no alcanza con tenant_id solo — dos tenants con el mismo tenant_id en
-- proyectos distintos NO deben verse entre sí.
--
-- COALESCE en ambos lados: tenant_id es NULLABLE (notificaciones de plataforma sin tenant
-- específico). Esas filas solo son visibles cuando la sesión tampoco tiene tenant (system_admin
-- con X-Tenant-ID vacío, ver database.TenantSession) — mismo criterio que el UNIQUE index de
-- dedup en 001_init.sql.
DROP POLICY IF EXISTS tenant_isolation ON notifications;
CREATE POLICY tenant_isolation ON notifications
    USING      (namespace = current_setting('app.namespace', true)
                AND COALESCE(tenant_id, '') = COALESCE(current_setting('app.tenant_id', true), ''))
    WITH CHECK (namespace = current_setting('app.namespace', true)
                AND COALESCE(tenant_id, '') = COALESCE(current_setting('app.tenant_id', true), ''));

-- Break-glass del owner (Devy): system_admin puede ver/arreglar cross-tenant.
-- Se activa con SET app.role = 'system_admin' — sólo si el rol viene de los claims JWT
-- validados por tenantmw.TenantValidation (nunca de un header crudo, ver
-- src/shared/database/database.go hasRole()), y queda auditado en canonical logs con
-- user_id (Ley 25.326). La GUC se resetea antes de devolver la conexión al pool para que
-- no contamine al siguiente request que la recicle.
DROP POLICY IF EXISTS break_glass ON notifications;
CREATE POLICY break_glass ON notifications
    USING (current_setting('app.role', true) = 'system_admin');

-- Nota: al crear una tabla nueva tenant-scoped (templates, project_config), repetir este
-- bloque para esa tabla.

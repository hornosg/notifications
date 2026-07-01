-- 002_rls.sql — Aislamiento fail-closed (Devy RULE-10 / roadmap E07).
-- Sólo se incluye en servicios multi-tenant (omitido con --single).
--
-- El aislamiento NO depende del WHERE tenant_id del código: si una query olvida filtrar,
-- la base la filtra igual. La app hace SET app.tenant_id por request (ver src/shared/database).

-- Aplicar a cada tabla tenant-scoped. Ejemplo sobre `example`:
ALTER TABLE example ENABLE ROW LEVEL SECURITY;
ALTER TABLE example FORCE  ROW LEVEL SECURITY;   -- aplica incluso al dueño de la tabla

-- Política compuesta namespace + tenant: notifications es cross-project (E23), así que
-- el aislamiento no alcanza con tenant_id solo — dos tenants con el mismo tenant_id en
-- proyectos distintos NO deben verse entre sí. app.namespace lo setea database.TenantSession
-- a partir del claim JWT ya validado (nunca de un header crudo).
DROP POLICY IF EXISTS tenant_isolation ON example;
CREATE POLICY tenant_isolation ON example
    USING      (namespace = current_setting('app.namespace', true)
                AND tenant_id = current_setting('app.tenant_id', true)::uuid)
    WITH CHECK (namespace = current_setting('app.namespace', true)
                AND tenant_id = current_setting('app.tenant_id', true)::uuid);

-- Break-glass del owner (Devy): system_admin puede ver/arreglar cross-tenant.
-- Se activa con SET app.role = 'system_admin' — sólo si el rol viene de los claims JWT
-- validados por tenantmw.TenantValidation (nunca de un header crudo, ver
-- src/shared/database/database.go.tmpl hasRole()), y queda auditado en canonical logs
-- con user_id (Ley 25.326). La GUC se resetea antes de devolver la conexión al pool
-- para que no contamine al siguiente request que la recicle.
DROP POLICY IF EXISTS break_glass ON example;
CREATE POLICY break_glass ON example
    USING (current_setting('app.role', true) = 'system_admin');

-- Nota: al crear una tabla nueva tenant-scoped, repetir este bloque para esa tabla.

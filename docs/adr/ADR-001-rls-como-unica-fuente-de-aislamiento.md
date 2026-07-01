# ADR-001: RLS como única fuente de aislamiento — namespace/tenant no viajan en las firmas de los ports

**Estado**: Aceptado
**Fecha**: 2026-07-01
**Deciders**: owner (hornosg), sesión E23 (Recrear notification-service como repo notifications)

## Contexto

`notifications` es cross-project (E23): además de multi-tenant dentro de un proyecto, sirve
a múltiples proyectos/IDPs (namespace). Namespace y tenant son dos ejes de scope
independientes y ambos obligatorios — decisión confirmada por el owner el 2026-07-01.

El repo predecesor, `notification-service`, resolvía esto pasando `namespace` y `tenantID`
como parámetros explícitos en cada método de `NotificationRepository` (`FindByID(ctx,
namespace, tenantID, id)`, etc.) y filtrando con `WHERE namespace = $1 AND tenant_id = $2`
en cada query, además de tener RLS en la base como defensa en profundidad. El namespace de
esos parámetros salía de un `context.Context` propio (`appctx`) poblado por un middleware
Gin (`RLSPropagation`) que, en el flujo real, terminaba leyendo `X-Namespace` de un **header
sin autenticar** como fallback — ese fue el hallazgo CRITICAL del audit de plataforma que
motivó E07 (Aislamiento fail-closed) y, en cascada, la recreación completa del servicio como
`notifications` (E23): un header no firmado es spoofeable, así que el aislamiento "en el
código" no era confiable.

Al recrear el servicio con el scaffold de E07, `database.TenantSession` (middleware Gin)
fija UNA conexión Postgres por request y setea `app.tenant_id` / `app.namespace` como GUC de
sesión, leyendo el namespace **siempre** del claim `"namespace"` del JWT ya validado por su
firma (nunca de un header). Las policies RLS de `002_rls.sql` (`FORCE ROW LEVEL SECURITY`)
filtran por esas GUC automáticamente, en la base, para cualquier query que corra sobre esa
conexión — sin que el código de aplicación tenga que acordarse de agregar el `WHERE`.

Esto dejó una pregunta abierta: si RLS ya filtra en la base, ¿tiene sentido que
`NotificationRepository` siga recibiendo `namespace`/`tenantID` como parámetros y los repita
en el `WHERE`? Repetirlo es defensa en profundidad, pero también es lógica duplicada que
puede desincronizarse (p. ej. alguien pasa un `tenantID` distinto al de la sesión "por
error" y el resultado se ve raro en vez de fallar claro).

## Decisión

**RLS es la única fuente de aislamiento para las queries de lectura/actualización por ID.**
Los métodos del port `NotificationRepository` que antes recibían `namespace, tenantID` como
parámetros de filtro los dejan de recibir:

```go
// Antes
FindByID(ctx context.Context, namespace, tenantID, id string) (*domain.Notification, error)
UpdateStatus(ctx context.Context, namespace, tenantID, id string, status domain.NotificationStatus, error string) error
FindPendingNotifications(ctx context.Context, namespace, tenantID string) ([]*domain.Notification, error)
ExistsByDedupKey(ctx context.Context, namespace, tenantID, dedupKey string) (bool, error)

// Ahora
FindByID(ctx context.Context, id string) (*domain.Notification, error)
UpdateStatus(ctx context.Context, id string, status domain.NotificationStatus, error string) error
FindPendingNotifications(ctx context.Context) ([]*domain.Notification, error)
ExistsByDedupKey(ctx context.Context, dedupKey string) (bool, error)
```

`domain.NotificationFilters` pierde los campos `Namespace`/`TenantID` por el mismo motivo.

La implementación Postgres del repositorio (`infrastructure/repository`, a portar) **no**
abre su propia transacción con `SET LOCAL` (como hacía el repo legacy vía
`shared/postgres.WithRLSInTransaction`). En cambio usa **la misma conexión** que
`database.TenantSession` fijó para el request, recuperándola con
`database.ConnFromContext(ctx)` — un nuevo helper que guarda esa conexión también en el
`context.Context` estándar (no solo en `gin.Context`), para que los repositories (que reciben
`context.Context` por regla de dependencia, no `*gin.Context`) puedan acceder a ella.

Esto es importante: si un repository tomara una conexión distinta del pool (`db.QueryContext`
sobre el `*sql.DB` genérico), esa conexión física **no tendría las GUC seteadas** y RLS no
filtraría nada — el bug sería silencioso (devolver de más, no de menos). Por eso
`ConnFromContext` no es un atajo cosmético: es la pieza que hace que "confiar en RLS" sea
correcto y no una ilusión.

Los `Save`/`Update` (que sí escriben `namespace`/`tenant_id` en la fila) **siguen** recibiendo
esos valores, pero ya vienen adentro del struct `domain.Notification` que arma el use case
(vía `appctx.NamespaceFromContext`/`TenantIDFromContext`, resueltos en la capa de aplicación
desde el JWT) — no se agregó ni quitó nada ahí. Lo que cambia es solo la lectura/filtrado.

## Por qué esto NO es "esconder lógica"

La lógica de aislamiento no desaparece: se **centraliza en un solo lugar**
(`database.TenantSession` + `002_rls.sql`) en vez de estar repetida en cada método de cada
repository. Antes había DOS lugares que debían estar de acuerdo (el `WHERE` de cada query y
la policy RLS); ahora hay UNO. Menos lugares para que algo se desincronice, pero también
menos "cinturón y tiradores": si `TenantSession` tiene un bug, no hay una segunda capa de
`WHERE` que lo tape.

## Alternativas consideradas

### Opción A — RLS como única fuente (elegida)
- **Pros**: una sola fuente de verdad; imposible que el `WHERE` del código y la policy de
  RLS diverjan porque solo existe la policy; firmas de ports más simples y legibles.
- **Cons**: si `TenantSession` falla en setear las GUC (bug, refactor futuro que rompe el
  middleware, orden de montado incorrecto), NO hay segundo filtro que lo compense — el fallo
  es "todo visible" en vez de "нада visible". Mitigación: `FORCE ROW LEVEL SECURITY` (ya
  aplicado), tests de integración que verifiquen aislamiento cruzando namespace/tenant antes
  de ir a producción, y mantener `RejectMissingTenant`/rechazo por namespace faltante
  fail-closed en el propio middleware (ya implementado).

### Opción B — Defensa en profundidad (WHERE explícito + RLS), patrón legacy
- **Pros**: si un filtro falla, el otro puede compensar.
- **Cons**: exactamente el patrón que produjo el CRITICAL original — el "WHERE explícito"
  dependía de un valor de namespace resuelto en la capa de aplicación que, en el flujo real,
  terminaba cayendo a un header sin autenticar. Mantener dos filtros no evita el bug si
  ambos beben de la misma fuente contaminada; solo lo hace parecer más seguro de lo que es.

## Consecuencias

- Los ports quedan documentados explícitamente (comentario en
  `domain/port/repositories.go`) explicando que la ausencia de `namespace`/`tenantID` es
  intencional, no un olvido.
- Cualquier implementación futura de `NotificationRepository` (Postgres u otra) **debe** usar
  `database.ConnFromContext(ctx)` (o el mecanismo equivalente que la reemplace) — usar el
  pool genérico rompe el aislamiento en silencio.
- Un worker o proceso batch (futuro, p. ej. `sqs_worker` a portar) que no corra dentro de un
  request HTTP **no** tiene `TenantSession` montado automáticamente: deberá fijar su propia
  conexión y setear las mismas GUC explícitamente, replicando el mismo patrón fail-closed
  (leer namespace/tenant de una fuente validada, nunca de un campo del mensaje sin verificar).
- Este patrón (namespace resuelto del JWT validado, nunca de header/config fija; conexión
  fijada + GUC + RLS; ports sin parámetros de scope redundantes) es candidato a normalizar
  en el resto de microservicios Go y Python de la plataforma — queda pendiente como revisión
  de arquitectura aparte (no se asume que `notifications` ya es la referencia final; ver
  decisión E31 sobre IAM cross-project, que es un caso relacionado pero distinto: ahí el
  namespace se **emite**, acá se **consume**).

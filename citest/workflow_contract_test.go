// Package citest verifica el CONTRATO de los workflows de CI de imagen (PLAT-E35.T2),
// no su implementación. Testea los invariantes que el archivo de la épica promete en el
// "Hecho cuando" de T2:
//
//	"en push a main construye multi-stage, corre Trivy con gate en CRITICAL/HIGH
//	 (--ignore-unfixed, .trivyignore versionado), y pushea imagen privada a GHCR".
//
// El invariante de seguridad central es el ORDEN: el gate Trivy corre ANTES del push, y
// si Trivy falla no se pushea. El caso negativo (TestReusable_NoPushBeforeTrivyGate) es el
// que rompe si alguien reordena los pasos y publica una imagen sin escanear.
//
// Estos tests NO requieren runner de GitHub Actions: parsean el YAML versionado en el repo.
// Git está deshabilitado en el lab (sin CI activo), así que este es el gate reproducible local.
package citest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// repoRoot sube desde el cwd del test hasta encontrar go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no se encontró go.mod subiendo desde el cwd del test")
		}
		dir = parent
	}
}

func loadYAML(t *testing.T, rel string) map[string]any {
	t.Helper()
	path := filepath.Join(repoRoot(t), rel)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("leyendo %s: %v", rel, err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(raw, &m); err != nil {
		t.Fatalf("parseando %s: %v", rel, err)
	}
	return m
}

const (
	reusableWF = ".github/workflows/build-push-ghcr.yml"
	callerWF   = ".github/workflows/ci-image.yml"
	trivyIgn   = ".trivyignore"
)

// steps devuelve la lista de steps del único job del workflow reusable.
func reusableSteps(t *testing.T) []map[string]any {
	t.Helper()
	wf := loadYAML(t, reusableWF)
	jobs, ok := wf["jobs"].(map[string]any)
	if !ok {
		t.Fatalf("workflow reusable sin bloque jobs")
	}
	// Un solo job — tomamos el primero que tenga steps.
	for _, jv := range jobs {
		job, ok := jv.(map[string]any)
		if !ok {
			continue
		}
		rawSteps, ok := job["steps"].([]any)
		if !ok {
			continue
		}
		out := make([]map[string]any, 0, len(rawSteps))
		for i, s := range rawSteps {
			sm, ok := s.(map[string]any)
			if !ok {
				t.Fatalf("step %d no es un mapa", i)
			}
			out = append(out, sm)
		}
		return out
	}
	t.Fatalf("ningún job del workflow reusable tiene steps")
	return nil
}

func stepUses(s map[string]any) string {
	if u, ok := s["uses"].(string); ok {
		return u
	}
	return ""
}

func stepWith(s map[string]any) map[string]any {
	if w, ok := s["with"].(map[string]any); ok {
		return w
	}
	return map[string]any{}
}

// ---- Trigger y forma del reusable ---------------------------------------------------

func TestReusable_TriggerIsWorkflowCall(t *testing.T) {
	wf := loadYAML(t, reusableWF)
	on, ok := wf["on"].(map[string]any)
	if !ok {
		t.Fatalf("reusable: falta bloque on: mapa, got %T", wf["on"])
	}
	wc, ok := on["workflow_call"].(map[string]any)
	if !ok {
		t.Fatalf("reusable: on.workflow_call ausente — no es reusable (Contrato T2)")
	}
	inputs, ok := wc["inputs"].(map[string]any)
	if !ok {
		t.Fatalf("reusable: workflow_call sin inputs")
	}
	img, ok := inputs["image-name"].(map[string]any)
	if !ok {
		t.Fatalf("reusable: falta input image-name — el Contrato exige parametrizar por nombre de imagen")
	}
	if req, _ := img["required"].(bool); !req {
		t.Errorf("reusable: input image-name debe ser required: true, got %v", img["required"])
	}
}

func TestReusable_PermissionsPackagesWrite(t *testing.T) {
	wf := loadYAML(t, reusableWF)
	perms, ok := wf["permissions"].(map[string]any)
	if !ok {
		t.Fatalf("reusable: falta bloque permissions (necesario para push a GHCR)")
	}
	if got, _ := perms["packages"].(string); got != "write" {
		t.Errorf("reusable: permissions.packages = %q, quiero \"write\" para push a GHCR", got)
	}
}

// ---- Gate Trivy: configuración exacta del Contrato ----------------------------------

func TestReusable_TrivyGateConfig(t *testing.T) {
	steps := reusableSteps(t)
	var gate map[string]any
	for _, s := range steps {
		if strings.Contains(stepUses(s), "trivy-action") {
			gate = s
			break
		}
	}
	if gate == nil {
		t.Fatalf("reusable: no hay step que use aquasecurity/trivy-action — falta el gate")
	}
	w := stepWith(gate)

	// exit-code 1: sin esto Trivy reporta pero NO bloquea → deja de ser gate.
	if ec := toStr(w["exit-code"]); ec != "1" {
		t.Errorf("Trivy: exit-code = %q, quiero \"1\" (sin esto no bloquea el push)", ec)
	}
	// severity CRITICAL,HIGH exactamente (el gate del Contrato).
	sev := strings.ToUpper(strings.ReplaceAll(toStr(w["severity"]), " ", ""))
	if !strings.Contains(sev, "CRITICAL") || !strings.Contains(sev, "HIGH") {
		t.Errorf("Trivy: severity = %q, quiero incluir CRITICAL y HIGH", toStr(w["severity"]))
	}
	// --ignore-unfixed: exigido literalmente por el Hecho cuando.
	if iu := toStr(w["ignore-unfixed"]); iu != "true" {
		t.Errorf("Trivy: ignore-unfixed = %q, quiero \"true\" (Contrato)", iu)
	}
	// .trivyignore versionado referenciado.
	if ti := toStr(w["trivyignores"]); ti == "" {
		t.Errorf("Trivy: falta trivyignores apuntando al .trivyignore versionado (Contrato)")
	}
}

func TestTrivyignore_Versionado(t *testing.T) {
	path := filepath.Join(repoRoot(t), trivyIgn)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf(".trivyignore ausente en el repo — el Contrato lo exige versionado: %v", err)
	}
}

// ---- Multi-stage: build local (load, sin push) antes del escaneo --------------------

func TestReusable_BuildLoadsWithoutPushBeforeScan(t *testing.T) {
	steps := reusableSteps(t)
	buildIdx, gateIdx := -1, -1
	var build map[string]any
	for i, s := range steps {
		uses := stepUses(s)
		if strings.Contains(uses, "trivy-action") && gateIdx == -1 {
			gateIdx = i
		}
		if strings.Contains(uses, "build-push-action") {
			w := stepWith(s)
			if toStr(w["load"]) == "true" && toStr(w["push"]) == "false" && buildIdx == -1 {
				buildIdx, build = i, s
			}
		}
	}
	if build == nil {
		t.Fatalf("reusable: no hay build con load: true / push: false — la imagen no existe para escanearla")
	}
	if gateIdx == -1 {
		t.Fatalf("reusable: no hay gate Trivy")
	}
	if buildIdx > gateIdx {
		t.Errorf("reusable: el build local (idx %d) debe preceder al gate Trivy (idx %d)", buildIdx, gateIdx)
	}
}

// ---- INVARIANTE DE SEGURIDAD: gate ANTES del push -----------------------------------

func TestReusable_TrivyGateRunsBeforePush(t *testing.T) {
	steps := reusableSteps(t)
	gateIdx, pushIdx := -1, -1
	for i, s := range steps {
		uses := stepUses(s)
		if strings.Contains(uses, "trivy-action") && gateIdx == -1 {
			gateIdx = i
		}
		if strings.Contains(uses, "build-push-action") && toStr(stepWith(s)["push"]) == "true" && pushIdx == -1 {
			pushIdx = i
		}
	}
	if gateIdx == -1 {
		t.Fatalf("reusable: no hay gate Trivy")
	}
	if pushIdx == -1 {
		t.Fatalf("reusable: no hay step de push (build-push-action con push: true)")
	}
	if gateIdx >= pushIdx {
		t.Errorf("SEGURIDAD: el gate Trivy (idx %d) debe correr ANTES del push (idx %d); "+
			"si no, se publica una imagen sin escanear", gateIdx, pushIdx)
	}
}

// CASO NEGATIVO: ningún push puede aparecer antes del gate Trivy. Si el control falta o
// alguien reordena los pasos y publica antes de escanear, este test DEBE fallar.
func TestReusable_NoPushBeforeTrivyGate(t *testing.T) {
	steps := reusableSteps(t)
	gateIdx := -1
	for i, s := range steps {
		if strings.Contains(stepUses(s), "trivy-action") {
			gateIdx = i
			break
		}
	}
	if gateIdx == -1 {
		t.Fatalf("reusable: no hay gate Trivy contra el cual medir el orden")
	}
	for i := 0; i < gateIdx; i++ {
		s := steps[i]
		if strings.Contains(stepUses(s), "build-push-action") && toStr(stepWith(s)["push"]) == "true" {
			t.Errorf("SEGURIDAD: step %d ('%v') pushea con push: true ANTES del gate Trivy (idx %d) — imagen sin escanear publicada",
				i, s["name"], gateIdx)
		}
	}
}

// ---- Caller de notifications --------------------------------------------------------

func TestCaller_TriggersAndDelegates(t *testing.T) {
	wf := loadYAML(t, callerWF)

	on, ok := wf["on"].(map[string]any)
	if !ok {
		t.Fatalf("caller: bloque on no es mapa, got %T", wf["on"])
	}
	push, ok := on["push"].(map[string]any)
	if !ok {
		t.Fatalf("caller: falta on.push (el Contrato exige disparar en push a main)")
	}
	branches := toStrSlice(push["branches"])
	if !contains(branches, "main") {
		t.Errorf("caller: on.push.branches = %v, quiero incluir main (Contrato)", branches)
	}
	// El cierre documenta main/master; verificamos master como parte del contrato de cierre.
	if !contains(branches, "master") {
		t.Errorf("caller: on.push.branches = %v, quiero incluir master (cierre T2)", branches)
	}
	tags := toStrSlice(push["tags"])
	if !contains(tags, "v*") {
		t.Errorf("caller: on.push.tags = %v, quiero incluir v* (tagging semver, T1)", tags)
	}

	jobs, ok := wf["jobs"].(map[string]any)
	if !ok {
		t.Fatalf("caller: sin jobs")
	}
	var job map[string]any
	for _, jv := range jobs {
		if jm, ok := jv.(map[string]any); ok {
			job = jm
			break
		}
	}
	if job == nil {
		t.Fatalf("caller: no se pudo leer el job")
	}
	uses := toStr(job["uses"])
	if !strings.Contains(uses, "build-push-ghcr.yml") {
		t.Errorf("caller: uses = %q, quiero delegar en build-push-ghcr.yml (reusable T2)", uses)
	}
	with, _ := job["with"].(map[string]any)
	if toStr(with["image-name"]) != "notifications" {
		t.Errorf("caller: with.image-name = %q, quiero \"notifications\"", toStr(with["image-name"]))
	}
	if toStr(job["secrets"]) != "inherit" {
		t.Errorf("caller: secrets = %q, quiero \"inherit\" (pasa GITHUB_TOKEN/GO_PRIVATE_TOKEN)", toStr(job["secrets"]))
	}
}

// ---- helpers ------------------------------------------------------------------------

func toStr(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	default:
		// ints/floats (p.ej. exit-code: 1 sin comillas en el YAML)
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func toStrSlice(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, e := range raw {
		out = append(out, toStr(e))
	}
	return out
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

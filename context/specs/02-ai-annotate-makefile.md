# Unit 02: `cast ai annotate` — autocompletar doc-lines y tags de un Makefile

## Goal

Añadir un subsistema `internal/ai/` y un subcomando `cast ai annotate`
que, dado el `Makefile` resuelto por la lógica de
`source.lookup_depth`, use Groq (`llama-3.3-70b-versatile` por
defecto) para proponer y escribir los `## name: desc [tags=…]` que
falten en cada target. La operación es **dual surface**:

1. **CLI** — diff coloreado global + confirmación `y/n` antes de
   escribir. Soporta `--target NAME` para anotar uno solo y `--all`
   para sobrescribir los que ya tienen doc-line manual.
2. **TUI** — `ctrl+i` desde el tab `commands` abre un popup que
   permite elegir entre **(t)** anotar solo el target seleccionado,
   **(a)** anotar los que no tienen doc-line, o **(A)** anotar todos
   sobrescribiendo. Renderiza el diff propuesto, spinner mientras
   espera al LLM, y aplica con `enter` o cancela con `esc`.

**Alcance acotado a doc-line + tags categóricos.** Este unit **no**
infiere `[stream]`, `[confirm]`, `[interactive]`, `[sc=X]` ni
`[pick=…]`. Esos quedan fuera para no introducir falsos positivos que
cambien semántica del runner.

## Design

### Tipos en `internal/ai/`

```go
package ai

// Annotation is what the LLM proposes for a single target.
type Annotation struct {
    Name string   // target name (must match an existing target)
    Desc string   // proposed description (≤ 80 chars, infinitive verb)
    Tags []string // categorical tags from the allowed list
}

// Plan is the result of a single Annotate() call.
type Plan struct {
    Annotations []Annotation
    Skipped     []SkipReason // targets the LLM declined or the filter excluded
}

type SkipReason struct {
    Name   string
    Reason string // e.g. "already documented", "no recipe to infer from"
}

// Provider abstracts the LLM backend. Only GroqProvider in this unit;
// the interface exists so a future Ollama/Anthropic backend slots in
// without touching call sites.
type Provider interface {
    Annotate(ctx context.Context, req Request) (Plan, error)
}

type Request struct {
    Targets       []TargetView // {Name, Recipe[]string, ExistingDocLine}
    AllowedTags   []string     // empty = LLM decides freely
    OverwriteAll  bool         // include already-annotated targets in payload
    Model         string
    MaxTargets    int          // batch size; caller handles > N by splitting
}
```

### Config — nueva sección `[ai]`

Añadir a `internal/config/toml.go` y a `internal/config/config.go`:

```toml
[ai]
provider     = "groq"                       # only "groq" wired in this unit
model        = "llama-3.3-70b-versatile"
api_key_env  = "GROQ_API_KEY"
endpoint     = "https://api.groq.com/openai/v1/chat/completions"
max_targets  = 40
timeout_secs = 30

[ai.tags]
allowed = ["build", "test", "deploy", "lint", "db", "docker", "dev", "clean", "release", "docs", "ci"]
```

Layering: defaults → global TOML → local TOML → env → flags (igual
que el resto). El API key se resuelve leyendo `os.Getenv(api_key_env)`;
si está vacío, error claro con instrucciones de configurar la env var
en `~/.config/cast/.env` o el shell. **No** se persiste el key en
TOML.

### Paquete `internal/ai/`

Estructura:

```
internal/ai/
  ai.go              // Provider interface, Request/Plan/Annotation types
  groq.go            // GroqProvider implementing Provider via net/http
  prompt.go          // system prompt + JSON schema for the response
  filter.go          // BuildTargetViews: parse Makefile, filter by mode
  apply.go           // RenderDiff(plan, src) + ApplyPlan(plan, path)
```

**Constraints (architecture invariants):**

- `internal/ai/` solo importa `stdlib` + `internal/source`. No
  importa `tui`, `runner`, `db`, ni `lipgloss` para los tipos
  centrales. El diff renderer (`apply.go`) puede usar `lipgloss` para
  colorear, pero la versión sin color (plain string) se expone
  aparte para que las views puedan re-renderizar con el palette
  activo.
- Cero side-effects en `Annotate()`. La única función que escribe a
  disco es `ApplyPlan(plan, path)`.

### Prompt al LLM

System prompt (resumen, ver `internal/ai/prompt.go` para el texto
completo):

```
You annotate GNU Make targets with doc-lines for the `cast` task
runner. For each target, return:
  - desc: a concise English description (≤ 80 chars), infinitive verb
    (e.g. "Build the production Docker image").
  - tags: 1–3 categorical tags from this allowed list: <ALLOWED>.
    If none fits, return an empty array.
Never invent target names. Skip targets where the recipe is empty
or you cannot infer purpose; list them in `skipped`.
Respond with strict JSON: {"annotations":[…], "skipped":[…]}.
```

Payload por target (limitado a `max_targets` por call):

```
TARGET: <name>
RECIPE:
<recipe lines, max 40 lines per target, truncated with "… (truncated)" tail>
EXISTING_DOC: <line> | <empty>
```

### Reescritura del Makefile

Nueva función pública en `internal/source/makefile.go`:

```go
// WriteDocLine inserts or replaces the ## doc-line directly above
// the target named `name` in `lines`. If a `## name: …` line already
// sits immediately above the target, it is replaced; otherwise a new
// line is inserted. Returns the new lines slice; does not write to
// disk.
func WriteDocLine(lines []string, name, desc string, tags []string) ([]string, error)
```

`ApplyPlan` en `internal/ai/apply.go`:

1. Lee el Makefile entero a `[]string`.
2. Para cada `Annotation`, llama `source.WriteDocLine`.
3. Escribe atómicamente: `.<file>.tmp` → `os.Rename`.
4. No deja backup automático; el usuario debe usar git.

### CLI — `cmd/cast/main.go`

Nuevo dispatcher en el switch principal:

```go
case "ai":
    runAICommand(os.Args[2:])
```

Implementación en `cmd/cast/ai.go`:

```
cast ai annotate              → todos los targets sin doc-line
cast ai annotate --all        → todos, sobrescribiendo
cast ai annotate --target X   → solo X (X debe existir; error si no)
cast ai annotate --dry-run    → imprime diff y termina
cast ai annotate --yes        → aplica sin pedir confirmación
cast ai annotate --json       → imprime el Plan como JSON (para scripts)
```

Flujo default:

1. Carga config (incluye `[ai]`).
2. Localiza Makefile (reusa la misma lógica que el TUI:
   `cfg.Source.LookupDepth`).
3. Construye `TargetView`s con `filter.go` según las flags.
4. Llama `provider.Annotate(ctx, req)` con timeout
   `cfg.AI.TimeoutSecs`.
5. Renderiza diff coloreado y lo manda a stdout.
6. Si `--dry-run`: termina con exit 0.
7. Si `--yes`: aplica directo.
8. Default: prompt `Apply N annotation(s)? [y/N]` con `bufio.NewReader(os.Stdin)`.

Exit codes:

- `0` — anotaciones aplicadas (o `--dry-run` sin error).
- `1` — error de config, API key faltante, Makefile no encontrado.
- `2` — error de LLM (timeout, status ≠ 200, JSON inválido).
- `3` — usuario respondió `n` al prompt; sin cambios.

### TUI — keybinding `ctrl+i`

Añadir a `internal/tui/keys.go`:

```go
AnnotateAI string // "ctrl+i" — open the AI annotate popup
```

Default: `"ctrl+i"`. Confirmar que no choca con bindings existentes
(`ctrl+a/d/e/k/o/r/s/t/y/x` ya están tomados; `ctrl+i` está libre).

Flujo dentro del Model (`internal/tui/model.go`):

1. `ctrl+i` desde el tab `commands` abre `aiAnnotatePopup` (nuevo
   sub-modelo en `internal/tui/views/ai_popup.go` siguiendo el
   contrato de Props puras).
2. Estado inicial: menú con tres opciones:
   - `t` — Anotar este target (el resaltado en sidebar).
   - `a` — Anotar todos los targets sin doc-line.
   - `A` — Anotar todos (sobrescribir doc-lines existentes).
   - `esc` — cerrar.
3. Al elegir, se dispara una `tea.Cmd` que llama
   `ai.GroqProvider.Annotate(ctx, req)` en goroutine. **Mientras
   tanto, el popup muestra spinner y la TUI queda bloqueada al
   popup** (el resto del Model ignora keys excepto `esc` cancela).
4. Al volver el `aiPlanMsg`:
   - Si error: el popup muestra el error y `esc` cierra.
   - Si éxito: el popup renderiza el diff (reutiliza
     `ai.RenderDiff`) en un viewport con `↑↓ pgup pgdn` y un footer:
     `enter apply  esc cancel`.
5. `enter` invoca `ApplyPlan` y cierra el popup con un notice
   `"annotated N target(s)"`. Tras aplicar, **se reparsea el
   Makefile** (`reloadCommandsCmd`) para que el sidebar refleje los
   nuevos doc-lines sin reiniciar cast.

### Diff renderer

`ai.RenderDiff(plan Plan, src []byte, p ui.Palette) string`:

- Formato unified diff (3 líneas de contexto), por target.
- Líneas `+` en `p.Success`, `-` en `p.Danger`, contexto en `p.FgDim`.
- Si `p == nil` (CLI sin terminal interactivo / `--json`), versión
  sin color.

### Telemetría / observabilidad

Mínima: log de un solo line a stderr en CLI con
`provider=groq model=… targets=N elapsed=Xms` tras cada call. Sin
persistencia en SQLite en este unit.

## Implementation

### `internal/ai/ai.go`

- Definir `Provider`, `Request`, `Plan`, `Annotation`, `SkipReason`,
  `TargetView`.

### `internal/ai/groq.go`

- `GroqProvider struct { APIKey, Model, Endpoint string; HTTP *http.Client }`.
- `Annotate`: arma payload OpenAI-compatible (`/chat/completions`
  con `response_format: {type:"json_object"}`), envía, parsea, valida
  que cada `Name` exista en `req.Targets`. Targets devueltos con
  nombre inexistente → se descartan y se loguean.
- Errores envueltos con `fmt.Errorf("groq: %w", err)`.

### `internal/ai/prompt.go`

- `SystemPrompt(allowed []string) string`.
- `UserPayload(req Request) string`.
- Schema JSON esperado documentado en comentario.

### `internal/ai/filter.go`

- `BuildTargetViews(lines []string, mode FilterMode, only string) ([]TargetView, error)`.
- `FilterMode`: `OnlyMissingDoc | All | SingleTarget`.

### `internal/ai/apply.go`

- `RenderDiff(plan, src, palette)`.
- `ApplyPlan(plan, path) error` — atomic write.

### `internal/source/makefile.go`

- Nueva `WriteDocLine` (ver firma arriba). **Test obligatorio**:
  insertar arriba de un target sin doc-line, reemplazar un doc-line
  existente, no tocar líneas fuera de la región.

### `internal/config/`

- Añadir `AISection` a `toml.go` y `config.go`.
- Defaults sanos en `Defaults()`.
- `EnsureGlobal` mantiene compatibilidad si el archivo viejo no
  tiene `[ai]`.

### `cmd/cast/ai.go`

- `runAICommand(args []string)` con flag set propio.
- Help text bajo `cast ai --help`.

### `cmd/cast/main.go`

- Despacho del subcomando `ai`.
- Actualizar `usage` con las nuevas líneas.

### `internal/tui/keys.go`

- Campo `AnnotateAI = "ctrl+i"`.

### `internal/tui/model.go`

- Estado del popup (modo, plan, error, spinner, viewport offset).
- Handlers para `ctrl+i`, transiciones, `aiPlanMsg`, `aiErrorMsg`,
  apply + reload.

### `internal/tui/views/ai_popup.go`

- Pure render: `AIAnnotatePopup(props AIAnnotatePopupProps, p Palette) string`.
- Props: `Mode`, `Plan`, `Error`, `Spinner string`, `DiffViewport
  string`, `Hint string`.

### `internal/tui/render.go`

- Overlay del popup cuando `m.aiPopupOpen`.

## Dependencies

- Ninguna nueva en `go.mod`. `net/http`, `encoding/json` y
  `context` son stdlib. Groq endpoint es compatible OpenAI por lo que
  el cliente se escribe a mano (cero SDK).

## Verify when done

- [ ] `cast ai annotate --dry-run` sobre un Makefile sin doc-lines
      imprime un diff coloreado con doc-lines propuestos para cada
      target válido y termina con exit 0 sin tocar el archivo.
- [ ] `cast ai annotate` (sin flags) pide confirmación `[y/N]`;
      respondiendo `n` termina con exit 3 sin modificar el archivo;
      respondiendo `y` aplica y termina con exit 0.
- [ ] `cast ai annotate --target build` solo afecta al target
      `build`; si no existe, error con exit 1.
- [ ] `cast ai annotate --all` re-anota incluso targets que ya tenían
      doc-line manual.
- [ ] Targets sin receta o que el LLM no pudo inferir aparecen en la
      lista `skipped` del output y **no** se modifican.
- [ ] Si `GROQ_API_KEY` no está definida, el comando falla con un
      mensaje claro indicando dónde configurarla (env o
      `~/.config/cast/.env`).
- [ ] En la TUI, `ctrl+i` abre el popup con el menú de tres
      opciones. `esc` cierra sin llamadas a Groq.
- [ ] La opción `t` (este target) envía un payload con un único
      target y aplica solo a él.
- [ ] Las opciones `a` y `A` aplican a múltiples targets; tras
      `enter` el sidebar se reparsea y muestra los nuevos
      doc-lines sin reiniciar cast.
- [ ] Mientras espera la respuesta del LLM, el popup muestra el
      spinner y bloquea otros key inputs (excepto `esc` para
      cancelar).
- [ ] `WriteDocLine` tiene tests unitarios en
      `internal/source/makefile_doctest.go` (o equivalente)
      cubriendo: inserción, reemplazo, target inexistente.
- [ ] `internal/ai/` no importa `tui`, `runner`, `db`, ni `lipgloss`
      desde sus tipos centrales (`ai.go`, `groq.go`, `filter.go`).
      Solo `apply.go` puede usar `lipgloss` para el render
      coloreado.
- [ ] `make build` pasa.
- [ ] `make test` pasa.
- [ ] `make lint` no produce hallazgos nuevos.
- [ ] `version.Current` bumpea a `0.25.0` (feature aditiva,
      MINOR) y `CHANGELOG.md` añade entrada con ejemplo de uso
      tanto CLI como TUI.
- [ ] `context/project-overview.md` → añadir bullet en *Features*
      sobre `ai annotate`.
- [ ] `context/architecture.md` → añadir `internal/ai/` a *System
      Boundaries* y actualizar el import graph
      (`cmd/cast → ai`, `tui → ai`, `ai → {source, stdlib, net/http}`).
- [ ] `context/progress-tracker.md` → mover a *Completed* tras
      merge.

## Open questions to resolve antes del merge

1. **Endpoint configurable** — `groq.com` hoy; si más adelante se
   añade Anthropic/Ollama, ¿el switch va por `provider` string o por
   build tags? Decisión diferida a un unit posterior.
2. **Caching** — ¿Cachear respuestas por hash de la receta para
   evitar re-pagar tokens en re-runs cercanos? Diferido; primero
   medir.
3. **Rate limit / batching** — Si un Makefile tiene > `max_targets`
   targets, ¿cuántas llamadas en paralelo? Este unit hace
   secuencial; el paralelismo es optimización futura.

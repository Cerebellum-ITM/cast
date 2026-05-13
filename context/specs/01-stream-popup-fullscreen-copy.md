# Unit 01: Stream popup — global quit, fullscreen mode, OSC52 copy

## Goal

Mejorar el popup expandido de output (el que se abre con `ctrl+e`
mientras un comando está en stream) en tres puntos:

1. Respetar el atajo global de salida `ctrl+x` (`keys.QuitAlt`)
   estando dentro del popup. Hoy `handleExpandedOutput` solo procesa
   `ctrl+c`, `esc`, `ctrl+e`, navegación y `g/G`; `ctrl+x` no hace nada.
2. Añadir un modo **fullscreen** del popup, accesible con un segundo
   `ctrl+e` (primer `ctrl+e` → popup centrado, segundo `ctrl+e` →
   fullscreen). El fullscreen ocupa todo el ancho/alto del programa
   **menos la status bar inferior**, que sigue visible. `esc` o un
   tercer `ctrl+e` cierran el popup por completo (no se cicla).
3. Permitir copiar todo el log del popup al portapapeles del usuario
   vía **OSC52** con `y`, funcionando incluso por SSH o en
   terminales sin acceso al clipboard nativo.

## Design

### Estados del popup

Hoy: `m.showOutputExpand bool`. Pasa a un enum de tres estados:

```go
type outputExpandMode int

const (
    outputExpandHidden outputExpandMode = iota
    outputExpandPopup
    outputExpandFullscreen
)
```

El campo del Model deja de ser `showOutputExpand bool` y pasa a
`outputExpandMode outputExpandMode`. Cualquier predicado existente
`if m.showOutputExpand` se reemplaza por
`if m.outputExpandMode != outputExpandHidden`.

### Transiciones (al presionar `ctrl+e`)

| Estado actual | Acción |
|---|---|
| `Hidden` | → `Popup`. Igual que hoy: `outputExpandOff = max(0, len-visH)`. |
| `Popup` | → `Fullscreen`. Recalcula `outputExpandOff` para mantener la línea actualmente en el tope visible (ver más abajo). |
| `Fullscreen` | → `Hidden`. |

`esc` siempre va a `Hidden` desde cualquier estado.

Mantener la posición de scroll al pasar de popup ↔ fullscreen: el
`visH` cambia, así que tras la transición se ajusta
`outputExpandOff = clamp(outputExpandOff, 0, maxOff)`.

### Layout fullscreen

- Dimensiones del box: `popupW = m.width`, `popupH = m.height - 1`
  (deja 1 fila para la status bar inferior). Si el render del header
  inferior necesita más altura, ajustar para no taparlo — referencia
  en `internal/tui/render.go` donde se compone `full` con
  `views.StatusBar`.
- En lugar de `views.OverlayCenter(full, box)`, en fullscreen se
  renderiza directamente `box + "\n" + views.StatusBar(...)`. No hay
  overlay porque el box cubre toda la zona.
- El `views.ExpandedOutput` actual sirve si se le pasan las
  dimensiones expandidas; el borde redondeado y el hint inferior se
  mantienen (ver `ui-context.md` → aesthetic guardrails: borders =
  `Rounded`, paleta = `palette.Accent`).

### Hint row

La línea de hints en `views.ExpandedOutput` actualmente dice:

```
↑↓ / j k    pgup pgdn    g G top/end    ctrl+e  esc  close
```

Pasa a depender del modo:

- **Popup:** `↑↓ pgup pgdn  g G  y copy  ctrl+e fullscreen  esc close`
- **Fullscreen:** `↑↓ pgup pgdn  g G  y copy  ctrl+e close  esc close`

`ctrl+x` no se anuncia en el hint (es un quit global, no una acción
del popup), pero **debe funcionar** desde ambos estados.

## Implementation

### `internal/tui/keys.go`

Añadir un campo `CopyOutput string` al `KeyMap` con valor por defecto
`"y"`. No reutilizar `ToggleSecrets` ("s") ni ningún otro binding
existente. Anotar en el comentario del struct que `y` aquí solo
aplica dentro del popup expandido.

### `internal/tui/model.go`

1. Sustituir el campo `showOutputExpand bool` por
   `outputExpandMode outputExpandMode` (definir el tipo en el mismo
   archivo, junto a las demás constantes de modo).
2. `toggleOutputExpand()` se renombra a `cycleOutputExpand()` y
   implementa las tres transiciones de la tabla.
3. `handleExpandedOutput(msg)`:
   - Añadir `case m.keys.QuitAlt: return m, tea.Quit` **antes** del
     bloque `esc` para que `ctrl+x` cierre cast desde el popup.
   - Reemplazar el caso `case "esc", m.keys.ExpandOutput:` por dos
     ramas:
     - `case m.keys.ExpandOutput:` → invoca `cycleOutputExpand()`.
     - `case "esc":` → setea
       `m.outputExpandMode = outputExpandHidden`.
   - Añadir `case m.keys.CopyOutput:` que dispara
     `copyOutputToClipboard(m.output)` (ver helper abajo) y muestra
     una `setNotice("copied N lines", views.NoticeInfo)`.
4. `outputExpandVisH()` recibe el modo o el flag de fullscreen para
   calcular alto distinto:
   - **Popup:** `popupH = m.height - 4` (sin cambios).
   - **Fullscreen:** `popupH = m.height - 1` (status bar inferior).
5. Donde hoy se hace `m.showOutputExpand = false` (cancel run o
   finalización forzada), pasa a
   `m.outputExpandMode = outputExpandHidden`.

### `internal/tui/render.go`

Sustituir el bloque `if m.showOutputExpand { … OverlayCenter … }` por
un `switch m.outputExpandMode`:

- `outputExpandPopup`: igual que hoy (`popupW = m.width - 8`,
  `popupH = m.height - 4`, `OverlayCenter`).
- `outputExpandFullscreen`: `popupW = m.width`,
  `popupH = m.height - 1`, devolver
  `views.ExpandedOutput(...) + "\n" + views.StatusBar(...)` directamente
  (sin `OverlayCenter`). Reutilizar la misma `StatusBar` Props que
  usa `renderMain`.
- `outputExpandHidden`: cae al `return full` actual.

### `internal/tui/views/expanded_output.go`

1. Añadir parámetro `mode` (o booleano `fullscreen`) a
   `ExpandedOutput` para que el hint row cambie según el modo.
   Mantener la firma actual como wrapper si conviene, o actualizar
   todas las llamadas.
2. Nada más cambia: el borde, padding y colorización se mantienen.

### OSC52 helper

Nuevo helper `copyOutputToClipboard(lines []string) tea.Cmd` en
`internal/tui/model.go` (o `internal/tui/clipboard.go` si crece):

- Une las líneas con `\n` (sin ANSI: usar `ansi.Strip` de
  `github.com/charmbracelet/x/ansi` que ya está en `go.mod`).
- Codifica el payload en base64.
- Emite la secuencia OSC52: `"\x1b]52;c;<base64>\x07"`.
- Devuelve un `tea.Cmd` que escribe esa secuencia al terminal. Bubble
  Tea v2 expone `tea.SetClipboard` (verificar disponibilidad en
  `charm.land/bubbletea/v2`); si existe, usarlo en vez del escape
  literal. Si no, escribir directamente con `tea.Printf` o emitir
  un `tea.WriteMsg`.
- No usar `github.com/atotto/clipboard` (ya está como indirect dep,
  pero solo funciona localmente — falla por SSH, que es exactamente
  el caso que OSC52 resuelve).

Truncar el payload a un máximo razonable (p. ej. 100 KB) y, si se
trunca, anunciar en el notice: `"copied N lines (truncated to X KB)"`.

## Dependencies

- Ninguna nueva. `github.com/charmbracelet/x/ansi` ya está disponible
  (usado en `expanded_output.go`).
- Verificar la API de `charm.land/bubbletea/v2` para clipboard nativo;
  si no existe, fallback al escape OSC52 manual (sin dependencia).

## Verify when done

- [ ] Con un comando en stream y popup abierto (`ctrl+e`), presionar
      `ctrl+x` cierra cast (no se queda atrapado el popup).
- [ ] Primer `ctrl+e` abre popup centrado; segundo `ctrl+e` lo lleva
      a fullscreen ocupando todo el alto menos la status bar inferior;
      tercer `ctrl+e` lo cierra. La status bar permanece visible en
      ambos modos del popup.
- [ ] Al alternar popup ↔ fullscreen, la posición de scroll no
      "salta" — el contenido visible al tope se mantiene (clamp por
      `maxOff` recalculado).
- [ ] `y` dentro del popup copia el log al portapapeles del sistema
      vía OSC52, incluso en una sesión SSH (probar con `tmux` o
      conexión remota). Muestra un notice con la cantidad de líneas
      copiadas.
- [ ] El hint inferior del popup refleja el modo activo (incluye
      `ctrl+e fullscreen` en popup y `ctrl+e close` en fullscreen,
      `y copy` en ambos).
- [ ] `esc` cierra el popup desde cualquier modo (no cicla).
- [ ] No se rompe `ctrl+c` actual (cancela el run y deja el popup
      abierto si el comando estaba corriendo; cierra cast si no).
- [ ] `make build` pasa.
- [ ] `make test` pasa.
- [ ] `make lint` no produce hallazgos nuevos.
- [ ] `version.Current` bumpea a `0.18.0` (es feature aditiva =
      MINOR) y `CHANGELOG.md` añade una entrada con un snippet de uso
      de `ctrl+e` (popup → fullscreen) y `y` (OSC52 copy).
- [ ] `context/progress-tracker.md` actualizado: mover esta unidad
      de "In Progress" a "Completed" tras merge.

# Unit NN: [Feature Name]

## Goal

One or two sentences describing the concrete, verifiable output of
this unit. Be specific. Bad: "Improve the picker." Good: "Add a
`[pick=…/*.yaml]` file-filter variant so the picker can list files
matching a glob, exposed to the recipe as `$(CAST_PICK_N)`. Keep the
existing folder-only behavior when no glob is provided."

## Design

Visual and structural decisions specific to this unit. Reference
`context/ui-context.md` palette/icon fields by name. If the unit
touches a view, describe the Props the view will accept and how the
existing render functions should compose. The agent should make zero
visual decisions on its own.

## Implementation

### [Package or sub-section name — e.g. `internal/source/makefile.go`]

Detailed description of what to build: function signatures, struct
fields, behavior, edge cases. Enough detail that "done" is unambiguous.
Reference invariants from `architecture.md` that constrain the
implementation.

### [Next sub-section]

Description.

## Dependencies

- `module/path` — reason it's needed in this unit
- (or: none)

## Verify when done

- [ ] _Unit-specific verifiable condition 1_
- [ ] _Unit-specific verifiable condition 2_
- [ ] _Unit-specific verifiable condition 3_
- [ ] `make build` passes
- [ ] `make test` passes
- [ ] `make lint` produces no new findings
- [ ] If user-visible: `version.Current` bumped + `CHANGELOG.md`
      entry added, same commit
- [ ] `progress-tracker.md` updated

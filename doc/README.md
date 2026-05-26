# Yaran — Design Documentation

## Overview

Yaran is a terminal address book manager written in Go. It stores contacts in a
local SQLite database and exposes a keyboard-driven TUI for browsing, editing,
filtering, importing, and exporting address data.

The name *yaran* is Persian for *dearest friends*.

---

## Goals

- Single-binary, zero-dependency deployment (SQLite is embedded via CGo-free
  `modernc.org/sqlite`).
- All data stays local; no network access.
- Keyboard-first UX with mouse support as a secondary input method.
- Extensible import/export via a plugin registry.

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────┐
│                   cmd/yaran/main.go                 │
│  CLI flags → Service init → TUI launch              │
└────────────────────┬────────────────────────────────┘
                     │
          ┌──────────▼──────────┐
          │   internal/tui      │
          │   App (tview)       │  ← keyboard / mouse events
          └──────────┬──────────┘
                     │ calls
          ┌──────────▼──────────┐
          │  internal/yaran     │
          │  Service            │  ← application logic
          └──────┬──────┬───────┘
                 │      │
        ┌────────▼─┐  ┌─▼──────────┐
        │  Store   │  │  Registry  │
        │ (SQLite) │  │ (plugins)  │
        └──────────┘  └─────┬──────┘
                             │ FormatHandler
                    ┌────────┴────────┐
                    │  CSV / JSON /   │
                    │  HTML / VCard   │
                    └─────────────────┘
```

### Layers

| Layer | Package | Responsibility |
|---|---|---|
| Entry point | `cmd/yaran` | Parse CLI flags, wire dependencies, start TUI |
| Presentation | `internal/tui` | Render UI, handle input, call Service |
| Application | `internal/yaran.Service` | Orchestrate Store and Registry |
| Persistence | `internal/yaran.Store` | SQLite CRUD, schema migration |
| Plugin system | `internal/yaran.Registry` | Register and dispatch format handlers |
| Plugins | `internal/yaran.{CSV,JSON,HTML,VCard}Plugin` | Import/export logic per format |
| Domain | `internal/yaran.Address`, `Filters` | Data model and validation |

---

## Package Details

### `cmd/yaran`

The entry point. Responsibilities:

- Parse `--db-path` and `--version` flags.
- Construct a `yaran.Service` with the resolved database path.
- Call `service.InitializeDatabase()` to ensure the schema is up to date.
- Hand off to `tui.New(service).Run()`.

Default database path: `~/.yaran.db`.

### `internal/yaran`

Core domain and business logic. All types and functions that are not
presentation-specific live here.

#### `Address` and `Filters`

```go
type Address struct {
    ID       int64
    Name     string   // required
    Email    string   // required
    Birthday string   // optional, YYYY-MM-DD
    Address  string
    Phone    string
    Mobile   string
    Custom   string   // semicolon-separated tags
    Notes    string
}
```

`Filters` mirrors `Address` fields and is used to build `WHERE` clauses in
`Store.List`. `Filters.WithField` provides a fluent setter used by the TUI.

Birthday validation (`ValidateBirthday`) accepts only `YYYY-MM-DD` via
`time.Parse`. Normalization (`NormalizeAddress`) trims whitespace on all fields
and validates the birthday before any write.

#### `Service`

Thin orchestration layer. It owns a `*Store` and a `*Registry` and exposes the
operations the TUI needs:

- `ListAddresses(Filters) ([]Address, error)`
- `GetAddress(id) (Address, error)`
- `AddAddress(Address) (Address, error)`
- `UpdateAddress(id, Address) (Address, error)`
- `DeleteAddress(id) error`
- `ImportAddresses(path, format, ProgressFunc) error`
- `ExportAddresses(path, format, Filters, ProgressFunc) error`
- `AvailableFormats() []string`

#### `Store`

Wraps `database/sql` with a `modernc.org/sqlite` driver. Key design decisions:

- **Schema migration**: `Init()` creates the table if absent, then checks for
  missing columns via `PRAGMA table_info` and applies `ALTER TABLE` as needed.
  This keeps the migration logic simple and avoids a versioned migration
  framework for a single-table schema.
- **Duplicate detection**: `duplicateExists` compares all fields (excluding ID)
  before insert and update to prevent exact duplicates.
- **Filtering**: `List` builds a parameterised `WHERE` clause dynamically using
  `LOWER(column) LIKE LOWER(?)` for case-insensitive substring matching.
- **Ordering**: Results are always sorted by `LOWER(name), LOWER(email)`.

#### `Registry` and `FormatHandler`

```go
type FormatHandler interface {
    Format() string
    Import(path string, sink AddressInserter, progress ProgressFunc) error
    Export(path string, addresses []Address, progress ProgressFunc) error
}
```

`Registry` is a map from format string to handler. Handlers are registered at
startup via `NewRegistry()`. Adding a new format requires only implementing
`FormatHandler` and calling `registry.Register`.

`ProgressFunc` is `func(completed int, total *int)`. Passing `nil` is safe;
`reportProgress` guards the call. `total` is a pointer so handlers can report
progress before the total is known (e.g., streaming parsers).

#### Import behaviour (all plugins)

- Records missing `name` or `email` are skipped.
- Records with an invalid `birthday` are skipped.
- Duplicates are silently skipped (the store returns an error; the plugin
  ignores it and continues).
- Progress is reported for every record, including skipped ones.

### `internal/tui`

Built on [tview](https://github.com/rivo/tview) and
[tcell](https://github.com/gdamore/tcell).

#### Layout

```
┌─ Title ──────────────────────────────────────────────────────┐
│ ┌─ Sidebar ──────────┐  ┌─ Toolbar ──────────────────────── ┐│
│ │ Selected details   │  │ Add | Edit | Delete | Import | Exp ││
│ │                    │  ├─ Addresses ──────────────────────── ┤│
│ │ Stats              │  │ Name | Email | Birthday | … | ID   ││
│ │ Filter hint        │  │ …                                  ││
│ │ Filter field/input │  │                                    ││
│ │ Apply | Clear      │  │                                    ││
│ │ Import | Export    │  │                                    ││
│ └────────────────────┘  └────────────────────────────────────┘│
├─ Help ────────────────────────────────────────────────────────┤
├─ Status ──────────────────────────────────────────────────────┤
└───────────────────────────────────────────────────────────────┘
```

#### Focus management

`App.focusOrder` is a slice of `tview.Primitive` values. `Tab` / `Shift+Tab`
cycle through this slice via `cycleFocus`. Global shortcuts (`a`, `e`, `d`,
`/`, `i`, `x`, `r`, `q`) are captured at the application level but suppressed
when an `InputField` or `DropDown` has focus.

#### Modal system

`showModal(name, primitive, width, height, focus)` adds a page on top of the
main page and sets `modalOpen = true`. `hideModal(name)` removes the page and
returns focus to the table. While a modal is open, global shortcuts are
suppressed.

#### Import / Export flow

1. User triggers import or export (toolbar button, keyboard shortcut, or
   sidebar dropdown).
2. `showTransferForm` opens a modal form to collect path and format.
3. On submit, `runTransfer` opens a progress modal and launches a goroutine.
4. The goroutine calls `service.ImportAddresses` or `service.ExportAddresses`,
   passing a `ProgressFunc` that calls `application.QueueUpdateDraw` to update
   the progress modal safely from the goroutine.
5. On completion, the progress modal is closed and the table is refreshed (for
   import) or a status message is shown (for export).

---

## Data Model

### SQLite schema

```sql
CREATE TABLE IF NOT EXISTS addresses (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    name     TEXT NOT NULL,
    email    TEXT NOT NULL,
    birthday TEXT NOT NULL DEFAULT '',
    address  TEXT NOT NULL DEFAULT '',
    phone    TEXT NOT NULL DEFAULT '',
    mobile   TEXT NOT NULL DEFAULT '',
    custom   TEXT NOT NULL DEFAULT '',
    notes    TEXT NOT NULL DEFAULT ''
);
```

All optional fields default to empty string rather than `NULL` to simplify
application-level handling.

### Duplicate detection

Two records are considered duplicates when all eight non-ID fields match
exactly (after normalisation). This is intentionally strict: two contacts with
the same name and email but different phone numbers are not duplicates.

---

## Plugin Details

### CSV

- Header row is required: `name,email,birthday,address,phone,mobile,custom,notes`.
- Column order is determined by the header, not position.
- Extra columns are ignored; missing optional columns default to empty string.

### JSON

- Top-level value must be a JSON array of objects.
- Object keys match the field names above.
- `name` and `email` are required; all others are optional.

### HTML

- Import reads `<li>` elements with `data-*` attributes:
  `data-name`, `data-email`, `data-birthday`, `data-phone`, `data-mobile`,
  `data-custom`, `data-notes`. The text content of the `<li>` is used as the
  `address` field.
- Export writes one `<li>` per address in the same format. The output is a
  fragment (no `<html>/<body>` wrapper) intended for embedding or round-tripping
  through Yaran itself.

### VCard

- Import supports vCard 3.0 and 4.0.
- Line unfolding (RFC 6350 §3.2) is applied before parsing.
- `FN` is preferred for the name; `N` is used as a fallback and reconstructed
  into a display name.
- Multiple `EMAIL` properties: the one with the lowest `PREF` parameter wins;
  ties are broken by document order.
- Multiple `TEL` properties: `TYPE=cell` / `TYPE=mobile` map to `mobile`;
  everything else maps to `phone`. Preference and document order break ties.
- `CATEGORIES` maps to the `custom` field as a semicolon-separated list.
- `X-YARAN-CUSTOM` and `X-DOOST-CUSTOM` are accepted as legacy fallbacks for
  `custom` when no `CATEGORIES` are present.
- Export writes vCard 4.0. Lines are folded at 75 bytes (RFC 6350 §3.2),
  respecting UTF-8 rune boundaries.
- `BDAY` is written in basic date format (`YYYYMMDD`).
- `custom` values are split on `;` and newlines and written as `CATEGORIES`.

---

## Error Handling

- All errors are wrapped with `fmt.Errorf("context: %w", err)` for stack-aware
  unwrapping.
- The TUI displays errors in the status bar and, for blocking operations, in an
  error modal.
- Import plugins skip invalid records rather than aborting the whole import.

---

## Testing

Tests live alongside the code they test (`_test.go` files in the same package).

| Test file | Coverage |
|---|---|
| `internal/yaran/yaran_test.go` | Birthday validation, Store CRUD + filtering, JSON round-trip, VCard parser |
| `internal/tui/app_test.go` | Tab focus cycling, find shortcut, import menu reset, path placeholder |

Run all tests:

```bash
go test ./...
```

---

## Dependency Summary

| Dependency | Purpose |
|---|---|
| `modernc.org/sqlite` | CGo-free SQLite driver |
| `github.com/rivo/tview` | TUI widget library |
| `github.com/gdamore/tcell/v2` | Terminal cell rendering and input |

All other entries in `go.sum` are transitive dependencies of the above.

---

## Future Considerations

- **Multi-field search**: the current filter applies to one field at a time. A
  free-text search across all fields would improve discoverability.
- **Schema versioning**: a lightweight migration table (e.g., storing a schema
  version integer) would make future schema changes safer than the current
  column-existence probe.
- **Export filtering**: export already respects the active `Filters`; exposing
  this more prominently in the UI would be useful.
- **vCard import streaming**: large `.vcf` files are read entirely into memory.
  A line-by-line streaming parser would reduce peak memory usage.

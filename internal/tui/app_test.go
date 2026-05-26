package tui

import (
	"path/filepath"
	"testing"

	"github.com/gdamore/tcell/v2"
	"yaran-go/internal/yaran"
)

func newTestApp(t *testing.T) *App {
	t.Helper()

	service, err := yaran.NewService(filepath.Join(t.TempDir(), "yaran.db"))
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	t.Cleanup(func() {
		_ = service.Close()
	})

	if err := service.InitializeDatabase(); err != nil {
		t.Fatalf("init database: %v", err)
	}

	return New(service)
}

func TestTabCyclesMainFocusOrder(t *testing.T) {
	ui := newTestApp(t)
	capture := ui.application.GetInputCapture()
	if capture == nil {
		t.Fatal("expected application input capture")
	}

	ui.application.SetFocus(ui.table)
	capture(tcell.NewEventKey(tcell.KeyTAB, 0, tcell.ModNone))
	if ui.application.GetFocus() != ui.filterField {
		t.Fatalf("tab from table should focus filter field, got %T", ui.application.GetFocus())
	}

	capture(tcell.NewEventKey(tcell.KeyTAB, 0, tcell.ModNone))
	if ui.application.GetFocus() != ui.filterInput {
		t.Fatalf("second tab should focus filter input, got %T", ui.application.GetFocus())
	}

	capture(tcell.NewEventKey(tcell.KeyBacktab, 0, tcell.ModNone))
	if ui.application.GetFocus() != ui.filterField {
		t.Fatalf("backtab should return to filter field, got %T", ui.application.GetFocus())
	}
}

func TestFindShortcutFocusesFilterInputAndUsesSelectedField(t *testing.T) {
	ui := newTestApp(t)
	capture := ui.application.GetInputCapture()
	if capture == nil {
		t.Fatal("expected application input capture")
	}

	ui.filterField.SetCurrentOption(1)
	ui.filterInput.SetText("person@example.com")
	ui.application.SetFocus(ui.table)

	capture(tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone))
	if ui.application.GetFocus() != ui.filterInput {
		t.Fatalf("find shortcut should focus filter input, got %T", ui.application.GetFocus())
	}

	filters := ui.currentFilters()
	if filters.Email != "person@example.com" {
		t.Fatalf("expected email filter to be set, got %+v", filters)
	}
	if filters.Name != "" {
		t.Fatalf("expected name filter to stay empty, got %+v", filters)
	}
}

func TestImportMenuSelectionResetsAndOpensModal(t *testing.T) {
	ui := newTestApp(t)

	ui.importFormat.SetCurrentOption(0)

	index, option := ui.importFormat.GetCurrentOption()
	if index != -1 || option != "" {
		t.Fatalf("import menu should reset after selection, got index=%d option=%q", index, option)
	}
	if !ui.pages.HasPage("modal") || !ui.modalOpen {
		t.Fatal("expected import menu selection to open a modal")
	}
}

func TestTransferPathPlaceholder(t *testing.T) {
	if got := transferPathPlaceholder(""); got != "/path/to/file" {
		t.Fatalf("unexpected empty-format placeholder: %q", got)
	}
	if got := transferPathPlaceholder("json"); got != "/path/to/file.json" {
		t.Fatalf("unexpected json placeholder: %q", got)
	}
}

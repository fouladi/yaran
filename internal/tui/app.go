package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"yaran-go/internal/yaran"
)

var (
	filterLabels = []string{"Full name", "Email", "Birthday", "Phone", "Mobile", "Address", "Custom"}
	filterKeys   = []string{"name", "email", "birthday", "phone", "mobile", "address", "custom"}
	filterHints  = map[string]string{
		"name":     "Full name contains...",
		"email":    "Email contains...",
		"birthday": "Birthday: YYYY-MM-DD",
		"phone":    "Phone contains...",
		"mobile":   "Mobile contains...",
		"address":  "Address contains...",
		"custom":   "Custom contains...",
	}
)

type App struct {
	application  *tview.Application
	service      *yaran.Service
	pages        *tview.Pages
	table        *tview.Table
	details      *tview.TextView
	stats        *tview.TextView
	status       *tview.TextView
	filterHint   *tview.TextView
	filterField  *tview.DropDown
	filterInput  *tview.InputField
	importFormat *tview.DropDown
	exportFormat *tview.DropDown
	addresses    []yaran.Address
	selectedID   int64
	formats      []string
	focusOrder   []tview.Primitive
	modalOpen    bool
}

func New(service *yaran.Service) *App {
	ui := &App{
		application: tview.NewApplication(),
		service:     service,
		pages:       tview.NewPages(),
		formats:     service.AvailableFormats(),
	}
	ui.application.EnableMouse(true)
	ui.build()
	return ui
}

func (ui *App) Run() error {
	ui.refreshTable(0)
	ui.application.SetFocus(ui.table)
	return ui.application.SetRoot(ui.pages, true).Run()
}

func (ui *App) build() {
	header := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[::b]Yaran[::-]  Address book manager in Go")
	header.SetBorder(true).SetTitle("Title")

	ui.details = tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	ui.details.SetBorder(true).SetTitle("Selected")
	ui.details.SetText("No address selected.")

	ui.stats = tview.NewTextView().SetDynamicColors(true)
	ui.stats.SetBorder(true).SetTitle("Stats")

	ui.filterHint = tview.NewTextView().SetDynamicColors(true)
	ui.filterHint.SetBorder(true).SetTitle("Find")

	ui.filterField = tview.NewDropDown().
		SetLabel("Field ").
		SetFieldWidth(12).
		SetOptions(filterLabels, func(option string, index int) {
			ui.syncFilterHint()
		})

	ui.filterInput = tview.NewInputField().
		SetLabel("Value ").
		SetPlaceholder(filterHints["name"]).
		SetFieldWidth(12).
		SetDoneFunc(func(key tcell.Key) {
			if key == tcell.KeyEnter {
				ui.refreshTable(0)
			}
		})
	ui.filterField.SetCurrentOption(0)

	applyButton := tview.NewButton("Apply").SetSelectedFunc(func() {
		ui.refreshTable(0)
	})
	clearButton := tview.NewButton("Clear").SetSelectedFunc(func() {
		ui.filterField.SetCurrentOption(0)
		ui.filterInput.SetText("")
		ui.syncFilterHint()
		ui.refreshTable(0)
	})

	ui.importFormat = ui.newTransferMenu("Import", func(format string) {
		ui.openImportScreen(format, true)
	})
	ui.exportFormat = ui.newTransferMenu("Export", func(format string) {
		ui.openExportScreen(format, true)
	})

	filterRow := tview.NewFlex().
		AddItem(ui.filterField, 18, 0, false).
		AddItem(nil, 2, 0, false).
		AddItem(ui.filterInput, 18, 0, false)
	filterActions := tview.NewFlex().
		AddItem(applyButton, 18, 0, false).
		AddItem(nil, 2, 0, false).
		AddItem(clearButton, 18, 0, false)
	fileActions := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.importFormat, 1, 0, false).
		AddItem(ui.exportFormat, 1, 0, false)
	sidebarDivider := tview.NewTextView().
		SetDynamicColors(true).
		SetText("[darkgray]----------------------------------------")

	sidebar := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(ui.details, 0, 3, false).
		AddItem(ui.stats, 3, 0, false).
		AddItem(ui.filterHint, 3, 0, false).
		AddItem(filterRow, 1, 0, false).
		AddItem(filterActions, 1, 0, false).
		AddItem(sidebarDivider, 1, 0, false).
		AddItem(fileActions, 2, 0, false)
	sidebar.SetBorder(true).SetTitle("Sidebar")
	sidebar.SetBorderPadding(0, 0, 1, 1)

	toolbarAdd := tview.NewButton("Add").SetSelectedFunc(ui.openAddForm)
	toolbarEdit := tview.NewButton("Edit").SetSelectedFunc(ui.openEditForm)
	toolbarDelete := tview.NewButton("Delete").SetSelectedFunc(ui.deleteSelected)
	toolbarImport := tview.NewButton("Import").SetSelectedFunc(func() {
		ui.openImportScreen(ui.defaultTransferFormat(), false)
	})
	toolbarExport := tview.NewButton("Export").SetSelectedFunc(func() {
		ui.openExportScreen(ui.defaultTransferFormat(), false)
	})
	toolbar := tview.NewFlex().
		AddItem(toolbarAdd, 0, 1, false).
		AddItem(toolbarEdit, 0, 1, false).
		AddItem(toolbarDelete, 0, 1, false).
		AddItem(toolbarImport, 0, 1, false).
		AddItem(toolbarExport, 0, 1, false)

	ui.table = tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetFixed(1, 0)
	ui.table.SetBorder(true).SetTitle("Addresses")
	ui.table.SetSelectionChangedFunc(func(row int, column int) {
		if row <= 0 || row-1 >= len(ui.addresses) {
			return
		}
		ui.selectedID = ui.addresses[row-1].ID
		ui.refreshSidebar()
	})
	ui.table.SetSelectedFunc(func(row int, column int) {
		if row <= 0 || row-1 >= len(ui.addresses) {
			return
		}
		ui.showDetails(ui.addresses[row-1])
	})

	mainPane := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(toolbar, 1, 0, false).
		AddItem(ui.table, 0, 1, true)

	content := tview.NewFlex().
		AddItem(sidebar, 42, 0, false).
		AddItem(mainPane, 0, 1, true)

	help := tview.NewTextView().
		SetDynamicColors(true).
		SetText("Keys: [::b]a[::-] add  [::b]e[::-] edit  [::b]d[::-] delete  [::b]/[::-] find  [::b]i[::-] import  [::b]x[::-] export  [::b]tab[::-] focus  [::b]r[::-] refresh  [::b]q[::-] quit")
	help.SetBorder(true).SetTitle("Help")

	ui.status = tview.NewTextView().SetDynamicColors(true)
	ui.status.SetBorder(true).SetTitle("Status")
	ui.setStatus("Ready.")

	root := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 3, 0, false).
		AddItem(content, 0, 1, true).
		AddItem(help, 3, 0, false).
		AddItem(ui.status, 3, 0, false)

	ui.pages.AddPage("main", root, true, true)
	ui.focusOrder = []tview.Primitive{
		ui.table,
		ui.filterField,
		ui.filterInput,
		applyButton,
		clearButton,
		ui.importFormat,
		ui.exportFormat,
		toolbarAdd,
		toolbarEdit,
		toolbarDelete,
		toolbarImport,
		toolbarExport,
	}
	ui.syncFilterHint()
	ui.installGlobalShortcuts()
}

func (ui *App) installGlobalShortcuts() {
	ui.application.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if ui.modalOpen {
			return event
		}

		switch event.Key() {
		case tcell.KeyTAB:
			ui.cycleFocus(1)
			return nil
		case tcell.KeyBacktab:
			ui.cycleFocus(-1)
			return nil
		}

		switch ui.application.GetFocus().(type) {
		case *tview.InputField, *tview.DropDown:
			return event
		}

		switch {
		case event.Key() == tcell.KeyCtrlC:
			ui.application.Stop()
			return nil
		case event.Rune() == 'q':
			ui.application.Stop()
			return nil
		case event.Rune() == 'r':
			ui.refreshTable(ui.selectedID)
			ui.setStatus("Addresses refreshed.")
			return nil
		case event.Rune() == 'a':
			ui.openAddForm()
			return nil
		case event.Rune() == 'e':
			ui.openEditForm()
			return nil
		case event.Rune() == 'd':
			ui.deleteSelected()
			return nil
		case event.Rune() == '/' || event.Rune() == 'f':
			ui.application.SetFocus(ui.filterInput)
			return nil
		case event.Rune() == 'i':
			ui.openImportScreen(ui.defaultTransferFormat(), false)
			return nil
		case event.Rune() == 'x':
			ui.openExportScreen(ui.defaultTransferFormat(), false)
			return nil
		}

		return event
	})
}

func (ui *App) selectedFilterField() string {
	index, _ := ui.filterField.GetCurrentOption()
	if index >= 0 && index < len(filterKeys) {
		return filterKeys[index]
	}
	return "name"
}

func (ui *App) currentFilters() yaran.Filters {
	value := strings.TrimSpace(ui.filterInput.GetText())
	if value == "" {
		return yaran.Filters{}
	}
	return yaran.Filters{}.WithField(ui.selectedFilterField(), value)
}

func (ui *App) syncFilterHint() {
	field := ui.selectedFilterField()
	ui.filterHint.SetText(filterHints[field])
	ui.filterInput.SetPlaceholder(filterHints[field])
}

func (ui *App) newTransferMenu(label string, onSelect func(format string)) *tview.DropDown {
	var menu *tview.DropDown
	menu = tview.NewDropDown().
		SetLabel(label+" ").
		SetOptions(ui.formats, func(option string, index int) {
			if index < 0 {
				return
			}
			menu.SetCurrentOption(-1)
			onSelect(option)
		}).
		SetTextOptions("", "", "", "", "format...")
	menu.SetCurrentOption(-1)
	return menu
}

func (ui *App) cycleFocus(step int) {
	if len(ui.focusOrder) == 0 {
		return
	}

	current := ui.application.GetFocus()
	index := -1
	for candidateIndex, primitive := range ui.focusOrder {
		if current == primitive {
			index = candidateIndex
			break
		}
	}

	if index < 0 {
		if step < 0 {
			ui.application.SetFocus(ui.focusOrder[len(ui.focusOrder)-1])
			return
		}
		ui.application.SetFocus(ui.focusOrder[0])
		return
	}

	next := (index + step + len(ui.focusOrder)) % len(ui.focusOrder)
	ui.application.SetFocus(ui.focusOrder[next])
}

func (ui *App) defaultTransferFormat() string {
	if len(ui.formats) == 0 {
		return ""
	}
	return ui.formats[0]
}

func (ui *App) refreshTable(preferredID int64) {
	addresses, err := ui.service.ListAddresses(ui.currentFilters())
	if err != nil {
		ui.showError(err)
		return
	}

	ui.addresses = addresses
	ui.table.Clear()
	ui.addHeader()

	selectedRow := 1
	for index, address := range ui.addresses {
		row := index + 1
		ui.table.SetCell(row, 0, tview.NewTableCell(address.Name))
		ui.table.SetCell(row, 1, tview.NewTableCell(address.Email))
		ui.table.SetCell(row, 2, tview.NewTableCell(address.Birthday))
		ui.table.SetCell(row, 3, tview.NewTableCell(address.Phone))
		ui.table.SetCell(row, 4, tview.NewTableCell(address.Mobile))
		ui.table.SetCell(row, 5, tview.NewTableCell(address.Custom))
		ui.table.SetCell(row, 6, tview.NewTableCell(fmt.Sprintf("%d", address.ID)))

		if preferredID > 0 && address.ID == preferredID {
			selectedRow = row
		}
	}

	if len(ui.addresses) == 0 {
		ui.selectedID = 0
		ui.refreshSidebar()
		return
	}

	ui.table.Select(selectedRow, 0)
	ui.selectedID = ui.addresses[selectedRow-1].ID
	ui.refreshSidebar()
}

func (ui *App) addHeader() {
	headers := []string{"Name", "Email", "Birthday", "Phone", "Mobile", "Custom", "ID"}
	for column, value := range headers {
		cell := tview.NewTableCell(value).
			SetSelectable(false).
			SetTextColor(tcell.ColorYellow).
			SetAttributes(tcell.AttrBold)
		ui.table.SetCell(0, column, cell)
	}
}

func (ui *App) selectedAddress() (yaran.Address, bool) {
	for _, address := range ui.addresses {
		if address.ID == ui.selectedID {
			return address, true
		}
	}
	return yaran.Address{}, false
}

func (ui *App) refreshSidebar() {
	ui.stats.SetText(fmt.Sprintf("%d address(es) loaded", len(ui.addresses)))

	address, ok := ui.selectedAddress()
	if !ok {
		ui.details.SetText("No address selected.")
		return
	}

	ui.details.SetText(formatAddressDetails(address))
}

func (ui *App) openAddForm() {
	ui.showAddressForm("Add address", yaran.Address{}, func(address yaran.Address) error {
		added, err := ui.service.AddAddress(address)
		if err != nil {
			return err
		}
		ui.refreshTable(added.ID)
		ui.setStatus(fmt.Sprintf("Added %q.", added.Name))
		return nil
	})
}

func (ui *App) openEditForm() {
	address, ok := ui.selectedAddress()
	if !ok {
		ui.setStatus("Select an address first.")
		return
	}

	ui.showAddressForm("Edit address", address, func(updated yaran.Address) error {
		saved, err := ui.service.UpdateAddress(address.ID, updated)
		if err != nil {
			return err
		}
		ui.refreshTable(saved.ID)
		ui.setStatus(fmt.Sprintf("Updated %q.", saved.Name))
		return nil
	})
}

func (ui *App) deleteSelected() {
	address, ok := ui.selectedAddress()
	if !ok {
		ui.setStatus("Select an address first.")
		return
	}

	modal := tview.NewModal().
		SetText(fmt.Sprintf("Delete %q?", address.Name)).
		AddButtons([]string{"Delete", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			ui.hideModal("modal")
			if buttonLabel != "Delete" {
				return
			}
			if err := ui.service.DeleteAddress(address.ID); err != nil {
				ui.showError(err)
				return
			}
			ui.refreshTable(0)
			ui.setStatus(fmt.Sprintf("Deleted %q.", address.Name))
		})

	ui.showModal("modal", modal, 60, 10, modal)
}

func (ui *App) showDetails(address yaran.Address) {
	details := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(true).
		SetText(formatAddressDetails(address))
	details.SetBorder(true).SetTitle(address.Name)
	details.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Rune() == 'q' {
			ui.hideModal("modal")
			return nil
		}
		return event
	})

	ui.showModal("modal", details, 74, 16, details)
}

func (ui *App) showAddressForm(title string, initial yaran.Address, onSave func(yaran.Address) error) {
	nameField := tview.NewInputField().SetLabel("Name ").SetText(initial.Name).SetFieldWidth(32)
	emailField := tview.NewInputField().SetLabel("Email ").SetText(initial.Email).SetFieldWidth(32)
	birthdayField := tview.NewInputField().SetLabel("Birthday ").SetText(initial.Birthday).SetFieldWidth(16)
	phoneField := tview.NewInputField().SetLabel("Phone ").SetText(initial.Phone).SetFieldWidth(24)
	mobileField := tview.NewInputField().SetLabel("Mobile ").SetText(initial.Mobile).SetFieldWidth(24)
	addressField := tview.NewInputField().SetLabel("Address ").SetText(initial.Address).SetFieldWidth(40)
	customField := tview.NewInputField().SetLabel("Custom ").SetText(initial.Custom).SetFieldWidth(40)
	notesField := tview.NewInputField().SetLabel("Notes ").SetText(initial.Notes).SetFieldWidth(40)

	form := tview.NewForm().
		AddFormItem(nameField).
		AddFormItem(emailField).
		AddFormItem(birthdayField).
		AddFormItem(phoneField).
		AddFormItem(mobileField).
		AddFormItem(addressField).
		AddFormItem(customField).
		AddFormItem(notesField)

	save := func() {
		address := yaran.Address{
			Name:     nameField.GetText(),
			Email:    emailField.GetText(),
			Birthday: birthdayField.GetText(),
			Phone:    phoneField.GetText(),
			Mobile:   mobileField.GetText(),
			Address:  addressField.GetText(),
			Custom:   customField.GetText(),
			Notes:    notesField.GetText(),
		}
		if err := onSave(address); err != nil {
			ui.setStatus(err.Error())
			return
		}
		ui.hideModal("modal")
	}

	form.AddButton("Save", save)
	form.AddButton("Cancel", func() {
		ui.hideModal("modal")
	})
	form.SetCancelFunc(func() {
		ui.hideModal("modal")
	})
	form.SetBorder(true).SetTitle(title)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			ui.hideModal("modal")
			return nil
		}
		return event
	})

	ui.showModal("modal", form, 82, 20, form)
}

func (ui *App) openImportScreen(defaultFormat string, fixedFormat bool) {
	ui.showTransferForm("Import addresses", defaultFormat, fixedFormat, func(path string, format string) {
		ui.runTransfer(
			"Import addresses",
			path,
			"Reading file...",
			func(update yaran.ProgressFunc) error {
				return ui.service.ImportAddresses(path, format, update)
			},
			func() {
				ui.refreshTable(0)
				ui.setStatus(fmt.Sprintf("Imported addresses from %s.", path))
			},
		)
	})
}

func (ui *App) openExportScreen(defaultFormat string, fixedFormat bool) {
	ui.showTransferForm("Export current results", defaultFormat, fixedFormat, func(path string, format string) {
		ui.runTransfer(
			"Export current results",
			path,
			"Collecting current results...",
			func(update yaran.ProgressFunc) error {
				return ui.service.ExportAddresses(path, format, ui.currentFilters(), update)
			},
			func() {
				ui.setStatus(fmt.Sprintf("Exported current results to %s.", path))
			},
		)
	})
}

func (ui *App) showTransferForm(title string, defaultFormat string, fixedFormat bool, onSubmit func(path string, format string)) {
	selectedFormat := defaultFormat
	if selectedFormat == "" {
		selectedFormat = ui.defaultTransferFormat()
	}

	pathField := tview.NewInputField().
		SetLabel("Path ").
		SetFieldWidth(48).
		SetPlaceholder(transferPathPlaceholder(selectedFormat))

	var formatField *tview.DropDown
	if !fixedFormat {
		formatField = tview.NewDropDown().
			SetLabel("Format ").
			SetOptions(ui.formats, func(option string, index int) {
				if index < 0 {
					return
				}
				pathField.SetPlaceholder(transferPathPlaceholder(option))
			})
		ui.setDropDownToFormat(formatField, selectedFormat)
	}

	form := tview.NewForm()
	if formatField != nil {
		form.AddFormItem(formatField)
	}
	form.AddFormItem(pathField)
	form.AddButton("Run", func() {
		path := strings.TrimSpace(pathField.GetText())
		if path == "" {
			ui.setStatus("A file path is required.")
			return
		}
		format := selectedFormat
		if formatField != nil {
			format = ui.selectedFormat(formatField)
		}
		ui.hideModal("modal")
		onSubmit(path, format)
	})
	form.AddButton("Cancel", func() {
		ui.hideModal("modal")
	})
	form.SetCancelFunc(func() {
		ui.hideModal("modal")
	})
	form.SetBorder(true).SetTitle(title)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			ui.hideModal("modal")
			return nil
		}
		return event
	})

	frame := tview.NewFrame(form)
	frame.AddText("Choose a format and file path.", true, tview.AlignLeft, tcell.ColorWhite)
	ui.showModal("modal", frame, 80, 10, form)
}

func (ui *App) runTransfer(title string, subtitle string, pending string, run func(yaran.ProgressFunc) error, onSuccess func()) {
	status := tview.NewTextView().SetDynamicColors(true).SetWrap(true)
	status.SetText(fmt.Sprintf("%s\n\n%s", subtitle, pending))
	status.SetBorder(true).SetTitle(title)
	ui.showModal("progress", status, 80, 10, status)

	go func() {
		err := run(func(completed int, total *int) {
			ui.application.QueueUpdateDraw(func() {
				status.SetText(fmt.Sprintf("%s\n\n%s", subtitle, progressStatusText(pending, completed, total)))
			})
		})

		ui.application.QueueUpdateDraw(func() {
			ui.hideModal("progress")
			if err != nil {
				ui.showError(err)
				return
			}
			onSuccess()
		})
	}()
}

func (ui *App) selectedFormat(dropdown *tview.DropDown) string {
	index, option := dropdown.GetCurrentOption()
	if index < 0 {
		return ui.defaultTransferFormat()
	}
	return option
}

func (ui *App) setDropDownToFormat(dropdown *tview.DropDown, format string) {
	for index, item := range ui.formats {
		if item == format {
			dropdown.SetCurrentOption(index)
			return
		}
	}
	if len(ui.formats) > 0 {
		dropdown.SetCurrentOption(0)
	}
}

func (ui *App) showModal(name string, primitive tview.Primitive, width int, height int, focus tview.Primitive) {
	ui.modalOpen = true
	ui.pages.AddPage(name, center(width, height, primitive), true, true)
	ui.application.SetFocus(focus)
}

func (ui *App) hideModal(name string) {
	ui.pages.RemovePage(name)
	ui.modalOpen = false
	ui.application.SetFocus(ui.table)
}

func (ui *App) setStatus(message string) {
	ui.status.SetText(message)
}

func (ui *App) showError(err error) {
	ui.setStatus(err.Error())
	modal := tview.NewModal().
		SetText(err.Error()).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			ui.hideModal("modal")
		})
	ui.showModal("modal", modal, 70, 10, modal)
}

func formatAddressDetails(address yaran.Address) string {
	valueOrDash := func(value string) string {
		if strings.TrimSpace(value) == "" {
			return "-"
		}
		return value
	}

	return strings.Join([]string{
		fmt.Sprintf("ID: %d", address.ID),
		fmt.Sprintf("Name: %s", address.Name),
		fmt.Sprintf("Email: %s", address.Email),
		fmt.Sprintf("Birthday: %s", valueOrDash(address.Birthday)),
		fmt.Sprintf("Phone: %s", valueOrDash(address.Phone)),
		fmt.Sprintf("Mobile: %s", valueOrDash(address.Mobile)),
		fmt.Sprintf("Address: %s", valueOrDash(address.Address)),
		fmt.Sprintf("Custom: %s", valueOrDash(address.Custom)),
		fmt.Sprintf("Notes: %s", valueOrDash(address.Notes)),
	}, "\n")
}

func progressStatusText(pending string, completed int, total *int) string {
	if total == nil {
		return pending
	}
	noun := "addresses"
	if *total == 1 {
		noun = "address"
	}
	return fmt.Sprintf("%d of %d %s", completed, *total, noun)
}

func transferPathPlaceholder(fileFormat string) string {
	if strings.TrimSpace(fileFormat) == "" {
		return "/path/to/file"
	}
	return fmt.Sprintf("/path/to/file.%s", fileFormat)
}

func center(width int, height int, primitive tview.Primitive) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(
			tview.NewFlex().SetDirection(tview.FlexRow).
				AddItem(nil, 0, 1, false).
				AddItem(primitive, height, 1, true).
				AddItem(nil, 0, 1, false),
			width,
			1,
			true,
		).
		AddItem(nil, 0, 1, false)
}

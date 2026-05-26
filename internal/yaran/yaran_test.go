package yaran

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBirthday(t *testing.T) {
	value, err := ValidateBirthday("1990-01-02")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "1990-01-02" {
		t.Fatalf("unexpected value: %s", value)
	}

	if _, err := ValidateBirthday("1990-99-99"); err == nil {
		t.Fatal("expected invalid birthday error")
	}
}

func TestStoreCRUDAndFiltering(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "yaran.db"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	defer store.Close()

	if err := store.Init(); err != nil {
		t.Fatalf("init store: %v", err)
	}

	first, err := store.Insert(Address{
		Name:     "Alice Example",
		Email:    "alice@example.com",
		Birthday: "1990-01-02",
		Address:  "Main Street 1",
		Phone:    "111",
		Mobile:   "211",
		Custom:   "family",
		Notes:    "Primary",
	})
	if err != nil {
		t.Fatalf("insert first: %v", err)
	}

	if _, err := store.Insert(Address{
		Name:  "Bob Example",
		Email: "bob@example.com",
	}); err != nil {
		t.Fatalf("insert second: %v", err)
	}

	rows, err := store.List(Filters{Name: "Alice"})
	if err != nil {
		t.Fatalf("filter list: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != first.ID {
		t.Fatalf("unexpected filtered rows: %+v", rows)
	}

	updated, err := store.Update(first.ID, Address{
		Name:     "Alice Changed",
		Email:    "alice@example.com",
		Birthday: "1990-01-02",
		Address:  "Main Street 1",
		Phone:    "111",
		Mobile:   "211",
		Custom:   "client",
		Notes:    "Updated",
	})
	if err != nil {
		t.Fatalf("update address: %v", err)
	}
	if updated.Name != "Alice Changed" {
		t.Fatalf("unexpected updated name: %s", updated.Name)
	}

	if err := store.Delete(first.ID); err != nil {
		t.Fatalf("delete address: %v", err)
	}

	rows, err = store.List(Filters{})
	if err != nil {
		t.Fatalf("list remaining rows: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "Bob Example" {
		t.Fatalf("unexpected remaining rows: %+v", rows)
	}
}

func TestJSONPluginExportAndImport(t *testing.T) {
	plugin := JSONPlugin{}
	path := filepath.Join(t.TempDir(), "addresses.json")

	addresses := []Address{
		{
			Name:     "Alice",
			Email:    "alice@example.com",
			Birthday: "1990-01-02",
			Address:  "Main Street 1",
			Phone:    "111",
			Mobile:   "211",
			Custom:   "family",
			Notes:    "Primary",
		},
	}

	if err := plugin.Export(path, addresses, nil); err != nil {
		t.Fatalf("export json: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read export: %v", err)
	}

	var payload []map[string]string
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("parse export: %v", err)
	}
	if len(payload) != 1 || payload[0]["email"] != "alice@example.com" {
		t.Fatalf("unexpected payload: %+v", payload)
	}

	store, err := NewStore(filepath.Join(t.TempDir(), "import.db"))
	if err != nil {
		t.Fatalf("new import store: %v", err)
	}
	defer store.Close()
	if err := store.Init(); err != nil {
		t.Fatalf("init import store: %v", err)
	}

	if err := plugin.Import(path, store, nil); err != nil {
		t.Fatalf("import json: %v", err)
	}

	rows, err := store.List(Filters{})
	if err != nil {
		t.Fatalf("list imported rows: %v", err)
	}
	if len(rows) != 1 || rows[0].Email != "alice@example.com" {
		t.Fatalf("unexpected imported rows: %+v", rows)
	}
}

func TestVCardParserExtractsExpectedFields(t *testing.T) {
	address, err := cardToAddress([]string{
		"VERSION:4.0",
		"FN:Alice Example",
		"N:Example;Alice;;;",
		"BDAY:19900102",
		"ADR;TYPE=home:;;Main Street 1;Frankfurt;;60322;Germany",
		`TEL;VALUE=uri;TYPE="voice,home":tel:+49-111`,
		"TEL;VALUE=text;TYPE=cell:211",
		"EMAIL;PREF=1:alice@example.com",
		"EMAIL:alias@example.com",
		"CATEGORIES:family,client",
		`NOTE:Primary\nContact`,
	})
	if err != nil {
		t.Fatalf("parse vcard: %v", err)
	}

	if address.Name != "Alice Example" {
		t.Fatalf("unexpected name: %s", address.Name)
	}
	if address.Email != "alice@example.com" {
		t.Fatalf("unexpected email: %s", address.Email)
	}
	if address.Birthday != "1990-01-02" {
		t.Fatalf("unexpected birthday: %s", address.Birthday)
	}
	if address.Address != "Main Street 1, Frankfurt, 60322, Germany" {
		t.Fatalf("unexpected address: %s", address.Address)
	}
	if address.Phone != "+49-111" || address.Mobile != "211" {
		t.Fatalf("unexpected phones: %+v", address)
	}
	if address.Custom != "family;client" {
		t.Fatalf("unexpected custom value: %s", address.Custom)
	}
	if address.Notes != "Primary\nContact" {
		t.Fatalf("unexpected notes: %q", address.Notes)
	}
}

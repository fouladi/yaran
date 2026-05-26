package yaran

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Init() error {
	schema := `
CREATE TABLE IF NOT EXISTS addresses (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	email TEXT NOT NULL,
	birthday TEXT NOT NULL DEFAULT '',
	address TEXT NOT NULL DEFAULT '',
	phone TEXT NOT NULL DEFAULT '',
	mobile TEXT NOT NULL DEFAULT '',
	custom TEXT NOT NULL DEFAULT '',
	notes TEXT NOT NULL DEFAULT ''
)`
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	hasBirthday, err := s.hasColumn("addresses", "birthday")
	if err != nil {
		return err
	}
	if !hasBirthday {
		if _, err := s.db.Exec(`ALTER TABLE addresses ADD COLUMN birthday TEXT NOT NULL DEFAULT ''`); err != nil {
			return fmt.Errorf("migrate schema: %w", err)
		}
	}

	return nil
}

func (s *Store) hasColumn(table string, column string) (bool, error) {
	// PRAGMA does not support parameter binding; the table name is an internal
	// constant so this is safe, but we validate it to be explicit.
	if table != "addresses" {
		return false, fmt.Errorf("unexpected table name: %s", table)
	}
	rows, err := s.db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, fmt.Errorf("schema inspection: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var dataType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return false, fmt.Errorf("scan schema: %w", err)
		}
		if name == column {
			return true, nil
		}
	}

	return false, rows.Err()
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) List(filters Filters) ([]Address, error) {
	base := `SELECT id, name, email, birthday, address, phone, mobile, custom, notes FROM addresses`
	clauses := make([]string, 0, 8)
	args := make([]any, 0, 8)

	appendFilter := func(column string, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		clauses = append(clauses, fmt.Sprintf("LOWER(%s) LIKE LOWER(?)", column))
		args = append(args, "%"+strings.TrimSpace(value)+"%")
	}

	appendFilter("name", filters.Name)
	appendFilter("email", filters.Email)
	appendFilter("birthday", filters.Birthday)
	appendFilter("address", filters.Address)
	appendFilter("phone", filters.Phone)
	appendFilter("mobile", filters.Mobile)
	appendFilter("custom", filters.Custom)
	appendFilter("notes", filters.Notes)

	if len(clauses) > 0 {
		base += " WHERE " + strings.Join(clauses, " AND ")
	}
	base += " ORDER BY LOWER(name), LOWER(email)"

	rows, err := s.db.Query(base, args...)
	if err != nil {
		return nil, fmt.Errorf("list addresses: %w", err)
	}
	defer rows.Close()

	addresses := make([]Address, 0)
	for rows.Next() {
		address, err := scanAddress(rows)
		if err != nil {
			return nil, err
		}
		addresses = append(addresses, address)
	}

	return addresses, rows.Err()
}

func (s *Store) Get(id int64) (Address, error) {
	row := s.db.QueryRow(
		`SELECT id, name, email, birthday, address, phone, mobile, custom, notes FROM addresses WHERE id = ?`,
		id,
	)

	address, err := scanAddressRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Address{}, fmt.Errorf("address with id=%d not found", id)
		}
		return Address{}, err
	}

	return address, nil
}

func (s *Store) Insert(address Address) (Address, error) {
	address, err := NormalizeAddress(address)
	if err != nil {
		return Address{}, err
	}
	if address.Name == "" || address.Email == "" {
		return Address{}, fmt.Errorf("name and email are required")
	}

	duplicate, err := s.duplicateExists(address, 0)
	if err != nil {
		return Address{}, err
	}
	if duplicate {
		return Address{}, fmt.Errorf("address %q <%s> already exists", address.Name, address.Email)
	}

	result, err := s.db.Exec(
		`INSERT INTO addresses (name, email, birthday, address, phone, mobile, custom, notes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		address.Name,
		address.Email,
		address.Birthday,
		address.Address,
		address.Phone,
		address.Mobile,
		address.Custom,
		address.Notes,
	)
	if err != nil {
		return Address{}, fmt.Errorf("insert address: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return Address{}, fmt.Errorf("read inserted id: %w", err)
	}
	address.ID = id

	return address, nil
}

func (s *Store) Update(id int64, address Address) (Address, error) {
	address, err := NormalizeAddress(address)
	if err != nil {
		return Address{}, err
	}
	if address.Name == "" || address.Email == "" {
		return Address{}, fmt.Errorf("name and email are required")
	}

	_, err = s.Get(id)
	if err != nil {
		return Address{}, err
	}

	duplicate, err := s.duplicateExists(address, id)
	if err != nil {
		return Address{}, err
	}
	if duplicate {
		return Address{}, fmt.Errorf("address %q <%s> already exists", address.Name, address.Email)
	}

	if _, err := s.db.Exec(
		`UPDATE addresses
		 SET name = ?, email = ?, birthday = ?, address = ?, phone = ?, mobile = ?, custom = ?, notes = ?
		 WHERE id = ?`,
		address.Name,
		address.Email,
		address.Birthday,
		address.Address,
		address.Phone,
		address.Mobile,
		address.Custom,
		address.Notes,
		id,
	); err != nil {
		return Address{}, fmt.Errorf("update address: %w", err)
	}

	address.ID = id
	return address, nil
}

func (s *Store) Delete(id int64) error {
	result, err := s.db.Exec(`DELETE FROM addresses WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete address: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete address: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("address with id=%d not found", id)
	}
	return nil
}

func (s *Store) duplicateExists(address Address, excludeID int64) (bool, error) {
	query := `
SELECT EXISTS(
	SELECT 1
	FROM addresses
	WHERE name = ?
	  AND email = ?
	  AND birthday = ?
	  AND address = ?
	  AND phone = ?
	  AND mobile = ?
	  AND custom = ?
	  AND notes = ?`
	args := []any{
		address.Name,
		address.Email,
		address.Birthday,
		address.Address,
		address.Phone,
		address.Mobile,
		address.Custom,
		address.Notes,
	}

	if excludeID > 0 {
		query += " AND id != ?"
		args = append(args, excludeID)
	}
	query += ")"

	var exists bool
	if err := s.db.QueryRow(query, args...).Scan(&exists); err != nil {
		return false, fmt.Errorf("check duplicate: %w", err)
	}
	return exists, nil
}

func scanAddress(rows interface{ Scan(dest ...any) error }) (Address, error) {
	var address Address
	if err := rows.Scan(
		&address.ID,
		&address.Name,
		&address.Email,
		&address.Birthday,
		&address.Address,
		&address.Phone,
		&address.Mobile,
		&address.Custom,
		&address.Notes,
	); err != nil {
		return Address{}, fmt.Errorf("scan address: %w", err)
	}
	return address, nil
}

func scanAddressRow(row *sql.Row) (Address, error) {
	return scanAddress(row)
}

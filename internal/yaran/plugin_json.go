package yaran

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type JSONPlugin struct{}

func (JSONPlugin) Format() string {
	return "json"
}

func (JSONPlugin) Import(path string, sink AddressInserter, progress ProgressFunc) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read json: %w", err)
	}

	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parse json: %w", err)
	}

	items, ok := raw.([]any)
	if !ok {
		return fmt.Errorf("invalid JSON format")
	}

	total := len(items)
	reportProgress(progress, 0, intRef(total))

	for index, item := range items {
		record, ok := item.(map[string]any)
		if !ok {
			reportProgress(progress, index+1, intRef(total))
			continue
		}

		name, ok := jsonStringValue(record, "name", true)
		if !ok {
			reportProgress(progress, index+1, intRef(total))
			continue
		}
		email, ok := jsonStringValue(record, "email", true)
		if !ok {
			reportProgress(progress, index+1, intRef(total))
			continue
		}

		birthday, err := ValidateBirthday(jsonString(record, "birthday"))
		if err == nil {
			_, _ = sink.Insert(Address{
				Name:     strings.TrimSpace(name),
				Email:    strings.TrimSpace(email),
				Birthday: birthday,
				Address:  strings.TrimSpace(jsonString(record, "address")),
				Phone:    strings.TrimSpace(jsonString(record, "phone")),
				Mobile:   strings.TrimSpace(jsonString(record, "mobile")),
				Custom:   strings.TrimSpace(jsonString(record, "custom")),
				Notes:    strings.TrimSpace(jsonString(record, "notes")),
			})
		}

		reportProgress(progress, index+1, intRef(total))
	}

	return nil
}

func (JSONPlugin) Export(path string, addresses []Address, progress ProgressFunc) error {
	total := len(addresses)
	reportProgress(progress, 0, intRef(total))

	payload := make([]map[string]string, 0, len(addresses))
	for index, address := range addresses {
		payload = append(payload, map[string]string{
			"name":     address.Name,
			"email":    address.Email,
			"birthday": address.Birthday,
			"address":  address.Address,
			"phone":    address.Phone,
			"mobile":   address.Mobile,
			"custom":   address.Custom,
			"notes":    address.Notes,
		})
		reportProgress(progress, index+1, intRef(total))
	}

	body, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("encode json: %w", err)
	}

	if err := os.WriteFile(path, body, 0o644); err != nil {
		return fmt.Errorf("write json: %w", err)
	}

	return nil
}

func jsonStringValue(item map[string]any, key string, required bool) (string, bool) {
	raw, ok := item[key]
	if !ok {
		return "", !required
	}

	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	return value, true
}

func jsonString(item map[string]any, key string) string {
	value, ok := jsonStringValue(item, key, false)
	if !ok {
		return ""
	}
	return value
}

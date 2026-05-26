package yaran

import (
	"encoding/csv"
	"fmt"
	"os"
)

type CSVPlugin struct{}

func (CSVPlugin) Format() string {
	return "csv"
}

func (CSVPlugin) Import(path string, sink AddressInserter, progress ProgressFunc) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open csv: %w", err)
	}
	defer file.Close()

	rows, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return fmt.Errorf("read csv: %w", err)
	}
	if len(rows) == 0 {
		return nil
	}

	header := map[string]int{}
	for index, column := range rows[0] {
		header[column] = index
	}

	total := len(rows) - 1
	reportProgress(progress, 0, intRef(total))

	for index, row := range rows[1:] {
		recordIndex := index + 1

		name, ok := csvValue(row, header, "name")
		if !ok {
			reportProgress(progress, recordIndex, intRef(total))
			continue
		}
		email, ok := csvValue(row, header, "email")
		if !ok {
			reportProgress(progress, recordIndex, intRef(total))
			continue
		}

		birthday, err := ValidateBirthday(csvOptional(row, header, "birthday"))
		if err == nil {
			_, _ = sink.Insert(Address{
				Name:     name,
				Email:    email,
				Birthday: birthday,
				Address:  csvOptional(row, header, "address"),
				Phone:    csvOptional(row, header, "phone"),
				Mobile:   csvOptional(row, header, "mobile"),
				Custom:   csvOptional(row, header, "custom"),
				Notes:    csvOptional(row, header, "notes"),
			})
		}

		reportProgress(progress, recordIndex, intRef(total))
	}

	return nil
}

func (CSVPlugin) Export(path string, addresses []Address, progress ProgressFunc) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create csv: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	total := len(addresses)
	reportProgress(progress, 0, intRef(total))

	if err := writer.Write([]string{"name", "email", "birthday", "address", "phone", "mobile", "custom", "notes"}); err != nil {
		return fmt.Errorf("write csv header: %w", err)
	}

	for index, address := range addresses {
		if err := writer.Write([]string{
			address.Name,
			address.Email,
			address.Birthday,
			address.Address,
			address.Phone,
			address.Mobile,
			address.Custom,
			address.Notes,
		}); err != nil {
			return fmt.Errorf("write csv row: %w", err)
		}

		reportProgress(progress, index+1, intRef(total))
	}

	if err := writer.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}

	return nil
}

func csvValue(row []string, header map[string]int, key string) (string, bool) {
	index, ok := header[key]
	if !ok || index >= len(row) {
		return "", false
	}
	return row[index], true
}

func csvOptional(row []string, header map[string]int, key string) string {
	value, ok := csvValue(row, header, key)
	if !ok {
		return ""
	}
	return value
}

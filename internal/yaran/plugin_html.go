package yaran

import (
	"fmt"
	"html"
	"os"
	"regexp"
	"strings"
)

var (
	htmlListItemPattern = regexp.MustCompile(`(?is)<li\b([^>]*)>(.*?)</li>`)
	htmlAttrPattern     = regexp.MustCompile(`data-([a-z-]+)="([^"]*)"`)
)

type HTMLPlugin struct{}

func (HTMLPlugin) Format() string {
	return "html"
}

func (HTMLPlugin) Import(path string, sink AddressInserter, progress ProgressFunc) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read html: %w", err)
	}

	matches := htmlListItemPattern.FindAllStringSubmatch(string(body), -1)
	total := len(matches)
	reportProgress(progress, 0, intRef(total))

	for index, match := range matches {
		attrs := map[string]string{}
		for _, attr := range htmlAttrPattern.FindAllStringSubmatch(match[1], -1) {
			attrs[attr[1]] = html.UnescapeString(attr[2])
		}

		birthday, err := ValidateBirthday(strings.TrimSpace(attrs["birthday"]))
		if err != nil {
			reportProgress(progress, index+1, intRef(total))
			continue
		}

		_, _ = sink.Insert(Address{
			Name:     strings.TrimSpace(attrs["name"]),
			Email:    strings.TrimSpace(attrs["email"]),
			Birthday: birthday,
			Address:  strings.TrimSpace(html.UnescapeString(match[2])),
			Phone:    strings.TrimSpace(attrs["phone"]),
			Mobile:   strings.TrimSpace(attrs["mobile"]),
			Custom:   strings.TrimSpace(attrs["custom"]),
			Notes:    strings.TrimSpace(attrs["notes"]),
		})

		reportProgress(progress, index+1, intRef(total))
	}

	return nil
}

func (HTMLPlugin) Export(path string, addresses []Address, progress ProgressFunc) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create html: %w", err)
	}
	defer file.Close()

	total := len(addresses)
	reportProgress(progress, 0, intRef(total))

	for index, address := range addresses {
		line := fmt.Sprintf(
			`<li data-name="%s" data-email="%s" data-birthday="%s" data-phone="%s" data-mobile="%s" data-custom="%s" data-notes="%s">%s</li>`+"\n",
			html.EscapeString(address.Name),
			html.EscapeString(address.Email),
			html.EscapeString(address.Birthday),
			html.EscapeString(address.Phone),
			html.EscapeString(address.Mobile),
			html.EscapeString(address.Custom),
			html.EscapeString(address.Notes),
			html.EscapeString(address.Address),
		)
		if _, err := file.WriteString(line); err != nil {
			return fmt.Errorf("write html: %w", err)
		}
		reportProgress(progress, index+1, intRef(total))
	}

	return nil
}

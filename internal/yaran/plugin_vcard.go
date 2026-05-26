package yaran

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	basicDatePattern        = regexp.MustCompile(`^\d{8}$`)
	extendedDatePattern     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	basicDateTimePattern    = regexp.MustCompile(`^(?P<date>\d{8})T\d{2}(\d{2}(\d{2})?)?([Z]|[+-]\d{2}(\d{2})?)?$`)
	extendedDateTimePattern = regexp.MustCompile(`^(?P<date>\d{4}-\d{2}-\d{2})T\d{2}(:\d{2}(:\d{2})?)?([Z]|[+-]\d{2}(:?\d{2})?)?$`)
	partialDatePattern      = regexp.MustCompile(`^(\d{4}|\d{4}-\d{2}|--\d{2}\d{2}|--\d{2}-\d{2}|---\d{2}|T.+)$`)
	customCategoryPattern   = regexp.MustCompile(`[;\n]+`)
)

type VCardPlugin struct{}

type rankedValue struct {
	rank  []int
	value string
}

func (VCardPlugin) Format() string {
	return "vcard"
}

func (VCardPlugin) Import(path string, sink AddressInserter, progress ProgressFunc) error {
	body, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read vcard: %w", err)
	}

	cards := iterCards(string(body))
	total := len(cards)
	reportProgress(progress, 0, intRef(total))

	for index, card := range cards {
		address, err := cardToAddress(card)
		if err == nil && address.Name != "" && address.Email != "" {
			_, _ = sink.Insert(address)
		}
		reportProgress(progress, index+1, intRef(total))
	}

	return nil
}

func (VCardPlugin) Export(path string, addresses []Address, progress ProgressFunc) error {
	total := len(addresses)
	reportProgress(progress, 0, intRef(total))

	var builder strings.Builder
	for index, address := range addresses {
		lines := []string{
			"BEGIN:VCARD",
			"VERSION:4.0",
			"KIND:individual",
			"FN:" + escapeText(address.Name),
			"N:" + fullNameToStructuredName(address.Name),
			"EMAIL:" + escapeText(address.Email),
		}

		if address.Birthday != "" {
			lines = append(lines, "BDAY:"+strings.ReplaceAll(address.Birthday, "-", ""))
		}
		if address.Address != "" {
			lines = append(lines, "ADR;TYPE=home:;;"+escapeComponent(address.Address)+";;;;")
		}
		if address.Phone != "" {
			lines = append(lines, "TEL;VALUE=text;TYPE=home:"+escapeText(address.Phone))
		}
		if address.Mobile != "" {
			lines = append(lines, "TEL;VALUE=text;TYPE=cell:"+escapeText(address.Mobile))
		}
		if address.Custom != "" {
			categories := splitCustomCategories(address.Custom)
			if len(categories) > 0 {
				escaped := make([]string, 0, len(categories))
				for _, category := range categories {
					escaped = append(escaped, escapeText(category))
				}
				lines = append(lines, "CATEGORIES:"+strings.Join(escaped, ","))
			}
		}
		if address.Notes != "" {
			lines = append(lines, "NOTE:"+escapeText(address.Notes))
		}

		lines = append(lines, "END:VCARD")
		builder.WriteString(renderLines(lines))
		reportProgress(progress, index+1, intRef(total))
	}

	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		return fmt.Errorf("write vcard: %w", err)
	}

	return nil
}

func splitOutsideQuotes(value string, separator rune) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false

	for _, char := range value {
		switch {
		case char == '"':
			inQuotes = !inQuotes
			current.WriteRune(char)
		case char == separator && !inQuotes:
			parts = append(parts, current.String())
			current.Reset()
		default:
			current.WriteRune(char)
		}
	}

	parts = append(parts, current.String())
	return parts
}

func splitEscaped(value string, separator rune) []string {
	var parts []string
	var current strings.Builder

	for index := 0; index < len(value); index++ {
		char := value[index]
		if char == '\\' && index+1 < len(value) {
			current.WriteByte(char)
			current.WriteByte(value[index+1])
			index++
			continue
		}
		if rune(char) == separator {
			parts = append(parts, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(char)
	}

	parts = append(parts, current.String())
	return parts
}

func unfoldLines(text string) []string {
	rawLines := strings.Split(text, "\n")
	unfolded := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSuffix(line, "\r")
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if len(unfolded) > 0 {
				unfolded[len(unfolded)-1] += line[1:]
			}
			continue
		}
		unfolded = append(unfolded, line)
	}
	return unfolded
}

func splitContentLine(line string) (string, map[string][]string, string) {
	var leftSide strings.Builder
	inQuotes := false
	valueStart := len(line)

	for index, char := range line {
		if char == '"' {
			inQuotes = !inQuotes
		}
		if char == ':' && !inQuotes {
			valueStart = index
			break
		}
		leftSide.WriteRune(char)
	}

	value := ""
	if valueStart < len(line) {
		value = line[valueStart+1:]
	}

	parts := splitOutsideQuotes(leftSide.String(), ';')
	propertyName := strings.ToUpper(parts[0])
	if strings.Contains(propertyName, ".") {
		segments := strings.Split(propertyName, ".")
		propertyName = segments[len(segments)-1]
	}

	parameters := map[string][]string{}
	for _, part := range parts[1:] {
		if part == "" {
			continue
		}

		if strings.Contains(part, "=") {
			key, rawValues, _ := strings.Cut(part, "=")
			key = strings.ToUpper(key)
			rawValues = strings.TrimSpace(rawValues)
			if strings.HasPrefix(rawValues, `"`) && strings.HasSuffix(rawValues, `"`) && len(rawValues) >= 2 {
				rawValues = rawValues[1 : len(rawValues)-1]
			}

			for _, item := range splitOutsideQuotes(rawValues, ',') {
				item = strings.TrimSpace(item)
				if item != "" {
					parameters[key] = append(parameters[key], item)
				}
			}
			continue
		}

		parameters["TYPE"] = append(parameters["TYPE"], strings.TrimSpace(part))
	}

	return propertyName, parameters, value
}

func unescapeText(value string) string {
	var builder strings.Builder

	for index := 0; index < len(value); index++ {
		if value[index] == '\\' && index+1 < len(value) {
			next := value[index+1]
			if next == 'n' || next == 'N' {
				builder.WriteByte('\n')
			} else {
				builder.WriteByte(next)
			}
			index++
			continue
		}
		builder.WriteByte(value[index])
	}

	return builder.String()
}

func escapeText(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, "\n", `\n`, ",", `\,`)
	return replacer.Replace(value)
}

func escapeComponent(value string) string {
	return strings.ReplaceAll(escapeText(value), ";", `\;`)
}

func foldContentLine(line string) string {
	const limit = 75

	parts := make([]string, 0, 2)
	current := ""
	prefix := ""

	for _, char := range line {
		candidate := current + string(char)
		if utf8.RuneCountInString(prefix+candidate) == len([]rune(prefix+candidate)) && len([]byte(prefix+candidate)) <= limit {
			current = candidate
			continue
		}

		if current != "" {
			parts = append(parts, prefix+current)
			prefix = " "
			current = string(char)
			continue
		}

		parts = append(parts, prefix)
		prefix = " "
		current = string(char)
	}

	parts = append(parts, prefix+current)
	return strings.Join(parts, "\r\n")
}

func renderLines(lines []string) string {
	folded := make([]string, 0, len(lines))
	for _, line := range lines {
		folded = append(folded, foldContentLine(line))
	}
	return strings.Join(folded, "\r\n") + "\r\n"
}

func parseBirthdayValue(rawValue string, parameters map[string][]string) (string, error) {
	for _, item := range parameters["VALUE"] {
		if strings.EqualFold(item, "text") {
			return "", nil
		}
	}

	text := strings.TrimSpace(unescapeText(rawValue))
	if text == "" {
		return "", nil
	}
	if basicDatePattern.MatchString(text) {
		return ValidateBirthday(text[:4] + "-" + text[4:6] + "-" + text[6:])
	}
	if extendedDatePattern.MatchString(text) {
		return ValidateBirthday(text)
	}
	if match := basicDateTimePattern.FindStringSubmatch(text); match != nil {
		datePart := match[1]
		return ValidateBirthday(datePart[:4] + "-" + datePart[4:6] + "-" + datePart[6:])
	}
	if match := extendedDateTimePattern.FindStringSubmatch(text); match != nil {
		return ValidateBirthday(match[1])
	}
	if partialDatePattern.MatchString(text) {
		return "", nil
	}

	return "", fmt.Errorf("invalid BDAY value")
}

func decodeComponentValues(component string) []string {
	parts := splitEscaped(component, ',')
	decoded := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(unescapeText(part))
		if value != "" {
			decoded = append(decoded, value)
		}
	}
	return decoded
}

func structuredNameToFullName(value string) string {
	parts := splitEscaped(value, ';')
	for len(parts) < 5 {
		parts = append(parts, "")
	}

	familyName := strings.Join(decodeComponentValues(parts[0]), " ")
	givenName := strings.Join(decodeComponentValues(parts[1]), " ")
	additionalName := strings.Join(decodeComponentValues(parts[2]), " ")
	honorificPrefix := strings.Join(decodeComponentValues(parts[3]), " ")
	honorificSuffix := strings.Join(decodeComponentValues(parts[4]), ", ")

	ordered := []string{honorificPrefix, givenName, additionalName, familyName, honorificSuffix}
	filtered := make([]string, 0, len(ordered))
	for _, part := range ordered {
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.TrimSpace(strings.Join(filtered, " "))
}

func fullNameToStructuredName(value string) string {
	parts := strings.Fields(value)
	if len(parts) <= 1 {
		return ";" + escapeComponent(value) + ";;;"
	}

	familyName := parts[len(parts)-1]
	givenName := strings.Join(parts[:len(parts)-1], " ")
	return strings.Join([]string{escapeComponent(familyName), escapeComponent(givenName), "", "", ""}, ";")
}

func parseAddressValue(rawValue string) string {
	components := splitEscaped(rawValue, ';')
	for len(components) < 7 {
		components = append(components, "")
	}

	rendered := make([]string, 0, 7)
	for _, component := range components[:7] {
		values := decodeComponentValues(component)
		if len(values) == 0 {
			continue
		}
		rendered = append(rendered, strings.Join(values, ", "))
	}

	return strings.Join(rendered, ", ")
}

func parseCategories(rawValue string) []string {
	parts := splitEscaped(rawValue, ',')
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(unescapeText(part))
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func splitCustomCategories(value string) []string {
	raw := customCategoryPattern.Split(value, -1)
	categories := make([]string, 0, len(raw))
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item != "" {
			categories = append(categories, item)
		}
	}
	return categories
}

func normalizeTelValue(rawValue string, parameters map[string][]string) string {
	value := strings.TrimSpace(rawValue)
	hasURI := false
	for _, item := range parameters["VALUE"] {
		if strings.EqualFold(item, "uri") {
			hasURI = true
			break
		}
	}

	if hasURI || strings.HasPrefix(strings.ToLower(value), "tel:") {
		if strings.HasPrefix(strings.ToLower(value), "tel:") {
			value = value[4:]
		}
		decoded, err := url.PathUnescape(value)
		if err == nil {
			return strings.TrimSpace(decoded)
		}
	}

	return strings.TrimSpace(unescapeText(value))
}

func prefRank(parameters map[string][]string, fallback int) int {
	for _, rawValue := range parameters["PREF"] {
		value, err := strconv.Atoi(rawValue)
		if err == nil && value > 0 {
			return value
		}
	}
	return fallback
}

func typeValues(parameters map[string][]string) map[string]struct{} {
	values := map[string]struct{}{}
	for _, item := range parameters["TYPE"] {
		values[strings.ToLower(item)] = struct{}{}
	}
	return values
}

func betterCandidate(current *rankedValue, rank []int, value string) *rankedValue {
	if current == nil || compareRank(rank, current.rank) < 0 {
		return &rankedValue{rank: rank, value: value}
	}
	return current
}

func compareRank(left []int, right []int) int {
	limit := len(left)
	if len(right) < limit {
		limit = len(right)
	}
	for index := 0; index < limit; index++ {
		if left[index] < right[index] {
			return -1
		}
		if left[index] > right[index] {
			return 1
		}
	}
	switch {
	case len(left) < len(right):
		return -1
	case len(left) > len(right):
		return 1
	default:
		return 0
	}
}

func iterCards(text string) [][]string {
	lines := unfoldLines(text)
	cards := make([][]string, 0)
	var current []string

	for _, line := range lines {
		normalized := strings.ToUpper(strings.TrimSpace(line))
		switch normalized {
		case "BEGIN:VCARD":
			current = []string{}
		case "END:VCARD":
			if current != nil {
				cards = append(cards, current)
				current = nil
			}
		default:
			if current != nil && strings.TrimSpace(line) != "" {
				current = append(current, line)
			}
		}
	}

	return cards
}

func cardToAddress(lines []string) (Address, error) {
	name := ""
	structuredName := ""
	birthday := ""
	noteParts := make([]string, 0)
	categories := make([]string, 0)
	legacyCustom := ""

	var emailChoice *rankedValue
	var addressChoice *rankedValue
	var phoneChoice *rankedValue
	var mobileChoice *rankedValue

	for index, line := range lines {
		propertyName, parameters, rawValue := splitContentLine(line)
		value := strings.TrimSpace(unescapeText(rawValue))

		switch propertyName {
		case "VERSION":
			continue
		case "FN":
			if value != "" && name == "" {
				name = value
			}
		case "N":
			if rawValue != "" && structuredName == "" {
				structuredName = rawValue
			}
		case "BDAY":
			parsed, err := parseBirthdayValue(rawValue, parameters)
			if err != nil {
				return Address{}, err
			}
			birthday = parsed
		case "EMAIL":
			if value != "" {
				emailChoice = betterCandidate(emailChoice, []int{prefRank(parameters, 100), index}, value)
			}
		case "ADR":
			if strings.TrimSpace(rawValue) != "" {
				typeRank := 1
				if _, ok := typeValues(parameters)["home"]; ok {
					typeRank = 0
				}
				addressChoice = betterCandidate(addressChoice, []int{prefRank(parameters, 100), typeRank, index}, parseAddressValue(rawValue))
			}
		case "TEL":
			if strings.TrimSpace(rawValue) == "" {
				continue
			}

			phone := normalizeTelValue(rawValue, parameters)
			if phone == "" {
				continue
			}

			typeSet := typeValues(parameters)
			typeRank := 1
			if _, ok := typeSet["home"]; ok {
				typeRank = 0
			}

			if _, ok := typeSet["cell"]; ok {
				mobileChoice = betterCandidate(mobileChoice, []int{prefRank(parameters, 100), typeRank, index}, phone)
				continue
			}
			if _, ok := typeSet["mobile"]; ok {
				mobileChoice = betterCandidate(mobileChoice, []int{prefRank(parameters, 100), typeRank, index}, phone)
				continue
			}
			phoneChoice = betterCandidate(phoneChoice, []int{prefRank(parameters, 100), typeRank, index}, phone)
		case "NOTE":
			if value != "" {
				noteParts = append(noteParts, value)
			}
		case "CATEGORIES":
			if strings.TrimSpace(rawValue) != "" {
				categories = append(categories, parseCategories(rawValue)...)
			}
		case "X-YARAN-CUSTOM", "X-DOOST-CUSTOM":
			if value != "" && legacyCustom == "" {
				legacyCustom = value
			}
		}
	}

	if name == "" && structuredName != "" {
		name = structuredNameToFullName(structuredName)
	}

	customValues := categories
	if len(customValues) == 0 {
		customValues = splitCustomCategories(legacyCustom)
	}
	customValues = dedupeStrings(customValues)

	address := Address{
		Name:     strings.TrimSpace(name),
		Email:    strings.TrimSpace(valueOrEmpty(emailChoice)),
		Birthday: strings.TrimSpace(birthday),
		Address:  strings.TrimSpace(valueOrEmpty(addressChoice)),
		Phone:    strings.TrimSpace(valueOrEmpty(phoneChoice)),
		Mobile:   strings.TrimSpace(valueOrEmpty(mobileChoice)),
		Custom:   strings.Join(customValues, ";"),
		Notes:    strings.TrimSpace(strings.Join(noteParts, "\n")),
	}

	return address, nil
}

func valueOrEmpty(candidate *rankedValue) string {
	if candidate == nil {
		return ""
	}
	return candidate.value
}

func dedupeStrings(values []string) []string {
	seen := map[string]struct{}{}
	deduped := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		deduped = append(deduped, value)
	}
	return deduped
}

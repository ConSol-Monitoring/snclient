package utils

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

const defaultMaxLineLength = 140

type ASCIITableHeader struct {
	Name      string // name in table header
	Field     string // attribute name in data row
	Alignment string // flag whether column is aligned to the right
	size      int    // calculated max size of column
}

// ASCIITable creates an ascii table from columns and data rows
func ASCIITable(header []ASCIITableHeader, rows any, escapePipes bool, maxLineLength int) (string, error) {
	dataRows := reflect.ValueOf(rows)
	if dataRows.Kind() != reflect.Slice {
		return "", fmt.Errorf("rows is not a slice")
	}

	moveLargeColToBack := false
	if header == nil {
		header = buildHeaderFromData(dataRows)
		moveLargeColToBack = true
	}

	err := calculateHeaderSize(header, dataRows, escapePipes, maxLineLength, moveLargeColToBack)
	if err != nil {
		return "", err
	}

	// output header
	out := ""
	strBuilder := strings.Builder{}
	for _, head := range header {
		fmt.Fprintf(&strBuilder, fmt.Sprintf("| %%-%ds ", head.size), head.Name)
	}
	out += strBuilder.String() + "|\n"

	// output separator
	strBuilder.Reset()
	for _, head := range header {
		padding := " "
		if head.Alignment == "centered" {
			padding = ":"
		}
		fmt.Fprintf(&strBuilder, "|%s%s%s", padding, strings.Repeat("-", head.size), padding)
	}
	out += strBuilder.String() + "|\n"
	strBuilder.Reset()

	// output data
	for i := range dataRows.Len() {
		rowVal := dataRows.Index(i)
		for _, head := range header {
			value, _ := asciiTableRowValue(escapePipes, rowVal, head)

			switch head.Alignment {
			case "right":
				fmt.Fprintf(&strBuilder, fmt.Sprintf("| %%%ds ", head.size), value)
			case "left", "":
				fmt.Fprintf(&strBuilder, fmt.Sprintf("| %%-%ds ", head.size), value)
			case "centered":
				padding := (head.size - len(value)) / 2
				fmt.Fprintf(&strBuilder, "| %*s%-*s ", padding, "", head.size-padding, value)
			default:
				err := fmt.Errorf("unsupported alignment '%s' in table", head.Alignment)

				return "", err
			}
		}
		strBuilder.WriteString("|\n")
	}

	out += strBuilder.String()
	strBuilder.Reset()

	return out, nil
}

func asciiTableRowValue(escape bool, rowVal reflect.Value, head ASCIITableHeader) (string, error) {
	value := ""
	var field reflect.Value
	switch rowVal.Kind() {
	case reflect.Struct:
		field = rowVal.FieldByName(head.Field)
	case reflect.Map:
		field = rowVal.MapIndex(reflect.ValueOf(head.Field))
	default:
	}

	if field.IsValid() {
		t := field.Type().String()
		switch t {
		case "string":
			value = field.String()
		default:
			return "", fmt.Errorf("unsupported struct attribute type for field %s: %s", head.Field, t)
		}
	}

	if escape {
		value = strings.ReplaceAll(value, "\n", `\n`)
		value = strings.ReplaceAll(value, "|", "\\|")
		value = strings.ReplaceAll(value, "$", "\\$")
		value = strings.ReplaceAll(value, "*", "\\*")
	}

	return value, nil
}

func calculateHeaderSize(header []ASCIITableHeader, dataRows reflect.Value, escapePipes bool, maxLineLength int, moveLargeColToBack bool) error {
	// set headers as minimum size
	for i, head := range header {
		header[i].size = len(head.Name)
	}

	// adjust column size from max row data
	for rowNum := range dataRows.Len() {
		rowVal := dataRows.Index(rowNum)
		switch rowVal.Kind() {
		case reflect.Struct:
		case reflect.Map:
		default:
			return fmt.Errorf("row %d is not a struct or map", rowNum)
		}
		for num, head := range header {
			value, err := asciiTableRowValue(escapePipes, rowVal, head)
			if err != nil {
				return err
			}
			length := len(value)
			if length > header[num].size {
				header[num].size = length
			}
		}
	}

	// calculate total line length
	total := 0
	for i := range header {
		total += header[i].size + 3 // add padding
	}

	if maxLineLength <= 0 {
		maxLineLength = defaultMaxLineLength
	}

	if total < maxLineLength {
		return nil
	}

	avgAvail := maxLineLength / len(header)
	sumTooWide := 0
	numTooWide := 0
	for i := range header {
		if header[i].size > avgAvail {
			sumTooWide += header[i].size
			numTooWide++
		}
	}

	if moveLargeColToBack {
		// sort headers by name and col size
		sort.Slice(header, func(i, j int) bool {
			if header[i].size == header[j].size {
				return header[i].Name < header[j].Name
			}

			return header[i].size < header[j].size
		})
	}

	avgLargeCol := (maxLineLength - (total - sumTooWide)) / numTooWide
	for i := range header {
		if header[i].size > avgAvail {
			header[i].size = avgLargeCol
		}
	}

	return nil
}

func (header ASCIITableHeader) Size() int {
	return header.size
}

func buildHeaderFromData(dataRows reflect.Value) []ASCIITableHeader {
	header := []ASCIITableHeader{}
	seen := map[string]bool{}

	for i := range dataRows.Len() {
		rowVal := dataRows.Index(i)
		if rowVal.Kind() != reflect.Map {
			continue
		}

		iter := rowVal.MapRange()
		for iter.Next() {
			key := iter.Key().String()
			if seen[key] {
				continue
			}
			seen[key] = true
			header = append(header, ASCIITableHeader{
				Name:  key,
				Field: key,
			})
		}
	}

	// sort headers by name
	sort.Slice(header, func(i, j int) bool {
		return header[i].Name < header[j].Name
	})

	return header
}

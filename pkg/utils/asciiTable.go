package utils

import (
	"fmt"
	"reflect"
	"strings"
)

type ASCIITableHeader struct {
	Name     string // name in table header
	Field    string // attribute name in data row
	Centered bool   // flag wether column is centered
	size     int    // calculated max size of column
}

// ASCIITable creates an ascii table from columns and data rows
func ASCIITable(header []ASCIITableHeader, rows interface{}, escapePipes bool) (string, error) {
	dataRows := reflect.ValueOf(rows)
	if dataRows.Kind() != reflect.Slice {
		return "", fmt.Errorf("rows is not a slice")
	}

	// set headers as minimum size
	for i, head := range header {
		header[i].size = len(head.Name)
	}

	// adjust column size from max row data
	for i := 0; i < dataRows.Len(); i++ {
		rowVal := dataRows.Index(i)
		if rowVal.Kind() != reflect.Struct {
			return "", fmt.Errorf("row %d is not a struct", i)
		}
		for num, head := range header {
			value, err := asciiTableRowValue(escapePipes, rowVal, head)
			if err != nil {
				return "", err
			}
			length := len(value)
			if length > header[num].size {
				header[num].size = length
			}
		}
	}

	// output header
	out := ""
	for _, head := range header {
		out += fmt.Sprintf(fmt.Sprintf("| %%-%ds ", head.size), head.Name)
	}
	out += "|\n"

	// output separator
	for _, head := range header {
		centered := " "
		if head.Centered {
			centered = ":"
		}
		out += fmt.Sprintf("|%s%s%s", centered, strings.Repeat("-", head.size), centered)
	}
	out += "|\n"

	// output data
	for i := 0; i < dataRows.Len(); i++ {
		rowVal := dataRows.Index(i)
		for _, head := range header {
			value, _ := asciiTableRowValue(escapePipes, rowVal, head)
			out += fmt.Sprintf(fmt.Sprintf("| %%-%ds ", head.size), value)
		}
		out += "|\n"
	}

	return out, nil
}

func asciiTableRowValue(escape bool, rowVal reflect.Value, head ASCIITableHeader) (string, error) {
	value := ""
	field := rowVal.FieldByName(head.Field)
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
		value = strings.ReplaceAll(value, "|", "\\|")
	}

	return value, nil
}

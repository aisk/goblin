package extension

import (
	"encoding/csv"
	"strings"
	"unicode/utf8"

	"github.com/aisk/goblin/object"
)

func ExecuteCSV() (object.Object, error) {
	return &object.Module{Name: "csv", Members: map[string]object.Object{
		"read_all":  &object.Function{Name: "read_all", Fn: csvReadAll},
		"write_all": &object.Function{Name: "write_all", Fn: csvWriteAll},
	}}, nil
}

func csvRune(name, parameter string, value object.String) (rune, error) {
	text := string(value)
	r, size := utf8.DecodeRuneInString(text)
	if text == "" || r == utf8.RuneError && size == 0 || size != len(text) {
		return 0, object.NewValueError("%s() argument '%s' must contain exactly one character", name, parameter)
	}
	return r, nil
}

func csvReadAll(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("read_all", args)
	text := p.Str("text")
	commaValue := p.StrOr("comma", ",")
	commentValue := p.StrOr("comment", "")
	fieldsPerRecord := p.IntOr("fields_per_record", 0)
	lazyQuotes := p.BoolOr("lazy_quotes", object.False)
	trimLeadingSpace := p.BoolOr("trim_leading_space", object.False)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	comma, err := csvRune("read_all", "comma", commaValue)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(strings.NewReader(string(text)))
	reader.Comma = comma
	reader.FieldsPerRecord = int(fieldsPerRecord)
	reader.LazyQuotes = bool(lazyQuotes)
	reader.TrimLeadingSpace = bool(trimLeadingSpace)
	if commentValue != "" {
		comment, err := csvRune("read_all", "comment", commentValue)
		if err != nil {
			return nil, err
		}
		reader.Comment = comment
	}
	records, err := reader.ReadAll()
	if err != nil {
		return nil, object.WrapError(object.ParseError, "read_all() failed", err)
	}
	return csvRecordsObject(records), nil
}

func csvWriteAll(args object.CallArgs) (object.Object, error) {
	p := object.NewArgParser("write_all", args)
	recordsObj := p.Any("records")
	commaValue := p.StrOr("comma", ",")
	useCRLF := p.BoolOr("use_crlf", object.False)
	if err := p.Finish(); err != nil {
		return nil, err
	}
	records, err := csvObjectRecords(recordsObj)
	if err != nil {
		return nil, err
	}
	comma, err := csvRune("write_all", "comma", commaValue)
	if err != nil {
		return nil, err
	}
	var output strings.Builder
	writer := csv.NewWriter(&output)
	writer.Comma = comma
	writer.UseCRLF = bool(useCRLF)
	writer.WriteAll(records)
	if err := writer.Error(); err != nil {
		return nil, object.WrapNativeError(object.IOError, "write_all() failed", err)
	}
	return object.String(output.String()), nil
}

func csvRecordsObject(records [][]string) *object.List {
	rows := make([]object.Object, len(records))
	for i, record := range records {
		fields := make([]object.Object, len(record))
		for j, field := range record {
			fields[j] = object.String(field)
		}
		rows[i] = &object.List{Elements: fields}
	}
	return &object.List{Elements: rows}
}

func csvObjectRecords(value object.Object) ([][]string, error) {
	rows, ok := value.(*object.List)
	if !ok {
		return nil, object.NewTypeError("write_all() argument 'records' must be a list, got %T", value)
	}
	records := make([][]string, len(rows.Elements))
	for i, rowObj := range rows.Elements {
		row, ok := rowObj.(*object.List)
		if !ok {
			return nil, object.NewTypeError("write_all() record %d must be a list, got %T", i, rowObj)
		}
		records[i] = make([]string, len(row.Elements))
		for j, fieldObj := range row.Elements {
			field, ok := fieldObj.(object.String)
			if !ok {
				return nil, object.NewTypeError("write_all() field %d in record %d must be a string, got %T", j, i, fieldObj)
			}
			records[i][j] = string(field)
		}
	}
	return records, nil
}

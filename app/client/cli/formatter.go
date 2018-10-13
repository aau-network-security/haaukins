package cli

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"
	tt "text/template"
)

const sep = "\t"

type formatElement interface{}
type header []string
type fields []string

type formatter struct {
	header header
	fields fields
}

type Formatter interface {
	Write(data []formatElement) (string, error)
}

func (h *header) string() string {
	return strings.Join(*h, sep)
}

func (f *fields) string(template string, data interface{}) (string, error) {
	t, err := tt.New("").Parse(template)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func (f *fields) template() string {
	var els []string
	for _, field := range *f {
		els = append(els, fmt.Sprintf("{{.%s}}", field))
	}
	return strings.Join(els, sep)
}

func (f *formatter) AsTable(data []formatElement) (string, error) {
	var res bytes.Buffer

	w := tabwriter.NewWriter(&res, 2, 0, 3, ' ', 0)
	header := f.header.string()
	if _, err := fmt.Fprintln(w, header); err != nil {
		return "", nil
	}

	template := f.fields.template()
	for _, d := range data {
		row, err := f.fields.string(template, d)
		if err != nil {
			return "", nil
		}
		if _, err := fmt.Fprintln(w, row); err != nil {
			return "", nil
		}
	}
	w.Flush()
	return res.String(), nil
}

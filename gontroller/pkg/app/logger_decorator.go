package app

import (
	"fmt"
	"strings"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

const (
	TreeBranch     = "├─"
	TreeLastBranch = "└─"
	TreePipe       = "│ "
)

type GontrollerDecorator struct{}

func (c *GontrollerDecorator) Decorate(buf *buffer.Buffer, fields []zapcore.Field) *buffer.Buffer {
	var validFields []zapcore.Field
	for _, field := range fields {
		if !isEmptyField(field) {
			validFields = append(validFields, field)
		}
	}

	if len(validFields) == 0 {
		return finalizeBuffer(buf)
	}

	if hasField(validFields, "sql") {
		return finalizeBuffer(c.decorateSQL(buf, validFields))
	}
	if hasField(validFields, "error") {
		return finalizeBuffer(c.decorateError(buf, validFields))
	}
	return finalizeBuffer(c.decorateDefault(buf, validFields))
}

func finalizeBuffer(buf *buffer.Buffer) *buffer.Buffer {
	buf.AppendString("\n\n")
	return buf
}

func (c *GontrollerDecorator) decorateSQL(buf *buffer.Buffer, fields []zapcore.Field) *buffer.Buffer {
	var elapsed, rows, sql string

	for _, field := range fields {
		switch field.Key {
		case "elapsed":
			if field.Interface != nil {
				elapsed = fmt.Sprintf("%v", field.Interface)
			}
		case "rows":
			if field.Integer != 0 {
				rows = fmt.Sprintf("%d", field.Integer)
			}
		case "sql":
			if field.String != "" {
				sql = field.String
			}
		}
	}

	if rows != "" && elapsed != "" {
		buf.AppendString(fmt.Sprintf("%s %s rows in %s", TreePipe, rows, elapsed))
	}
	if sql != "" {
		buf.AppendString(fmt.Sprintf("%s %s", TreeLastBranch, formatSQL(sql)))
	}
	return buf
}

func (c *GontrollerDecorator) decorateError(buf *buffer.Buffer, fields []zapcore.Field) *buffer.Buffer {
	for _, field := range fields {
		if field.Key == "error" && field.Interface != nil {
			buf.AppendString(fmt.Sprintf("╳ %v", field.Interface))
			return buf
		}
	}
	return buf
}

func (c *GontrollerDecorator) decorateDefault(buf *buffer.Buffer, fields []zapcore.Field) *buffer.Buffer {
	lastIdx := len(fields) - 1

	for i, field := range fields {
		value := formatFieldValue(field)
		if value != "" {
			prefix := TreeBranch
			if i == lastIdx {
				prefix = TreeLastBranch
			}
			buf.AppendString(fmt.Sprintf("%s %s: %s", prefix, field.Key, value))
			if i < lastIdx {
				buf.AppendString("\n")
			}
		}
	}
	return buf
}

func formatFieldValue(field zapcore.Field) string {
	switch {
	case field.Type == zapcore.StringType && field.String != "":
		return field.String
	case field.Type == zapcore.Int64Type && field.Integer != 0:
		return fmt.Sprintf("%d", field.Integer)
	case field.Interface != nil:
		return fmt.Sprintf("%v", field.Interface)
	}
	return ""
}

func isEmptyField(field zapcore.Field) bool {
	switch {
	case field.Type == zapcore.StringType:
		return field.String == ""
	case field.Type == zapcore.Int64Type:
		return field.Integer == 0
	case field.Interface == nil:
		return true
	default:
		return false
	}
}

func hasField(fields []zapcore.Field, key string) bool {
	for _, field := range fields {
		if field.Key == key {
			return true
		}
	}
	return false
}

func formatSQL(sql string) string {
	// Видаляємо зайві пробіли
	sql = strings.TrimSpace(sql)

	// Замінюємо множинні пробіли на один
	sql = strings.Join(strings.Fields(sql), " ")

	return sql
}

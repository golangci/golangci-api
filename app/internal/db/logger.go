package db

import (
	"database/sql/driver"
	"fmt"
	"reflect"
	"regexp"
	"time"
	"unicode"

	"github.com/golangci/golib/server/context"
)

var (
	sqlRegexp                = regexp.MustCompile(`\?`)
	numericPlaceHolderRegexp = regexp.MustCompile(`\$\d+`)
)

func isPrintable(s string) bool {
	for _, r := range s {
		if !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

var logFormatter = func(values ...interface{}) []interface{} {
	if len(values) == 0 {
		return []interface{}{}
	}

	var (
		sql             string
		formattedValues []string
		level           = values[0]
		currentTime     = "\033[33m[" + time.Now().Format("2006-01-02 15:04:05.000") + "]\033[0m"
		source          = ""
	)

	messages := []interface{}{source, currentTime}

	if level != "sql" {
		messages = append(messages, "\033[31;1m")
		messages = append(messages, values[2:]...)
		messages = append(messages, "\033[0m")
		return messages
	}

	// duration
	messages = append(messages, fmt.Sprintf(" \033[36;1m[%.2fms]\033[0m ", float64(values[2].(time.Duration).Nanoseconds()/1e4)/100.0))
	// sql

	for _, value := range values[4].([]interface{}) {
		indirectValue := reflect.Indirect(reflect.ValueOf(value))
		if indirectValue.IsValid() {
			value = indirectValue.Interface()
			if t, ok := value.(time.Time); ok {
				formattedValues = append(formattedValues, fmt.Sprintf("'%v'", t.Format("2006-01-02 15:04:05")))
			} else if b, ok := value.([]byte); ok {
				if str := string(b); isPrintable(str) {
					formattedValues = append(formattedValues, fmt.Sprintf("'%s'", str))
				} else {
					formattedValues = append(formattedValues, "'<binary>'")
				}
			} else if r, ok := value.(fmt.Stringer); ok {
				formattedValues = append(formattedValues, fmt.Sprintf("'%s'", r.String()))
			} else if r, ok := value.(driver.Valuer); ok {
				if value, err := r.Value(); err == nil && value != nil {
					formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
				} else {
					formattedValues = append(formattedValues, "NULL")
				}
			} else {
				formattedValues = append(formattedValues, fmt.Sprintf("'%v'", value))
			}
		} else {
			formattedValues = append(formattedValues, "NULL")
		}
	}

	// differentiate between $n placeholders or else treat like ?
	if numericPlaceHolderRegexp.MatchString(values[3].(string)) {
		sql = values[3].(string)
		for index, value := range formattedValues {
			placeholder := fmt.Sprintf(`\$%d([^\d]|$)`, index+1)
			sql = regexp.MustCompile(placeholder).ReplaceAllString(sql, value+"$1")
		}
	} else {
		formattedValuesLength := len(formattedValues)
		for index, value := range sqlRegexp.Split(values[3].(string), -1) {
			sql += value
			if index < formattedValuesLength {
				sql += formattedValues[index]
			}
		}
	}

	messages = append(messages, sql)

	return messages
}

type logger struct {
	ctx *context.C
}

func (lg logger) Print(values ...interface{}) {
	lg.ctx.L.Info(logFormatter(values...)...)
}

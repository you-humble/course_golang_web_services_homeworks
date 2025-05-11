package main

import (
	"fmt"
	"strings"
)

const (
	paramPrefix string = "p"
)

type RequiredValidator struct{}

func (v RequiredValidator) Stamp(f *Field) string {
	if f.Type == "string" {
		format := `if len(%s.%s) == 0 {
		return errors.New("%s must me not empty")
	}`
		return fmt.Sprintf(format, paramPrefix, f.Name, f.ParamName())
	} else if f.Type == "int" {
		format := `if %s.%s == 0 {
			return errors.New("%s must me not empty")
		}`
		return fmt.Sprintf(format, paramPrefix, f.Name, f.ParamName())
	}
	panic("unsuppoted type")
}

type EnumValidator struct {
	Values []string
}

func (v EnumValidator) Stamp(f *Field) string {
	values := []string{}

	if f.Type == "int" {
		values = append(values, v.Values...)
	} else if f.Type == "string" {
		for _, v := range v.Values {
			values = append(values, fmt.Sprintf("%q", v))
		}
	} else {
		panic("unknown type")
	}

	paramVal := strings.ToLower(f.Name)
	enumVar := paramVal + "Enum"
	isCorrectVar := "isCorrect" + f.Name

	enumSlice := fmt.Sprintf("[]string{%s}", strings.Join(values, ", "))
	acceptedValuesForError := fmt.Sprintf("[%s]", strings.Join(v.Values, ", "))
	format := `%s := %s
	%s := false
	for _, v := range %s {
		if %s.%s == v {
			%s = true
			break
		}
	}
	if !%s {
		return errors.New("%s must be one of %s")
	}
	`
	return fmt.Sprintf(format,
		enumVar, enumSlice, isCorrectVar, enumVar,
		paramPrefix, f.Name, isCorrectVar, isCorrectVar,
		paramVal, acceptedValuesForError,
	)

}

type DefaultValidator struct {
	DefaultValue interface{}
}

func (v DefaultValidator) Stamp(f *Field) string {
	if v.DefaultValue == nil {
		return ""
	}
	format := `if %s.%s == %s {
		%s.%s = %q
	}
	`
	var zeroVal interface{}
	if f.Type == "string" {
		zeroVal = `""`
	} else if f.Type == "int" {
		zeroVal = 0
	} else {
		panic("unknown type")
	}
	return fmt.Sprintf(format, paramPrefix, f.Name, zeroVal,
		paramPrefix, f.Name, v.DefaultValue)

}

type MinValidator struct {
	Value int
}

func (v MinValidator) Stamp(f *Field) string {
	if f.Type == "string" {
		format := `if len(%s.%s) < %d {
		return errors.New("%s len must be >= %d")
	}`
		return fmt.Sprintf(format, paramPrefix, f.Name, v.Value, f.ParamName(), v.Value)
	} else if f.Type == "int" {
		format := `if %s.%s < %d {
		return errors.New("%s must be >= %d")
	}`
		return fmt.Sprintf(format, paramPrefix, f.Name, v.Value, f.ParamName(), v.Value)
	}
	panic("unsuppoted type")
}

type MaxValidator struct {
	Value int
}

func (v MaxValidator) Stamp(f *Field) string {
	if f.Type == "int" {
		format := `if %s.%s > %d {
		return errors.New("%s must be <= %d")
	}`
		return fmt.Sprintf(format, paramPrefix, f.Name, v.Value, f.ParamName(), v.Value)
	}
	panic("unsuppoted type")
}

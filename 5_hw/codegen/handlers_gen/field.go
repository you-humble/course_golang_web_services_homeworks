package main

import (
	"log"
	"strconv"
	"strings"
)

type Stamper interface {
	Stamp(*Field) string
}

type Field struct {
	Name       string
	Type       string
	Validators []Stamper

	paramName string
}

func NewField(fName, fType, fTag string) *Field {
	validators := strings.Split(fTag, ",")

	field := Field{
		Name:       fName,
		Type:       fType,
		Validators: make([]Stamper, 0, 5),
	}
	for _, validator := range validators {
		kv := strings.Split(validator, "=")
		switch kv[0] {
		case "default":
			var val any
			var err error
			switch fType {
			case "string":
				val = kv[1]
			case "int":
				val, err = strconv.Atoi(kv[1])
				if err != nil {
					log.Fatalf("NewField: %+v", err)
				}
			default:
				log.Fatalf("NewField unknown type - %s", fType)
			}
			field.Validators = append(field.Validators[:1], field.Validators...)
			field.Validators[0] = DefaultValidator{DefaultValue: val}
		case "required":
			field.Validators = append(field.Validators, RequiredValidator{})
		case "paramname":
			field.SetParamName(kv[1])
		case "enum":
			vals := strings.Split(kv[1], "|")
			field.Validators = append(field.Validators, EnumValidator{Values: vals})
		case "min":
			val, _ := strconv.Atoi(kv[1])
			field.Validators = append(field.Validators, MinValidator{Value: val})
		case "max":
			val, _ := strconv.Atoi(kv[1])
			field.Validators = append(field.Validators, MaxValidator{Value: val})
		}
	}

	return &field
}

func (f *Field) ParamName() string {
	if f.paramName == "" {
		return strings.ToLower(f.Name)
	}
	return f.paramName
}

func (f *Field) SetParamName(name string) {
	f.paramName = name
}

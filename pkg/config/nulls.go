package config

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// nullEntries returns the YAML paths of the null entries in the lists of
// cfg, in field order. YAML decodes a null list entry to a nil pointer,
// and so may a Go program that builds a Config; the defaults and the
// checks dereference every entry, so these are found first.
func nullEntries(cfg *Config) []string {
	var paths []string

	walkNulls(reflect.ValueOf(cfg).Elem(), "", &paths)

	return paths
}

// walkNulls appends the paths of nil pointers in the slices under val.
// It follows the exported fields that have a YAML key, so the paths match
// the YAML. No config type uses ",inline"; such a field would be skipped.
// A nil map value (a language without settings) is allowed.
func walkNulls(val reflect.Value, path string, out *[]string) {
	switch val.Kind() { //nolint:exhaustive // only containers hold entries
	case reflect.Pointer, reflect.Interface:
		if !val.IsNil() {
			walkNulls(val.Elem(), path, out)
		}
	case reflect.Struct:
		walkNullFields(val, path, out)
	case reflect.Slice:
		for i := range val.Len() {
			elem := val.Index(i)
			elemPath := fmt.Sprintf("%s[%d]", path, i)

			if elem.Kind() == reflect.Pointer && elem.IsNil() {
				*out = append(*out, elemPath)

				continue
			}

			walkNulls(elem, elemPath, out)
		}
	case reflect.Map:
		keys := val.MapKeys()
		slices.SortFunc(keys, func(a, b reflect.Value) int { return strings.Compare(fmt.Sprint(a), fmt.Sprint(b)) })

		for _, key := range keys {
			walkNulls(val.MapIndex(key), fmt.Sprintf("%s.%v", path, key), out)
		}
	}
}

// walkNullFields walks the exported fields of the struct val that have a
// YAML key.
func walkNullFields(val reflect.Value, path string, out *[]string) {
	typ := val.Type()

	for i := range typ.NumField() {
		field := typ.Field(i)

		key, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if !field.IsExported() || key == "" || key == "-" {
			continue
		}

		if path != "" {
			key = path + "." + key
		}

		walkNulls(val.Field(i), key, out)
	}
}

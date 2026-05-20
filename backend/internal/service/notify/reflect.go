package notify

import "reflect"

// Small reflection helpers used by the service to pull fields out of
// event payloads without importing the publisher packages (the
// notify service can't import job/billing/node directly without a
// circular dep).
//
// These are deliberately permissive — unknown field names return
// zero values rather than panicking, so a malformed event still
// produces a (less detailed) message instead of crashing the
// subscriber goroutine.

func reflectInt64(data any, field string) int64 {
	if data == nil {
		return 0
	}
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return 0
	}
	f := v.FieldByName(field)
	if !f.IsValid() {
		return 0
	}
	switch f.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return f.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(f.Uint())
	default:
		return 0
	}
}

func reflectString(data any, field string) string {
	if data == nil {
		return ""
	}
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return ""
	}
	f := v.FieldByName(field)
	if !f.IsValid() || f.Kind() != reflect.String {
		return ""
	}
	return f.String()
}

func reflectNodeID(data any) int64 {
	for _, name := range []string{"NodeID", "ID", "Node"} {
		if id := reflectInt64(data, name); id != 0 {
			return id
		}
	}
	return 0
}

func reflectNodeName(data any) string {
	for _, name := range []string{"NodeName", "Name"} {
		if s := reflectString(data, name); s != "" {
			return s
		}
	}
	return ""
}

package rest

import "reflect"

func isNil(v interface{}) bool {
	return v == nil || (reflect.TypeOf(v).Kind() == reflect.Ptr && reflect.ValueOf(v).IsNil())
}

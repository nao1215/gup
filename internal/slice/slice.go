// Package slice defines functions to manipulate the slice
package slice

import "reflect"

// Contains returns whether the list has the elements specified in the argument.
func Contains(list interface{}, elem interface{}) bool {
	rvList := reflect.ValueOf(list)

	if rvList.Kind() == reflect.Slice {
		for i := 0; i < rvList.Len(); i++ {
			item := rvList.Index(i).Interface()
			if !reflect.TypeOf(elem).ConvertibleTo(reflect.TypeOf(item)) {
				continue
			}
			target := reflect.ValueOf(elem).Convert(reflect.TypeOf(item)).Interface()
			if ok := reflect.DeepEqual(item, target); ok {
				return true
			}
		}
	}
	return false
}

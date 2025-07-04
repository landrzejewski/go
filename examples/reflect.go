package examples

import (
	"fmt"
	"reflect"
)

type Person struct {
	Name string `mymeta:"required" training:"required"`
	Age  int    `mymeta:"range=0..150"`
}

func Reflect() {
	t := reflect.TypeOf(Person{})

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if tagVal := f.Tag.Get("mymeta"); tagVal != "" {
			fmt.Printf("%s -> %q\n", f.Name, tagVal)
		}
	}
}

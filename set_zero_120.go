//go:build go1.20
// +build go1.20

package maxminddb

import "github.com/3JoB/go-reflect"

func reflectSetZero(v reflect.Value) {
	v.SetZero()
}

// Package deepcopy provides reflection-based value copies for internal SDK state.
package deepcopy

import "reflect"

type visitKey struct {
	typ    reflect.Type
	ptr    uintptr
	length int
}

// Value returns a best-effort deep copy of in.
func Value[T any](in T) T {
	v := reflect.ValueOf(in)
	if !v.IsValid() {
		var zero T
		return zero
	}

	cloned := cloneValue(v, make(map[visitKey]reflect.Value))
	out, ok := cloned.Interface().(T)
	if ok {
		return out
	}
	return in
}

func cloneValue(v reflect.Value, seen map[visitKey]reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}

	switch v.Kind() {
	case reflect.Pointer:
		return clonePointer(v, seen)
	case reflect.Interface:
		return cloneInterface(v, seen)
	case reflect.Struct:
		return cloneStruct(v, seen)
	case reflect.Slice:
		return cloneSlice(v, seen)
	case reflect.Array:
		return cloneArray(v, seen)
	case reflect.Map:
		return cloneMap(v, seen)
	default:
		return v
	}
}

func clonePointer(v reflect.Value, seen map[visitKey]reflect.Value) reflect.Value {
	if v.IsNil() {
		return reflect.Zero(v.Type())
	}
	key := visitKey{typ: v.Type(), ptr: v.Pointer()}
	if cached, ok := seen[key]; ok {
		return cached
	}
	cloned := reflect.New(v.Type().Elem())
	seen[key] = cloned
	cloned.Elem().Set(cloneValue(v.Elem(), seen))
	return cloned
}

func cloneInterface(v reflect.Value, seen map[visitKey]reflect.Value) reflect.Value {
	if v.IsNil() {
		return reflect.Zero(v.Type())
	}
	cloned := cloneValue(v.Elem(), seen)
	out := reflect.New(v.Type()).Elem()
	out.Set(cloned)
	return out
}

func cloneStruct(v reflect.Value, seen map[visitKey]reflect.Value) reflect.Value {
	cloned := reflect.New(v.Type()).Elem()
	for i := range v.NumField() {
		dst := cloned.Field(i)
		if !dst.CanSet() {
			return v
		}
		dst.Set(cloneValue(v.Field(i), seen))
	}
	return cloned
}

func cloneSlice(v reflect.Value, seen map[visitKey]reflect.Value) reflect.Value {
	if v.IsNil() {
		return reflect.Zero(v.Type())
	}
	cloned, ok := rememberNewClone(v, seen, func() reflect.Value {
		return reflect.MakeSlice(v.Type(), v.Len(), v.Len())
	})
	if ok {
		return cloned
	}
	for i := range v.Len() {
		cloned.Index(i).Set(cloneValue(v.Index(i), seen))
	}
	return cloned
}

func cloneArray(v reflect.Value, seen map[visitKey]reflect.Value) reflect.Value {
	cloned := reflect.New(v.Type()).Elem()
	for i := range v.Len() {
		cloned.Index(i).Set(cloneValue(v.Index(i), seen))
	}
	return cloned
}

func cloneMap(v reflect.Value, seen map[visitKey]reflect.Value) reflect.Value {
	if v.IsNil() {
		return reflect.Zero(v.Type())
	}
	cloned, ok := rememberNewClone(v, seen, func() reflect.Value {
		return reflect.MakeMapWithSize(v.Type(), v.Len())
	})
	if ok {
		return cloned
	}
	iter := v.MapRange()
	for iter.Next() {
		cloned.SetMapIndex(
			cloneValue(iter.Key(), seen),
			cloneValue(iter.Value(), seen),
		)
	}
	return cloned
}

func rememberNewClone(v reflect.Value, seen map[visitKey]reflect.Value, build func() reflect.Value) (reflect.Value, bool) {
	key, cached, ok := lookupSeen(v, seen)
	if ok {
		return cached, true
	}
	cloned := build()
	rememberSeen(key, cloned, seen)
	return cloned, false
}

func lookupSeen(v reflect.Value, seen map[visitKey]reflect.Value) (visitKey, reflect.Value, bool) {
	key := visitKey{typ: v.Type(), ptr: v.Pointer()}
	if v.Kind() == reflect.Slice {
		// Views of the same backing array may have different visible elements.
		key.length = v.Len()
	}
	if key.ptr == 0 {
		return key, reflect.Value{}, false
	}
	cached, ok := seen[key]
	return key, cached, ok
}

func rememberSeen(key visitKey, cloned reflect.Value, seen map[visitKey]reflect.Value) {
	if key.ptr == 0 {
		return
	}
	seen[key] = cloned
}

package codex

import "reflect"

func cloneThreadState(thread Thread) Thread {
	return cloneArbitraryValue(thread)
}

func cloneThreadStatusWrapper(w ThreadStatusWrapper) ThreadStatusWrapper {
	return cloneArbitraryValue(w)
}

func cloneStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := *s
	return &v
}

type cloneVisitKey struct {
	typ reflect.Type
	ptr uintptr
}

func cloneArbitraryValue[T any](in T) T {
	v := reflect.ValueOf(in)
	if !v.IsValid() {
		var zero T
		return zero
	}

	cloned := cloneReflectValue(v, make(map[cloneVisitKey]reflect.Value))
	out, ok := cloned.Interface().(T)
	if ok {
		return out
	}
	return in
}

func cloneReflectValue(v reflect.Value, seen map[cloneVisitKey]reflect.Value) reflect.Value {
	if !v.IsValid() {
		return v
	}

	switch v.Kind() {
	case reflect.Pointer:
		return cloneReflectPointer(v, seen)
	case reflect.Interface:
		return cloneReflectInterface(v, seen)
	case reflect.Struct:
		return cloneReflectStruct(v, seen)
	case reflect.Slice:
		return cloneReflectSlice(v, seen)
	case reflect.Array:
		return cloneReflectArray(v, seen)
	case reflect.Map:
		return cloneReflectMap(v, seen)
	default:
		return v
	}
}

func cloneReflectPointer(v reflect.Value, seen map[cloneVisitKey]reflect.Value) reflect.Value {
	if v.IsNil() {
		return reflect.Zero(v.Type())
	}
	key := cloneVisitKey{typ: v.Type(), ptr: v.Pointer()}
	if cached, ok := seen[key]; ok {
		return cached
	}
	cloned := reflect.New(v.Type().Elem())
	seen[key] = cloned
	cloned.Elem().Set(cloneReflectValue(v.Elem(), seen))
	return cloned
}

func cloneReflectInterface(v reflect.Value, seen map[cloneVisitKey]reflect.Value) reflect.Value {
	if v.IsNil() {
		return reflect.Zero(v.Type())
	}
	cloned := cloneReflectValue(v.Elem(), seen)
	out := reflect.New(v.Type()).Elem()
	out.Set(cloned)
	return out
}

func cloneReflectStruct(v reflect.Value, seen map[cloneVisitKey]reflect.Value) reflect.Value {
	cloned := reflect.New(v.Type()).Elem()
	for i := range v.NumField() {
		dst := cloned.Field(i)
		if !dst.CanSet() {
			return v
		}
		dst.Set(cloneReflectValue(v.Field(i), seen))
	}
	return cloned
}

func cloneReflectSlice(v reflect.Value, seen map[cloneVisitKey]reflect.Value) reflect.Value {
	if v.IsNil() {
		return reflect.Zero(v.Type())
	}
	key, cached, ok := lookupSeenClone(v, seen)
	if ok {
		return cached
	}
	cloned := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
	rememberSeenClone(key, cloned, seen)
	for i := range v.Len() {
		cloned.Index(i).Set(cloneReflectValue(v.Index(i), seen))
	}
	return cloned
}

func cloneReflectArray(v reflect.Value, seen map[cloneVisitKey]reflect.Value) reflect.Value {
	cloned := reflect.New(v.Type()).Elem()
	for i := range v.Len() {
		cloned.Index(i).Set(cloneReflectValue(v.Index(i), seen))
	}
	return cloned
}

func cloneReflectMap(v reflect.Value, seen map[cloneVisitKey]reflect.Value) reflect.Value {
	if v.IsNil() {
		return reflect.Zero(v.Type())
	}
	key, cached, ok := lookupSeenClone(v, seen)
	if ok {
		return cached
	}
	cloned := reflect.MakeMapWithSize(v.Type(), v.Len())
	rememberSeenClone(key, cloned, seen)
	iter := v.MapRange()
	for iter.Next() {
		cloned.SetMapIndex(
			cloneReflectValue(iter.Key(), seen),
			cloneReflectValue(iter.Value(), seen),
		)
	}
	return cloned
}

func lookupSeenClone(v reflect.Value, seen map[cloneVisitKey]reflect.Value) (cloneVisitKey, reflect.Value, bool) {
	key := cloneVisitKey{typ: v.Type(), ptr: v.Pointer()}
	if key.ptr == 0 {
		return key, reflect.Value{}, false
	}
	cached, ok := seen[key]
	return key, cached, ok
}

func rememberSeenClone(key cloneVisitKey, cloned reflect.Value, seen map[cloneVisitKey]reflect.Value) {
	if key.ptr == 0 {
		return
	}
	seen[key] = cloned
}

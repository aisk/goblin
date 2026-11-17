package object

// CallMethod runs a built-in method without first materializing a bound
// *Function for it. `x.m(args)` used to allocate a closure on every call just
// to throw it away after the call returned; the transpiler and the interpreter
// now route that shape through the package-level CallMethod below, and only
// `x.m` taken as a value still goes through GetAttr.
//
// handled reports whether name is one of the receiver's methods. It is false
// for a property and for anything unknown, so the caller falls back to
// GetAttr and the resulting value or error is unchanged.
//
// Each switch names the same methods as the type's methodTable, and
// TestCallMethodMatchesGetAttr holds them to that.

func (l *List) CallMethod(name string, args CallArgs) (Object, bool, error) {
	switch name {
	case "push":
		v, err := l.Push(args)
		return v, true, err
	case "pop":
		v, err := l.Pop(args)
		return v, true, err
	case "join":
		v, err := l.Join(args)
		return v, true, err
	case "insert":
		v, err := l.Insert(args)
		return v, true, err
	case "contains":
		v, err := l.Contains(args)
		return v, true, err
	case "count":
		v, err := l.Count(args)
		return v, true, err
	case "index":
		v, err := l.IndexOf(args)
		return v, true, err
	case "remove":
		v, err := l.Remove(args)
		return v, true, err
	case "reverse":
		v, err := l.Reverse(args)
		return v, true, err
	case "clear":
		v, err := l.Clear(args)
		return v, true, err
	case "copy":
		v, err := l.Copy(args)
		return v, true, err
	case "sort":
		v, err := l.sortMethod(args)
		return v, true, err
	case "map":
		v, err := l.mapMethod(args)
		return v, true, err
	case "filter":
		v, err := l.filterMethod(args)
		return v, true, err
	case "reduce":
		v, err := l.reduceMethod(args)
		return v, true, err
	case "find":
		v, err := l.findMethod(args)
		return v, true, err
	case "any":
		v, err := l.anyMethod(args)
		return v, true, err
	case "all":
		v, err := l.allMethod(args)
		return v, true, err
	}
	return nil, false, nil
}

func (s String) CallMethod(name string, args CallArgs) (Object, bool, error) {
	switch name {
	case "upper":
		v, err := s.Upper(args)
		return v, true, err
	case "lower":
		v, err := s.Lower(args)
		return v, true, err
	case "trim":
		v, err := s.trim("trim", args, trimBoth)
		return v, true, err
	case "trim_start":
		v, err := s.trim("trim_start", args, trimStart)
		return v, true, err
	case "trim_end":
		v, err := s.trim("trim_end", args, trimEnd)
		return v, true, err
	case "trim_prefix":
		v, err := s.TrimPrefix(args)
		return v, true, err
	case "trim_suffix":
		v, err := s.TrimSuffix(args)
		return v, true, err
	case "has_prefix":
		v, err := s.HasPrefix(args)
		return v, true, err
	case "has_suffix":
		v, err := s.HasSuffix(args)
		return v, true, err
	case "contains":
		v, err := s.Contains(args)
		return v, true, err
	case "count":
		v, err := s.Count(args)
		return v, true, err
	case "index":
		v, err := s.IndexOf(args)
		return v, true, err
	case "replace":
		v, err := s.Replace(args)
		return v, true, err
	case "split":
		v, err := s.Split(args)
		return v, true, err
	case "cut":
		v, err := s.Cut(args)
		return v, true, err
	}
	return nil, false, nil
}

func (d *Dict) CallMethod(name string, args CallArgs) (Object, bool, error) {
	switch name {
	case "keys":
		v, err := d.Keys(args)
		return v, true, err
	case "values":
		v, err := d.Values(args)
		return v, true, err
	case "items":
		v, err := d.Items(args)
		return v, true, err
	case "contains":
		v, err := d.Contains(args)
		return v, true, err
	case "get":
		v, err := d.GetValue(args)
		return v, true, err
	case "get_or_set":
		v, err := d.GetOrSet(args)
		return v, true, err
	case "remove":
		v, err := d.Remove(args)
		return v, true, err
	case "pop":
		v, err := d.Pop(args)
		return v, true, err
	case "update":
		v, err := d.Update(args)
		return v, true, err
	case "clear":
		v, err := d.Clear(args)
		return v, true, err
	case "copy":
		v, err := d.Copy(args)
		return v, true, err
	}
	return nil, false, nil
}

func (b Bytes) CallMethod(name string, args CallArgs) (Object, bool, error) {
	switch name {
	case "decode":
		v, err := b.Decode(args)
		return v, true, err
	case "contains":
		v, err := b.Contains(args)
		return v, true, err
	case "count":
		v, err := b.Count(args)
		return v, true, err
	case "index":
		v, err := b.IndexOf(args)
		return v, true, err
	case "has_prefix":
		v, err := b.HasPrefix(args)
		return v, true, err
	case "has_suffix":
		v, err := b.HasSuffix(args)
		return v, true, err
	case "trim_prefix":
		v, err := b.TrimPrefix(args)
		return v, true, err
	case "trim_suffix":
		v, err := b.TrimSuffix(args)
		return v, true, err
	case "replace":
		v, err := b.Replace(args)
		return v, true, err
	case "split":
		v, err := b.Split(args)
		return v, true, err
	case "cut":
		v, err := b.Cut(args)
		return v, true, err
	}
	return nil, false, nil
}

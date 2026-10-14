package model

// Accepts reports whether a call with argCount positional arguments, the
// given keyword arguments and possibly a spread argument can bind to sig.
// With a spread the length is unknown, so only "too many fixed arguments"
// can be proven wrong.
func Accepts(sig Signature, argCount int, keywords []string, spread bool) bool {
	var positional []Param
	hasRest, hasKwRest := false, false
	for _, p := range sig.Params {
		switch {
		case p.Rest:
			hasRest = true
		case p.KwRest:
			hasKwRest = true
		case !p.KwOnly:
			positional = append(positional, p)
		}
	}
	if argCount > len(positional) && !hasRest {
		return false
	}
	if spread {
		return true
	}
	filled := map[string]bool{}
	for i := 0; i < argCount && i < len(positional); i++ {
		filled[positional[i].Name] = true
	}
	for _, kw := range keywords {
		named := false
		for _, p := range sig.Params {
			if p.Name == kw && !p.Rest && !p.KwRest {
				named = true
				break
			}
		}
		switch {
		case named && !filled[kw]:
			filled[kw] = true
		case !named && hasKwRest:
		default:
			return false
		}
	}
	for _, p := range sig.Params {
		if !p.Optional && !p.Rest && !p.KwRest && !filled[p.Name] {
			return false
		}
	}
	return true
}

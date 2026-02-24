package middleware

import "strings"

// SetModelInput assigns the raw request payload destined for the model.
func (st *State) SetModelInput(v any) {
	if st == nil {
		return
	}
	st.ModelInput = v
}

// SetModelOutput assigns the raw response payload provided by the model layer.
func (st *State) SetModelOutput(v any) {
	if st == nil {
		return
	}
	st.ModelOutput = v
}

// SetValue stores arbitrary metadata on the state, ensuring the backing map exists.
func (st *State) SetValue(key string, value any) {
	if st == nil {
		return
	}
	if strings.TrimSpace(key) == "" {
		return
	}
	if st.Values == nil {
		st.Values = map[string]any{}
	}
	st.Values[key] = value
}

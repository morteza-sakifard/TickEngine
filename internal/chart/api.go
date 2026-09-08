package chart

import (
	"encoding/json"
	"io"
)

// WriteJSON writes v as JSON. It is the same value RenderSVG draws:
// one View, two renderers. encoding/json walks the exported fields
// in declaration order, so two calls with the same View are
// byte-identical. There is no second DTO.
func WriteJSON(w io.Writer, v View) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// ReadJSON is the inverse of WriteJSON. It exists so tests can prove
// the wire form still is a View, not a parallel schema.
func ReadJSON(r io.Reader) (View, error) {
	var v View
	err := json.NewDecoder(r).Decode(&v)
	return v, err
}

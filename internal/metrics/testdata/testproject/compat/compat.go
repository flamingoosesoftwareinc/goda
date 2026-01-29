// Package compat contains types that structurally satisfy interfaces in other
// packages without importing them. This exercises structural coupling detection.
package compat

// ByteReader has a Read method matching base.Reader's signature,
// but does NOT import the base package.
type ByteReader struct {
	Data []byte
}

// Read satisfies the base.Reader interface structurally.
func (r *ByteReader) Read(p []byte) (int, error) {
	n := copy(p, r.Data)
	r.Data = r.Data[n:]
	return n, nil
}

// ByteWriter has a Write method matching base.Writer's signature,
// but does NOT import the base package.
type ByteWriter struct {
	Data []byte
}

// Write satisfies the base.Writer interface structurally.
func (w *ByteWriter) Write(p []byte) (int, error) {
	w.Data = append(w.Data, p...)
	return len(p), nil
}

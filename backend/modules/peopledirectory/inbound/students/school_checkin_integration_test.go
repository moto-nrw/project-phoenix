package students_test

import (
	"bytes"
	"io"
)

// bytesReader is a local convenience wrapper — the existing helpers pass
// nil for GET bodies, but our POST cases need a real reader.
func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

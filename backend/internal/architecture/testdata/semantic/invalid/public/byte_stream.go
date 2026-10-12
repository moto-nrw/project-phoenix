package public

import "io"

// A public capability may return a standard byte stream, not a generic row read.
func OpenAttachment() (io.ReadCloser, error) { return nil, nil }

type RowReader interface{ Read() (string, error) }

func OpenRows() RowReader { return nil }

package application

// The guardian half of the recording engine: the row-level reads the
// composition supplies next to the person and student engines.
type guardianCall struct {
	accountIDs []int64
	ids        []int64
}

// The guardian lifecycle commands carry no facade rule of their own; the
// recording engine only completes the engine contract for them.

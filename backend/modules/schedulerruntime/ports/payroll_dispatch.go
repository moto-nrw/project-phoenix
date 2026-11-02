package ports

import "context"

// PayrollDispatch sends the due month files of the school in ctx to its
// payroll office, settles mailed download links and purges expired files
// (#4805). It reports the dispatches it finished. The root binds Workforce's
// payroll dispatch.
type PayrollDispatch func(ctx context.Context) (int, error)

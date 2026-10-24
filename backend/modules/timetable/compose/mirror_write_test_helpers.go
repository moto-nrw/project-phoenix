//go:build testhooks

package compose

import "github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"

func WithMirrorAfterWrite(rows NativeMirrorRows, after func(string, bool, error) error) NativeMirrorRows {
	rows.Participants = application.WithMirrorAfterWrite(rows.Participants, after)
	return rows
}

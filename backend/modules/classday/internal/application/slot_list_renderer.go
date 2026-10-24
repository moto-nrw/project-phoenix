package application

import "github.com/moto-nrw/project-phoenix/modules/classday"

// ListRenderer renders a slot list export into a file. The root binds it to
// Document Rendering; rendering neither stores the file nor decides who may
// read it.
type ListRenderer interface {
	Render(doc classday.ExportDocument, format classday.Format, filenameBase string) (classday.File, error)
}

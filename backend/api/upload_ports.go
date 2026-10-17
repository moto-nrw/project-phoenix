package api

import (
	"io"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/delivery/objects"
)

func saveImageUpload(file io.Reader, targetDir, prefix, contentType string) (string, error) {
	return common.SaveImage(file, targetDir, prefix, contentType, objects.UploadFiles{})
}
func savePDFUpload(file io.Reader, targetDir, prefix string) (string, error) {
	return common.SavePDF(file, targetDir, prefix, objects.UploadFiles{})
}

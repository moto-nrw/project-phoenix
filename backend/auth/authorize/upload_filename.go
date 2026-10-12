package authorize

import "github.com/moto-nrw/project-phoenix/internal/randstr"

// UploadFilenameSuffix is the collision-resistant suffix of an uploaded file.
func UploadFilenameSuffix() (string, error) {
	return randstr.String(8, randstr.Alphanumeric)
}

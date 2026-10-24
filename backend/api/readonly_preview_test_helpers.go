//go:build testhooks

package api

import "github.com/moto-nrw/project-phoenix/api/common"

func readOnlyPreviewTestCode() string {
	return common.CodeIdentityReadOnlyPreview
}

//go:build testhooks

package api

import "github.com/moto-ogs/project-phoenix/api/common"

func readOnlyPreviewTestCode() string {
	return common.CodeIdentityReadOnlyPreview
}

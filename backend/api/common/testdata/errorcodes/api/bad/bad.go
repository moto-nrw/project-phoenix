package bad

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/sample"
)

const localCode = "care.announcement_stale"

func respond() []any {
	return []any{
		common.ErrorConflictWithCode(errors.New("x"), "announcement_stale"),
		common.ErrorConflictWithCode(errors.New("x"), common.CodeCareAnnouncementStale),
		&common.ErrResponse{Code: "care.announcement_stale"},
		common.ErrorConflictWithCode(errors.New("x"), sample.UnknownCode),
		map[string]string{"x": "care.announcement_stale"},
		localCode,
	}
}

func conflictCode(code string) *common.ErrResponse {
	return common.ErrorConflictWithCode(errors.New("x"), code)
}

var wrapped = conflictCode("announcement_stale")

var classified = common.ErrorConflictWithCode(errors.New("x"), sample.ConflictCode(true))

package data

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/api/common"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
)

// feedbackErrorRenderer maps the Feedback capability errors of the kiosk
// feedback route to the responses PyrePortal expects. It replaces the part
// of the retired shared IoT error mapper this route used (#2698).
var feedbackErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Match: isInvalidEntryData, Render: common.ErrorInvalidRequest},
	{Target: feedbackModule.ErrEntryNotFound, Render: common.ErrorNotFound},
	{Target: feedbackModule.ErrInvalidEntryData, Render: common.ErrorInvalidRequest},
	{Target: feedbackModule.ErrStudentNotFound, Render: common.ErrorNotFound},
	{Target: feedbackModule.ErrInvalidDateRange, Render: common.ErrorInvalidRequest},
}, common.ErrorInternalServer)

func isInvalidEntryData(err error) bool {
	var invalid *feedbackModule.InvalidEntryDataError
	return errors.As(err, &invalid)
}

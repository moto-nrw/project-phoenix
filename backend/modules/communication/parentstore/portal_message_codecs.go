package parentstore

import (
	models "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func portalFromRetainedParentMessagePointer(v *models.ParentMessage) (*communication.PortalParentMessage, error) {
	if v == nil {
		return nil, nil
	}
	out, err := portalFromRetainedParentMessage(*v)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
func portalFromRetainedParentMessageSlice(v []*models.ParentMessage) ([]*communication.PortalParentMessage, error) {
	if v == nil {
		return nil, nil
	}
	out := make([]*communication.PortalParentMessage, len(v))
	for i, row := range v {
		value, err := portalFromRetainedParentMessagePointer(row)
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	return out, nil
}
func portalToRetainedParentMessagePointer(v *communication.PortalParentMessage) (*models.ParentMessage, error) {
	if v == nil {
		return nil, nil
	}
	out, err := portalToRetainedParentMessage(*v)
	if err != nil {
		return nil, err
	}
	return &out, nil
}

func portalFromRetainedInboxThreadPointer(v *models.InboxThread) (*communication.PortalInboxThread, error) {
	if v == nil {
		return nil, nil
	}
	out, err := portalFromRetainedInboxThread(*v)
	if err != nil {
		return nil, err
	}
	return &out, nil
}
func portalFromRetainedInboxThreadSlice(v []*models.InboxThread) ([]*communication.PortalInboxThread, error) {
	if v == nil {
		return nil, nil
	}
	out := make([]*communication.PortalInboxThread, len(v))
	for i, row := range v {
		value, err := portalFromRetainedInboxThreadPointer(row)
		if err != nil {
			return nil, err
		}
		out[i] = value
	}
	return out, nil
}

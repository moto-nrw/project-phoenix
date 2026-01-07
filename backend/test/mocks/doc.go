// Package mocks provides reusable mock implementations for testing.
//
// These mocks implement full service interfaces using testify/mock,
// allowing flexible test setup with On().Return() patterns.
//
// Usage:
//
//	import "github.com/moto-nrw/project-phoenix/test/mocks"
//
//	func TestSomething(t *testing.T) {
//	    eduMock := mocks.NewEducationServiceMock()
//	    eduMock.On("GetTeacherGroups", mock.Anything, int64(1)).
//	        Return([]*education.Group{{ID: 1}}, nil)
//
//	    // Use mock in test...
//	    eduMock.AssertExpectations(t)
//	}
//
// All mocks implement the full interface with sensible defaults (nil, nil).
// Only set expectations for methods your test actually calls.
package mocks

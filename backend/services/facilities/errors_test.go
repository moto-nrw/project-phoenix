package facilities

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFacilitiesError_Error_WithNilErr(t *testing.T) {
	err := &FacilitiesError{
		Op:  "CreateRoom",
		Err: nil,
	}

	expected := "facilities error during CreateRoom"
	assert.Equal(t, expected, err.Error())
}

func TestFacilitiesError_Error_WithErr(t *testing.T) {
	originalErr := errors.New("database connection failed")
	err := &FacilitiesError{
		Op:  "CreateRoom",
		Err: originalErr,
	}

	expected := "facilities error during CreateRoom: database connection failed"
	assert.Equal(t, expected, err.Error())
}

func TestFacilitiesError_Error_WithStandardError(t *testing.T) {
	err := &FacilitiesError{
		Op:  "GetRoom",
		Err: ErrRoomNotFound,
	}

	expected := "facilities error during GetRoom: room not found"
	assert.Equal(t, expected, err.Error())
}

func TestFacilitiesError_Unwrap(t *testing.T) {
	originalErr := errors.New("underlying error")
	err := &FacilitiesError{
		Op:  "UpdateRoom",
		Err: originalErr,
	}

	assert.Equal(t, originalErr, err.Unwrap())
}

func TestFacilitiesError_Unwrap_Nil(t *testing.T) {
	err := &FacilitiesError{
		Op:  "DeleteRoom",
		Err: nil,
	}

	assert.Nil(t, err.Unwrap())
}

func TestFacilitiesError_ErrorsIs(t *testing.T) {
	// Test that errors.Is works correctly with wrapped errors
	err := &FacilitiesError{
		Op:  "FindRoom",
		Err: ErrRoomNotFound,
	}

	assert.True(t, errors.Is(err, ErrRoomNotFound))
	assert.False(t, errors.Is(err, ErrDuplicateRoom))
}

func TestFacilitiesError_ChainedWrapping(t *testing.T) {
	// Test multiple levels of wrapping
	baseErr := errors.New("connection timeout")
	wrapped1 := &FacilitiesError{
		Op:  "GetRoom",
		Err: baseErr,
	}
	wrapped2 := &FacilitiesError{
		Op:  "ListRooms",
		Err: wrapped1,
	}

	// Should unwrap through the chain
	assert.True(t, errors.Is(wrapped2, baseErr))
	assert.Contains(t, wrapped2.Error(), "ListRooms")
	assert.Contains(t, wrapped2.Error(), "GetRoom")
}

func TestFacilitiesError_AllOperations(t *testing.T) {
	tests := []struct {
		name string
		op   string
		err  error
		want string
	}{
		{
			name: "create operation",
			op:   "CreateRoom",
			err:  ErrDuplicateRoom,
			want: "facilities error during CreateRoom: room with this name already exists",
		},
		{
			name: "read operation",
			op:   "GetRoom",
			err:  ErrRoomNotFound,
			want: "facilities error during GetRoom: room not found",
		},
		{
			name: "update operation",
			op:   "UpdateRoom",
			err:  ErrInvalidRoomData,
			want: "facilities error during UpdateRoom: invalid room data",
		},
		{
			name: "delete operation",
			op:   "DeleteRoom",
			err:  ErrRoomNotFound,
			want: "facilities error during DeleteRoom: room not found",
		},
		{
			name: "capacity check",
			op:   "CheckCapacity",
			err:  ErrRoomCapacityExceeded,
			want: "facilities error during CheckCapacity: room capacity exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &FacilitiesError{
				Op:  tt.op,
				Err: tt.err,
			}
			assert.Equal(t, tt.want, err.Error())
		})
	}
}

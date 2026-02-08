package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOperator_Validate_EmptyEmail(t *testing.T) {
	o := &Operator{
		Email:       "",
		DisplayName: "Test Operator",
	}
	err := o.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email is required")
}

func TestOperator_Validate_EmailTooLong(t *testing.T) {
	longEmail := ""
	for i := 0; i < 256; i++ {
		longEmail += "a"
	}
	longEmail += "@test.com"
	o := &Operator{
		Email:       longEmail,
		DisplayName: "Test Operator",
	}
	err := o.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "email must not exceed 255 characters")
}

func TestOperator_Validate_EmailWithoutAt(t *testing.T) {
	o := &Operator{
		Email:       "invalidemail.com",
		DisplayName: "Test Operator",
	}
	err := o.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid email format")
}

func TestOperator_Validate_EmptyDisplayName(t *testing.T) {
	o := &Operator{
		Email:       "test@example.com",
		DisplayName: "",
	}
	err := o.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "display name is required")
}

func TestOperator_Validate_DisplayNameTooLong(t *testing.T) {
	longName := ""
	for i := 0; i < 101; i++ {
		longName += "a"
	}
	o := &Operator{
		Email:       "test@example.com",
		DisplayName: longName,
	}
	err := o.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "display name must not exceed 100 characters")
}

func TestOperator_Validate_Valid(t *testing.T) {
	o := &Operator{
		Email:       "test@example.com",
		DisplayName: "Test Operator",
	}
	err := o.Validate()
	assert.NoError(t, err)
}

func TestOperator_Validate_EmailNormalized(t *testing.T) {
	o := &Operator{
		Email:       "  TEST@Example.Com  ",
		DisplayName: "Test Operator",
	}
	err := o.Validate()
	assert.NoError(t, err)
	assert.Equal(t, "test@example.com", o.Email)
}

func TestOperator_Validate_DisplayNameTrimmed(t *testing.T) {
	o := &Operator{
		Email:       "test@example.com",
		DisplayName: "  Test Operator  ",
	}
	err := o.Validate()
	assert.NoError(t, err)
	assert.Equal(t, "Test Operator", o.DisplayName)
}

func TestOperator_TableName(t *testing.T) {
	o := &Operator{}
	assert.Equal(t, "platform.operators", o.TableName())
}

func TestOperator_GetID(t *testing.T) {
	o := &Operator{}
	o.ID = 456
	assert.Equal(t, int64(456), o.GetID())
}

func TestOperator_GetCreatedAt(t *testing.T) {
	now := time.Now()
	o := &Operator{}
	o.CreatedAt = now
	assert.Equal(t, now, o.GetCreatedAt())
}

func TestOperator_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	o := &Operator{}
	o.UpdatedAt = now
	assert.Equal(t, now, o.GetUpdatedAt())
}

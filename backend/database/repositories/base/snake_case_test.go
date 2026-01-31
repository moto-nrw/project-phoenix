package base

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestToSnakeCase_StudentGuardian(t *testing.T) {
	result := toSnakeCase("StudentGuardian")
	assert.Equal(t, "student_guardian", result)
}

func TestToSnakeCase_Teacher(t *testing.T) {
	result := toSnakeCase("Teacher")
	assert.Equal(t, "teacher", result)
}

func TestToSnakeCase_EmptyString(t *testing.T) {
	result := toSnakeCase("")
	assert.Equal(t, "", result)
}

func TestToSnakeCase_SingleChar(t *testing.T) {
	result := toSnakeCase("T")
	assert.Equal(t, "t", result)
}

func TestToSnakeCase_AllLowercase(t *testing.T) {
	result := toSnakeCase("teacher")
	assert.Equal(t, "teacher", result)
}

func TestToSnakeCase_ID(t *testing.T) {
	result := toSnakeCase("ID")
	assert.Equal(t, "i_d", result)
}

func TestToSnakeCase_HTMLParser(t *testing.T) {
	result := toSnakeCase("HTMLParser")
	assert.Equal(t, "h_t_m_l_parser", result)
}

func TestToSnakeCase_AlreadySnakeCase(t *testing.T) {
	result := toSnakeCase("student_guardian")
	assert.Equal(t, "student_guardian", result)
}

func TestToSnakeCase_MixedCase(t *testing.T) {
	result := toSnakeCase("StudentID")
	assert.Equal(t, "student_i_d", result)
}

func TestToSnakeCase_LeadingUppercase(t *testing.T) {
	result := toSnakeCase("ActiveGroup")
	assert.Equal(t, "active_group", result)
}

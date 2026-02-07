package suggestions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

// =============================================================================
// Validate Tests
// =============================================================================

func TestComment_Validate_ValidOperatorComment(t *testing.T) {
	c := &Comment{
		PostID:     1,
		AuthorID:   42,
		AuthorType: AuthorTypeOperator,
		Content:    "Valid operator comment",
	}

	err := c.Validate()
	assert.NoError(t, err)
	assert.Equal(t, "Valid operator comment", c.Content) // Content should be trimmed
}

func TestComment_Validate_ValidUserComment(t *testing.T) {
	c := &Comment{
		PostID:     1,
		AuthorID:   42,
		AuthorType: AuthorTypeUser,
		Content:    "Valid user comment",
	}

	err := c.Validate()
	assert.NoError(t, err)
}

func TestComment_Validate_TrimWhitespace(t *testing.T) {
	c := &Comment{
		PostID:     1,
		AuthorID:   42,
		AuthorType: AuthorTypeUser,
		Content:    "  Content with spaces  \n\t",
	}

	err := c.Validate()
	assert.NoError(t, err)
	assert.Equal(t, "Content with spaces", c.Content)
}

func TestComment_Validate_MissingPostID(t *testing.T) {
	c := &Comment{
		PostID:     0,
		AuthorID:   42,
		AuthorType: AuthorTypeUser,
		Content:    "Valid content",
	}

	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "post ID is required")
}

func TestComment_Validate_NegativePostID(t *testing.T) {
	c := &Comment{
		PostID:     -1,
		AuthorID:   42,
		AuthorType: AuthorTypeUser,
		Content:    "Valid content",
	}

	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "post ID is required")
}

func TestComment_Validate_MissingAuthorID(t *testing.T) {
	c := &Comment{
		PostID:     1,
		AuthorID:   0,
		AuthorType: AuthorTypeUser,
		Content:    "Valid content",
	}

	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "author ID is required")
}

func TestComment_Validate_NegativeAuthorID(t *testing.T) {
	c := &Comment{
		PostID:     1,
		AuthorID:   -1,
		AuthorType: AuthorTypeUser,
		Content:    "Valid content",
	}

	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "author ID is required")
}

func TestComment_Validate_InvalidAuthorType(t *testing.T) {
	c := &Comment{
		PostID:     1,
		AuthorID:   42,
		AuthorType: "invalid_type",
		Content:    "Valid content",
	}

	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "author type must be 'operator' or 'user'")
}

func TestComment_Validate_EmptyAuthorType(t *testing.T) {
	c := &Comment{
		PostID:     1,
		AuthorID:   42,
		AuthorType: "",
		Content:    "Valid content",
	}

	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "author type must be 'operator' or 'user'")
}

func TestComment_Validate_EmptyContent(t *testing.T) {
	c := &Comment{
		PostID:     1,
		AuthorID:   42,
		AuthorType: AuthorTypeUser,
		Content:    "",
	}

	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "content is required")
}

func TestComment_Validate_WhitespaceOnlyContent(t *testing.T) {
	c := &Comment{
		PostID:     1,
		AuthorID:   42,
		AuthorType: AuthorTypeUser,
		Content:    "   \n\t   ",
	}

	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "content is required")
}

func TestComment_Validate_ContentTooLong(t *testing.T) {
	longContent := make([]byte, 5001)
	for i := range longContent {
		longContent[i] = 'a'
	}

	c := &Comment{
		PostID:     1,
		AuthorID:   42,
		AuthorType: AuthorTypeUser,
		Content:    string(longContent),
	}

	err := c.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "content must not exceed 5000 characters")
}

func TestComment_Validate_ContentExactly5000Chars(t *testing.T) {
	content := make([]byte, 5000)
	for i := range content {
		content[i] = 'a'
	}

	c := &Comment{
		PostID:     1,
		AuthorID:   42,
		AuthorType: AuthorTypeUser,
		Content:    string(content),
	}

	err := c.Validate()
	assert.NoError(t, err)
}

// =============================================================================
// Table and Model Methods Tests
// =============================================================================

func TestComment_TableName(t *testing.T) {
	c := &Comment{}
	assert.Equal(t, "suggestions.comments", c.TableName())
}

func TestComment_GetID(t *testing.T) {
	c := &Comment{}
	c.ID = 123

	assert.Equal(t, int64(123), c.GetID())
}

func TestComment_GetCreatedAt(t *testing.T) {
	c := &Comment{}
	// Since CreatedAt comes from base.Model, just verify the method exists
	_ = c.GetCreatedAt()
}

func TestComment_GetUpdatedAt(t *testing.T) {
	c := &Comment{}
	// Since UpdatedAt comes from base.Model, just verify the method exists
	_ = c.GetUpdatedAt()
}

func TestComment_BeforeAppendModel_SelectQuery(t *testing.T) {
	c := &Comment{}
	// Create a mock SelectQuery - we can't easily test the actual behavior
	// without a real DB connection, but we can verify the method doesn't panic
	query := &bun.SelectQuery{}
	err := c.BeforeAppendModel(query)
	assert.NoError(t, err)
}

func TestComment_BeforeAppendModel_UpdateQuery(t *testing.T) {
	c := &Comment{}
	query := &bun.UpdateQuery{}
	err := c.BeforeAppendModel(query)
	assert.NoError(t, err)
}

func TestComment_BeforeAppendModel_DeleteQuery(t *testing.T) {
	c := &Comment{}
	query := &bun.DeleteQuery{}
	err := c.BeforeAppendModel(query)
	assert.NoError(t, err)
}

func TestComment_BeforeAppendModel_OtherQueryType(t *testing.T) {
	c := &Comment{}
	// Pass some other type - should not panic
	err := c.BeforeAppendModel("not a query")
	assert.NoError(t, err)
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestIsValidAuthorType_Operator(t *testing.T) {
	assert.True(t, isValidAuthorType(AuthorTypeOperator))
}

func TestIsValidAuthorType_User(t *testing.T) {
	assert.True(t, isValidAuthorType(AuthorTypeUser))
}

func TestIsValidAuthorType_Invalid(t *testing.T) {
	assert.False(t, isValidAuthorType("admin"))
	assert.False(t, isValidAuthorType("guest"))
	assert.False(t, isValidAuthorType(""))
	assert.False(t, isValidAuthorType("OPERATOR"))
	assert.False(t, isValidAuthorType("USER"))
}

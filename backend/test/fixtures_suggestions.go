package test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// CreateTestPost inserts a suggestions post for tenant 1.
func CreateTestPost(tb testing.TB, db *bun.DB, accountID int64, title, description string) *suggestions.Post {
	tb.Helper()

	post := &suggestions.Post{
		Title:       title,
		Description: description,
		AuthorID:    accountID,
		Status:      suggestions.StatusOpen,
		Score:       0,
	}
	post.SetTenantID(fixtureTenantID(tb))

	ctx, cancel := context.WithTimeout(TenantContext(fixtureTenantID(tb)), 5*time.Second)
	defer cancel()

	_, err := db.NewInsert().
		Model(post).
		ModelTableExpr("suggestions.posts").
		Returning("*").
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test post")

	return post
}

// CreateTestComment inserts a suggestions comment for tenant 1.
func CreateTestComment(tb testing.TB, db *bun.DB, postID, authorID int64, content, authorType string) *suggestions.Comment {
	tb.Helper()

	comment := &suggestions.Comment{
		PostID:     postID,
		AuthorID:   authorID,
		AuthorType: authorType,
		Content:    content,
	}
	comment.SetTenantID(fixtureTenantID(tb))

	ctx, cancel := context.WithTimeout(TenantContext(fixtureTenantID(tb)), 5*time.Second)
	defer cancel()

	_, err := db.NewInsert().
		Model(comment).
		ModelTableExpr("suggestions.comments").
		Returning("*").
		Exec(ctx)
	require.NoError(tb, err, "Failed to create test comment")

	return comment
}

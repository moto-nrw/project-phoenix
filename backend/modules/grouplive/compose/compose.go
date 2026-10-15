// Package compose names the Group Live view for the typed root: the sources
// it reads and its constructor. The view's adapter stays inside the owner
// (ADR 0053 point 2).
package compose

import grouplivelegacy "github.com/moto-nrw/project-phoenix/modules/grouplive/legacy"

// Sources are the owner capabilities the live-group view reads.
type Sources = grouplivelegacy.Sources

// CallerContext is the caller the live-group view resolves its groups for,
// as the root's Identity & Access composition serves it (#4006).
type CallerContext = grouplivelegacy.CallerContext

// New builds the live-group view over its sources.
var New = grouplivelegacy.New

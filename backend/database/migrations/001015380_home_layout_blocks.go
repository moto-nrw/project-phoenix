package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	homeLayoutBlocksVersion     = "1.15.380"
	homeLayoutBlocksDescription = "Store the personal arrangement of start page blocks: order and width (#2180)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: homeLayoutBlocksVersion, Description: homeLayoutBlocksDescription,
		DependsOn: []string{homeLayoutsVersion},
	})
	Migrations.MustRegister(homeLayoutBlocksUp, homeLayoutBlocksDown)
}

// homeLayoutBlocksUp adds the ordered arrangement to config.home_layouts.
//
// #2875 stored visibility only (a map of deviations), which was enough while
// the start page was a fixed sequence of blocks. Since #2180 every person
// composes their own start page: which blocks, in which ORDER, and how WIDE
// each one is. Order and width have no place in a map, so they get their own
// column — a jsonb ARRAY of {key, span} in display order.
//
// Both columns stay, and they answer different questions. `blocks` is the
// arrangement of everything the person placed; `overrides` keeps saying which
// blocks the person deliberately removed. That distinction is what lets a
// block introduced in a later release appear for existing accounts: it is
// neither arranged nor removed, so the start page appends it at its default
// place instead of hiding it from everyone who ever opened the dialog.
//
// An empty array is "never arranged anything" and yields the recommended start
// page of that person's role, exactly like an empty map does today. As with
// `overrides`, keys are not constrained against a catalogue here: the
// catalogue lives in the frontend, the API validates the shape of a key and
// the span, and a key nobody renders any more is inert.
func homeLayoutBlocksUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		ALTER TABLE config.home_layouts
			ADD COLUMN IF NOT EXISTS blocks JSONB NOT NULL DEFAULT '[]'::jsonb;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("add blocks column to config.home_layouts: %w", err)
	}
	return nil
}

func homeLayoutBlocksDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.NewRaw(`
		ALTER TABLE config.home_layouts DROP COLUMN IF EXISTS blocks;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("drop blocks column from config.home_layouts: %w", err)
	}
	return nil
}

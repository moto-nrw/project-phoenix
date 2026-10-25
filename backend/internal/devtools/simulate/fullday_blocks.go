package simulate

import (
	"context"
	"fmt"
	"slices"
)

// startedBlock is a planned block the run started.
type startedBlock struct {
	dayBlock
	Completed bool
}

// startPlannedBlocks starts every planned block of today whose window holds
// the time of day c, with its planned staff: the start copies the staff who
// are not marked absent, a substitute included, into the supervisors. The
// block's room then takes the children of its roster.
func (rt *Runtime) startPlannedBlocks(c wallClock) ([]startedBlock, error) {
	var started []startedBlock
	for i, block := range rt.Today.Blocks {
		if !block.startable() || !block.runsAt(c) || block.RoomID == 0 {
			continue
		}
		if _, err := rt.Client.Post(fmt.Sprintf("/api/timetable/instances/%d/start", block.ID), nil); err != nil {
			return started, fmt.Errorf("start planned block %s: %w", block.Title, err)
		}
		rt.Today.Blocks[i].Status = "active"
		run := startedBlock{dayBlock: block}
		rt.Blocks = append(rt.Blocks, run)
		rt.Rooms.openBlock(block)
		rt.Counts.BlocksStarted++
		started = append(started, run)
		fmt.Printf("  Started: %s %s–%s (planned block, %d staff)\n", block.Title, block.StartTime, block.EndTime, len(block.presentStaff()))
	}
	return started, nil
}

// completeBlocks completes the running blocks the run started before the
// time of day c; their rooms take no more children.
func (rt *Runtime) completeBlocks(before wallClock) error {
	for i, block := range rt.Blocks {
		start, ok := parseWallClock(block.StartTime)
		if block.Completed || !ok || start >= before {
			continue
		}
		if _, err := rt.Client.Post(fmt.Sprintf("/api/timetable/instances/%d/complete", block.ID), nil); err != nil {
			return fmt.Errorf("complete planned block %s: %w", block.Title, err)
		}
		rt.Blocks[i].Completed = true
		rt.Rooms.closeBlock(block.dayBlock)
	}
	return nil
}

// rosterChildren lists the children of the block's roster who come today and
// carry a tag, in roster order.
func (rt *Runtime) rosterChildren(block dayBlock) []todayChild {
	var children []todayChild
	for _, id := range block.StudentIDs {
		child, ok := rt.child(id)
		if _, tagged := rt.RFIDTags[id]; ok && tagged && !child.Absent {
			children = append(children, child)
		}
	}
	return children
}

type earlyCareAction struct{}

func (earlyCareAction) Name() string { return "run early care" }

// Run opens the Frühbetreuung as planned: the planned blocks running at
// earlyCareStart start with their planned staff, and the children of their
// rosters arrive one per minute. A school without such a block has no early
// care.
func (earlyCareAction) Run(ctx context.Context, rt *Runtime) error {
	fmt.Println("\nPhase 3: Frühbetreuung...")
	blocks, err := rt.startPlannedBlocks(earlyCareStart)
	if err != nil {
		return err
	}
	if len(blocks) == 0 {
		fmt.Println("  Skipped: no block planned before school")
		return nil
	}
	device, err := rt.primaryDevice()
	if err != nil {
		return err
	}
	for _, block := range blocks {
		if err := rt.earlyChildrenArrive(ctx, block.dayBlock, device); err != nil {
			return err
		}
	}
	fmt.Printf("  %d children in early care\n", rt.Counts.EarlyCareChildren)
	return nil
}

// earlyChildrenArrive lets the children of the block's roster arrive one per
// minute from its start and checks them into its room.
func (rt *Runtime) earlyChildrenArrive(ctx context.Context, block dayBlock, device SeedDevice) error {
	start, _ := parseWallClock(block.StartTime)
	for _, child := range rt.rosterChildren(block) {
		if rt.Present[child.ID] {
			continue
		}
		if err := rt.advanceClock(ctx, max(start, earlyCareStart)+wallClock(rt.Counts.EarlyCareChildren)); err != nil {
			return err
		}
		if err := rt.confirmAttendance(child.SeedStudent, device); err != nil {
			return err
		}
		if err := rt.checkInto(child.SeedStudent, block.RoomID, device); err != nil {
			return err
		}
		rt.Rooms.record(child.ID, block.RoomID)
		rt.Counts.EarlyCareChildren++
	}
	return nil
}

type earlyCareEndAction struct{}

func (earlyCareEndAction) Name() string { return "end early care" }

// Run sends the early care children to their lessons and completes the
// morning blocks. Their arrival stays on record for the day.
func (earlyCareEndAction) Run(_ context.Context, rt *Runtime) error {
	if rt.Counts.EarlyCareChildren == 0 && len(rt.Blocks) == 0 {
		return nil
	}
	device, err := rt.primaryDevice()
	if err != nil {
		return err
	}
	for _, id := range slices.Clone(rt.Rooms.order) {
		child, _ := rt.child(id)
		if err := rt.leaveRoom(child.SeedStudent, device); err != nil {
			return err
		}
		rt.Rooms.leave(id)
	}
	return rt.completeBlocks(earlyCareEnd + 1)
}

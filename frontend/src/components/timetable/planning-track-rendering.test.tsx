import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  assignBlockLanes,
  comparePlanningInstances,
  mapInstance,
  mapTemplates,
} from "~/lib/timetable-helpers";
import type { EnrichedInstance } from "~/lib/timetable-types";
import { buildPlanningTrackLegend } from "./betreuungsplan-view";
import { InstanceBlock } from "./instance-block";

function instance(overrides: Partial<EnrichedInstance> = {}): EnrichedInstance {
  return {
    id: "1",
    date: "2026-08-03",
    startTime: "12:00",
    endTime: "13:00",
    title: "Mensa",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "care",
    roomId: "2",
    roomName: "Mensa",
    staff: [],
    students: [],
    studentIds: [],
    staffCount: 0,
    absentStaffCount: 0,
    expectedStudentsCount: 0,
    notScheduledStudentsCount: 0,
    presentStudentsCount: 0,
    requiredStaffCount: 0,
    assignedStaffCount: 0,
    conflictWarnings: [],
    ...overrides,
  };
}

describe("planning track rendering", () => {
  it("maps planning track metadata from the backend response", () => {
    const mapped = mapInstance({
      id: 1,
      date: "2026-08-03",
      start_time: "12:00",
      end_time: "13:00",
      title: "Mensa",
      status: "planned",
      is_spontaneous: false,
      is_live: false,
      activity_type: "care",
      planning_track_id: 8,
      planning_track_name: "Mittag",
      planning_track_color: "#F78C10",
      planning_track_sort_order: 2,
      room_id: 2,
      room_name: "Mensa",
      staff: [],
      staff_count: 0,
      absent_staff_count: 0,
      expected_students_count: 0,
      present_students_count: 0,
      required_staff_count: 0,
      assigned_staff_count: 0,
    });

    expect(mapped).toMatchObject({
      planningTrackId: "8",
      planningTrackName: "Mittag",
      planningTrackColor: "#F78C10",
      planningTrackSortOrder: 2,
    });
  });

  it("uses track color and exposes its name to assistive technology", () => {
    render(
      <InstanceBlock
        instance={instance({
          planningTrackId: "8",
          planningTrackName: "Mittag",
          planningTrackColor: "#F78C10",
        })}
        top={0}
        height={90}
        left="0%"
        width="100%"
        isSelected={false}
        onClick={vi.fn()}
      />,
    );

    expect(screen.getByRole("button")).toHaveStyle({
      borderLeft: "3px solid #F78C10",
      backgroundColor: "#F78C101A",
    });
    expect(screen.getByRole("button")).toHaveAccessibleName(
      /Planungsspur Mittag/,
    );
    expect(screen.getByText("Mittag")).toBeInTheDocument();
  });

  it("orders simultaneous blocks by planning track before id", () => {
    const result = assignBlockLanes([
      instance({ id: "30", planningTrackSortOrder: 2 }),
      instance({ id: "20", planningTrackSortOrder: 1 }),
      instance({ id: "10", planningTrackSortOrder: 1 }),
    ]);

    expect(result.map((item) => item.instance.id)).toEqual(["10", "20", "30"]);
    expect(result.map((item) => item.lane)).toEqual([0, 1, 2]);
  });

  it("uses the same stable order in non-lane planning views", () => {
    const result = [
      instance({ id: "30", planningTrackSortOrder: 2 }),
      instance({ id: "20", planningTrackSortOrder: 1 }),
      instance({ id: "10", planningTrackSortOrder: 1 }),
    ].sort(comparePlanningInstances);

    expect(result.map((item) => item.id)).toEqual(["10", "20", "30"]);
  });

  it("maps template metadata independently from category and target group", () => {
    const result = mapTemplates({
      templates: [
        {
          id: 4,
          name: "Lernzeit 1",
          type: "care",
          category_id: 9,
          category_name: "Lernzeit",
          planning_track_id: 8,
          planning_track_name: "Jahrgang 1",
          planning_track_color: "#5080D8",
          planning_track_sort_order: 1,
          is_open: true,
          max_participants: 20,
          target_group_type: "jahrgang",
          target_grade_level: 1,
          enrollment_count: 0,
          supervisor_count: 0,
          required_staff_count: 0,
          assigned_staff_count: 0,
          student_ids: [],
          staff_ids: [],
          schedules: [],
        },
      ],
    });

    expect(result.templates[0]).toMatchObject({
      categoryName: "Lernzeit",
      planningTrackName: "Jahrgang 1",
      targetGroupType: "jahrgang",
      targetGradeLevel: 1,
    });
  });

  it("builds a sorted legend with a neutral unassigned entry", () => {
    const entries = buildPlanningTrackLegend([
      instance({
        id: "2",
        planningTrackId: "20",
        planningTrackName: "Nord",
        planningTrackColor: "#F78C10",
        planningTrackSortOrder: 2,
      }),
      instance({
        id: "1",
        planningTrackId: "10",
        planningTrackName: "Süd",
        planningTrackColor: "#5080D8",
        planningTrackSortOrder: 1,
      }),
      instance({ id: "3" }),
    ]);

    expect(entries).toEqual([
      { key: "10", label: "Süd", color: "#5080D8" },
      { key: "20", label: "Nord", color: "#F78C10" },
      { key: "unassigned", label: "Ohne Planungsspur" },
    ]);
  });
});

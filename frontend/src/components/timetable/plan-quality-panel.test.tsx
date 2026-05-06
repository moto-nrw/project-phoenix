import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { PlanQualityPanel } from "./plan-quality-panel";
import type { Staff } from "~/lib/staff-api";
import type {
  EnrichedInstance,
  ExceptionConflict,
  GapInstance,
} from "~/lib/timetable-types";

function instance(overrides: Partial<EnrichedInstance> = {}): EnrichedInstance {
  return {
    id: "42",
    date: "2026-05-04",
    startTime: "12:00",
    endTime: "13:00",
    title: "Mensa",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "care",
    roomId: "3",
    roomName: "Mensa",
    staff: [
      {
        staffId: "11",
        isPrimary: true,
        isAbsent: true,
        isSubstitute: false,
      },
    ],
    studentIds: [],
    students: [],
    staffCount: 1,
    absentStaffCount: 1,
    expectedStudentsCount: 0,
    presentStudentsCount: 0,
    conflictWarnings: [
      {
        kind: "room",
        resourceId: "3",
        message: "Raum doppelt",
        canOverride: true,
      },
    ],
    ...overrides,
  };
}

const gap: GapInstance = {
  instanceId: "42",
  date: "2026-05-04",
  title: "Mensa",
  startTime: "12:00",
  endTime: "13:00",
  roomId: "3",
  status: "planned",
  assignedStaffCount: 1,
  absentStaffCount: 1,
};

const conflict: ExceptionConflict = {
  kind: "modified_instance_time_mismatch",
  date: "2026-05-04",
  activityGroupId: "7",
  instanceId: "42",
  activityTitle: "Mensa",
  studentId: "21",
  arrivalSource: "schedule",
  originalStartTime: "12:00",
  modifiedStartTime: "12:30",
};

const staff: Staff[] = [
  {
    id: "11",
    name: "Ada Absent",
    firstName: "Ada",
    lastName: "Absent",
    hasRfid: false,
    isTeacher: false,
    isSupervising: false,
    supervisions: [],
    workStatus: "checked_in",
  },
  {
    id: "12",
    name: "Erika Ersatz",
    firstName: "Erika",
    lastName: "Ersatz",
    hasRfid: false,
    isTeacher: false,
    isSupervising: false,
    supervisions: [],
    workStatus: "checked_in",
  },
  {
    id: "13",
    name: "Carl Checkout",
    firstName: "Carl",
    lastName: "Checkout",
    hasRfid: false,
    isTeacher: false,
    isSupervising: false,
    supervisions: [],
    workStatus: "checked_out",
  },
];

describe("PlanQualityPanel", () => {
  it("summarizes clean, loading and error states", () => {
    const { rerender } = render(
      <PlanQualityPanel
        instances={[]}
        gaps={[]}
        conflicts={[]}
        staff={[]}
        loading={false}
        onSelectInstance={vi.fn()}
        onEditInstance={vi.fn()}
        onSubstitute={vi.fn()}
      />,
    );

    expect(
      screen.getByText("Diese Ansicht hat keine offenen Planungsprobleme."),
    ).toBeInTheDocument();

    rerender(
      <PlanQualityPanel
        instances={[]}
        gaps={[]}
        conflicts={[]}
        staff={[]}
        loading
        onSelectInstance={vi.fn()}
        onEditInstance={vi.fn()}
        onSubstitute={vi.fn()}
      />,
    );
    expect(screen.getByText(/Prüfe Personal/)).toBeInTheDocument();

    rerender(
      <PlanQualityPanel
        instances={[]}
        gaps={[]}
        conflicts={[]}
        staff={[]}
        loading={false}
        error="Backend weg"
        onSelectInstance={vi.fn()}
        onEditInstance={vi.fn()}
        onSubstitute={vi.fn()}
      />,
    );
    expect(screen.getByText("Backend weg")).toBeInTheDocument();
  });

  it("selects/edit instances and applies staff substitution", async () => {
    const onSelectInstance = vi.fn();
    const onEditInstance = vi.fn();
    const onSubstitute = vi.fn().mockResolvedValue(undefined);

    render(
      <PlanQualityPanel
        instances={[instance()]}
        gaps={[gap]}
        conflicts={[conflict]}
        staff={staff}
        loading={false}
        onSelectInstance={onSelectInstance}
        onEditInstance={onEditInstance}
        onSubstitute={onSubstitute}
      />,
    );

    expect(
      screen.getByText(
        "3 Punkt(e), die vor dem Betrieb geprüft werden sollten.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Abwesend: Ada Absent")).toBeInTheDocument();
    expect(screen.queryByText("Carl Checkout")).not.toBeInTheDocument();

    fireEvent.click(screen.getAllByRole("button", { name: /Mo/ })[0]!);
    expect(onSelectInstance).toHaveBeenCalledWith("42");

    fireEvent.change(screen.getByRole("combobox"), {
      target: { value: "12" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Anwenden" }));

    await waitFor(() =>
      expect(onSubstitute).toHaveBeenCalledWith("11", "12", "2026-05-04"),
    );

    fireEvent.click(screen.getAllByRole("button", { name: /Mensa/ })[1]!);
    expect(onSelectInstance).toHaveBeenCalledWith("42");

    cleanup();
    const assignGap = {
      ...gap,
      instanceId: "99",
      assignedStaffCount: 0,
      absentStaffCount: 0,
    };
    render(
      <PlanQualityPanel
        instances={[instance({ id: "99", staff: [], conflictWarnings: [] })]}
        gaps={[assignGap]}
        conflicts={[]}
        staff={staff}
        loading={false}
        onSelectInstance={vi.fn()}
        onEditInstance={onEditInstance}
        onSubstitute={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Personal zuordnen" }));
    expect(onEditInstance).toHaveBeenCalledWith("99");
  });
});

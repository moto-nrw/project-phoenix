// Die Auswahlleiste und die Listendarstellung für "Betreuung beenden" (#2487).
//
// Eigene Datei, weil students-master-detail.test.tsx bereits sehr breit ist
// und die Auswahl-Mocks dort anders zugeschnitten sind.
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

import type { Student } from "~/lib/student-helpers";
import { StudentsMasterDetail } from "./students-master-detail";

vi.mock("~/components/database/master-detail-layout", () => ({
  MasterDetailLayout: (props: { list: React.ReactNode }) => (
    <div>{props.list}</div>
  ),
}));

vi.mock("~/components/database/grouped-list", () => ({
  GroupedList: (props: {
    groups: Array<{ id: string; items: Student[] }>;
    renderItem: (item: Student) => React.ReactNode;
    keyFor: (item: Student) => string;
    emptyState: React.ReactNode;
  }) => (
    <ul>
      {props.groups.flatMap((group) =>
        group.items.map((item) => (
          <li key={props.keyFor(item)}>{props.renderItem(item)}</li>
        )),
      )}
    </ul>
  ),
}));

vi.mock("~/components/database/database-list-item", () => ({
  DatabaseListItem: (props: {
    title: string;
    subtitle: React.ReactNode;
    isChecked?: boolean;
  }) => (
    <div data-testid={`item-${props.title}`}>
      <span>{props.title}</span>
      <span data-testid={`subtitle-${props.title}`}>{props.subtitle}</span>
    </div>
  ),
}));

vi.mock("~/lib/settings-api", () => ({
  fetchSettingsSchema: vi.fn().mockResolvedValue({ tabs: [] }),
}));

function makeStudent(id: string, overrides: Partial<Student> = {}): Student {
  return {
    id,
    name: `S ${id}`,
    first_name: `First${id}`,
    second_name: `Last${id}`,
    school_class: "3a",
    current_location: "class",
    group_name: "Füchse",
    group_id: "10",
    ...overrides,
  } as Student;
}

const baseProps = {
  selectedId: null,
  onSelect: vi.fn(),
  onArrivalDataChanged: vi.fn(),
  groups: [] as Array<{ value: string; label: string }>,
  onUpdateStudent: vi.fn(),
  studentsWithArrival: new Set<string>(["1", "2", "3"]),
  arrivalSummaryById: new Map<string, string>(),
  grouping: "class" as const,
};

describe("StudentsMasterDetail — Betreuung beenden", () => {
  beforeEach(() => vi.clearAllMocks());

  it("selects exactly the children currently shown", () => {
    const onSelectAllVisible = vi.fn();
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1"), makeStudent("2")]}
        selectionMode
        selectedStudentIds={new Set()}
        onSelectAllVisible={onSelectAllVisible}
      />,
    );

    const button = screen.getByRole("button", { name: "Alle 2 auswählen" });
    fireEvent.click(button);
    expect(onSelectAllVisible).toHaveBeenCalledWith(["1", "2"]);
  });

  it("offers 'Betreuung beenden' only with the delete permission", () => {
    const { rerender } = render(
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1")]}
        selectionMode
        selectedStudentIds={new Set(["1"])}
      />,
    );
    expect(
      screen.queryByRole("button", { name: "Betreuung beenden" }),
    ).toBeNull();

    const onEndCare = vi.fn();
    rerender(
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1")]}
        selectionMode
        selectedStudentIds={new Set(["1"])}
        onEndCare={onEndCare}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Betreuung beenden" }));
    expect(onEndCare).toHaveBeenCalled();
  });

  it("disables the bulk action while nothing is selected", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1")]}
        selectionMode
        selectedStudentIds={new Set()}
        onEndCare={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", { name: "Betreuung beenden" }),
    ).toBeDisabled();
  });

  it("labels a planned exit in the list", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[
          makeStudent("1", {
            care_ends_on: "2026-09-30",
            care_ended: false,
            care_exit_recorded: true,
          }),
        ]}
      />,
    );
    expect(screen.getByTestId("subtitle-First1 Last1").textContent).toContain(
      "Betreuung endet am 30.09.2026",
    );
  });

  it("says nothing about a mere end of the enrolment phase", () => {
    // Ohne eingetragenen Austritt ist das Datum nur die Laufzeit der
    // Anmeldung. Stünde es in der Liste, hätte fast jedes Kind den Hinweis
    // und der eine echte Austritt ginge darin unter (#2487).
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[
          makeStudent("1", { care_ends_on: "2027-07-31", care_ended: false }),
        ]}
      />,
    );
    expect(
      screen.getByTestId("subtitle-First1 Last1").textContent,
    ).not.toContain("Betreuung endet");
  });

  it("says nothing about a child without a planned exit", () => {
    render(
      <StudentsMasterDetail {...baseProps} students={[makeStudent("1")]} />,
    );
    expect(
      screen.getByTestId("subtitle-First1 Last1").textContent,
    ).not.toContain("Betreuung endet");
  });
});

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { Student } from "~/lib/student-helpers";

// ─────────────────────────────────────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────────────────────────────────────

vi.mock("~/components/database/database-list-layout", () => ({
  DatabaseListLayout: (props: { children: React.ReactNode }) => (
    <div data-testid="list-layout">{props.children}</div>
  ),
}));

vi.mock("~/components/database/grouped-list", () => ({
  GroupedList: (props: {
    groups: Array<{
      id: string;
      title: string;
      items: Student[];
      variant?: string;
      countSuffix?: string;
      bulkAction?: React.ReactNode;
    }>;
    renderItem: (item: Student) => React.ReactNode;
    keyFor: (item: Student) => string;
    emptyState: React.ReactNode;
  }) => {
    if (props.groups.length === 0) {
      return <div data-testid="groups-empty">{props.emptyState}</div>;
    }
    return (
      <ul>
        {props.groups.map((g) => (
          <li key={g.id} data-testid={`group-${g.id}`}>
            <div data-testid={`group-title-${g.id}`}>{g.title}</div>
            <div data-testid={`group-variant-${g.id}`}>{g.variant}</div>
            <div data-testid={`group-suffix-${g.id}`}>
              {g.countSuffix ?? ""}
            </div>
            <div>{g.bulkAction}</div>
            <ul>
              {g.items.map((item) => (
                <li key={props.keyFor(item)}>{props.renderItem(item)}</li>
              ))}
            </ul>
          </li>
        ))}
      </ul>
    );
  },
}));

vi.mock("./class-bulk-arrival-modal", () => ({
  FilteredBulkArrivalModal: (props: {
    isOpen: boolean;
    onClose: () => void;
    filter:
      | { type: "school_class"; schoolClass: string }
      | { type: "group"; groupId: string }
      | { type: "students"; studentIds: string[] };
    filterLabel: string;
    studentsInFilter: Student[];
    onSuccess?: () => void;
  }) =>
    props.isOpen ? (
      <div
        data-testid="bulk-modal"
        data-filter-type={props.filter.type}
        data-filter-value={
          props.filter.type === "school_class"
            ? props.filter.schoolClass
            : props.filter.type === "group"
              ? props.filter.groupId
              : props.filter.studentIds.join(",")
        }
        data-filter-label={props.filterLabel}
        data-student-count={props.studentsInFilter.length}
      >
        <button type="button" onClick={props.onClose} data-testid="bulk-close">
          close
        </button>
        <button
          type="button"
          onClick={() => props.onSuccess?.()}
          data-testid="bulk-success"
        >
          success
        </button>
      </div>
    ) : null,
}));

vi.mock("./selection-bulk-pickup-modal", () => ({
  SelectionBulkPickupModal: (props: {
    studentIds: string[];
    onClose: () => void;
  }) => (
    <div
      data-testid="pickup-selection-modal"
      data-student-ids={props.studentIds.join(",")}
    >
      <button type="button" onClick={props.onClose}>
        close pickup
      </button>
    </div>
  ),
}));

vi.mock("./class-trip-bulk-status-modal", () => ({
  ClassTripBulkStatusModal: (props: {
    students: Student[];
    onClose: () => void;
  }) => (
    <div
      data-testid="class-trip-selection-modal"
      data-student-ids={props.students.map((student) => student.id).join(",")}
    >
      <button type="button" onClick={props.onClose}>
        close class trip
      </button>
    </div>
  ),
}));

vi.mock("~/components/database/database-list-item", () => ({
  DatabaseListItem: (props: {
    title: string;
    subtitle: React.ReactNode;
    href?: string;
    trailingAccessory?: React.ReactNode;
    selectionMode?: boolean;
    isChecked?: boolean;
  }) => {
    const idMatch = /First(\S+)/.exec(props.title);
    const idSlug = idMatch ? idMatch[1] : props.title;
    return (
      <a
        href={props.href}
        data-testid={`student-${idSlug}`}
        data-trailing={props.trailingAccessory ? "true" : ""}
        data-selection-mode={props.selectionMode ? "true" : "false"}
        data-checked={props.isChecked ? "true" : "false"}
      >
        {props.title}
        <span data-testid={`subtitle-${props.title}`}>{props.subtitle}</span>
      </a>
    );
  },
}));

import { StudentsList } from "./students-list";

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

const objectHref = (student: Student) =>
  `/students/${student.id}?from=%2Fdatabase%2Fstudents`;

describe("StudentsList", () => {
  const baseProps = {
    onArrivalDataChanged: vi.fn(),
    objectHref,
    arrivalSummaryById: new Map<string, string>(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows empty state when students list is empty", () => {
    render(
      <StudentsList
        {...baseProps}
        students={[]}
        grouping="class"
        studentsWithArrival={new Set()}
      />,
    );

    expect(screen.getByTestId("groups-empty")).toBeInTheDocument();
    expect(screen.getByText("Keine Kinder gefunden.")).toBeInTheDocument();
  });

  it("links every row to the child's object route", () => {
    render(
      <StudentsList
        {...baseProps}
        students={[makeStudent("1"), makeStudent("2")]}
        grouping="none"
        studentsWithArrival={new Set(["1", "2"])}
      />,
    );

    expect(screen.getByTestId("student-1")).toHaveAttribute(
      "href",
      "/students/1?from=%2Fdatabase%2Fstudents",
    );
    expect(screen.getByTestId("student-2")).toHaveAttribute(
      "href",
      "/students/2?from=%2Fdatabase%2Fstudents",
    );
  });

  it("renders a flat single-group list when grouping is 'none'", () => {
    render(
      <StudentsList
        {...baseProps}
        students={[makeStudent("1"), makeStudent("2")]}
        grouping="none"
        studentsWithArrival={new Set(["1", "2"])}
      />,
    );

    expect(screen.getByTestId("group-title-__flat__")).toHaveTextContent(
      "Alle Kinder",
    );
  });

  it("groups by class and shows warning variant when some miss arrival", () => {
    render(
      <StudentsList
        {...baseProps}
        students={[
          makeStudent("1", { school_class: "3a" }),
          makeStudent("2", { school_class: "3a" }),
          makeStudent("3", { school_class: "3b" }),
        ]}
        grouping="class"
        studentsWithArrival={new Set(["1"])}
      />,
    );

    expect(screen.getByTestId("group-title-3a")).toHaveTextContent("3a");
    expect(screen.getByTestId("group-variant-3a")).toHaveTextContent("warning");
    expect(screen.getByTestId("group-suffix-3a")).toHaveTextContent(
      "· 1 offen",
    );
  });

  it("labels fallback groups when class or group are missing", () => {
    render(
      <StudentsList
        {...baseProps}
        students={[
          makeStudent("1", { school_class: "" }),
          makeStudent("2", { school_class: "3a" }),
        ]}
        grouping="class"
        studentsWithArrival={new Set(["1", "2"])}
      />,
    );
    expect(screen.getByTestId("group-title-Ohne Klasse")).toBeInTheDocument();
  });

  it("groups by group_name and falls back on missing name", () => {
    render(
      <StudentsList
        {...baseProps}
        students={[
          makeStudent("1", { group_name: "Füchse" }),
          makeStudent("2", { group_name: "", group_id: undefined }),
        ]}
        grouping="group"
        studentsWithArrival={new Set(["1", "2"])}
      />,
    );

    expect(screen.getByTestId("group-title-group-10")).toHaveTextContent(
      "Füchse",
    );
    expect(
      screen.getByTestId("group-title-__without_group__"),
    ).toHaveTextContent("Ohne Gruppe");
  });

  it("passes arrival summary and arrival flag to list items", () => {
    render(
      <StudentsList
        {...baseProps}
        students={[makeStudent("1"), makeStudent("2")]}
        grouping="class"
        studentsWithArrival={new Set(["1"])}
        arrivalSummaryById={new Map([["1", "Mo-Fr 08:00"]])}
      />,
    );

    const one = screen.getByTestId("student-1");
    expect(one).toHaveAttribute("data-trailing", "");
    expect(one).toHaveTextContent("Mo-Fr 08:00");

    const two = screen.getByTestId("student-2");
    expect(two).toHaveAttribute("data-trailing", "true");
    expect(two).toHaveTextContent("keine Ankunft");
  });

  it("renders class actions menu only for known class groups", () => {
    render(
      <StudentsList
        {...baseProps}
        students={[
          makeStudent("1", { school_class: "3a" }),
          makeStudent("2", { school_class: "" }),
        ]}
        grouping="class"
        studentsWithArrival={new Set(["1", "2"])}
      />,
    );

    expect(screen.getByLabelText("Aktionen für 3a")).toBeInTheDocument();
    expect(
      screen.queryByLabelText("Aktionen für Ohne Klasse"),
    ).not.toBeInTheDocument();
  });

  it("renders arrival actions when grouping by a known group", () => {
    render(
      <StudentsList
        {...baseProps}
        students={[
          makeStudent("1", {
            school_class: "3a",
            group_id: "17",
            group_name: "Füchse",
          }),
        ]}
        grouping="group"
        studentsWithArrival={new Set(["1"])}
      />,
    );

    expect(screen.getByLabelText("Aktionen für Füchse")).toBeInTheDocument();
  });

  it("opens bulk arrival modal from class actions menu", () => {
    render(
      <StudentsList
        {...baseProps}
        students={[makeStudent("1", { school_class: "3a" })]}
        grouping="class"
        studentsWithArrival={new Set(["1"])}
      />,
    );

    fireEvent.click(screen.getByLabelText("Aktionen für 3a"));
    fireEvent.click(
      screen.getByRole("menuitem", { name: /Ankunftszeit bearbeiten/ }),
    );

    expect(screen.getByTestId("bulk-modal")).toHaveAttribute(
      "data-filter-type",
      "school_class",
    );
    expect(screen.getByTestId("bulk-modal")).toHaveAttribute(
      "data-filter-value",
      "3a",
    );
  });

  it("opens bulk arrival modal with the selected group filter", () => {
    render(
      <StudentsList
        {...baseProps}
        students={[makeStudent("1", { group_id: "17", group_name: "Füchse" })]}
        grouping="group"
        studentsWithArrival={new Set(["1"])}
      />,
    );

    fireEvent.click(screen.getByLabelText("Aktionen für Füchse"));
    fireEvent.click(
      screen.getByRole("menuitem", { name: /Ankunftszeit bearbeiten/ }),
    );

    expect(screen.getByTestId("bulk-modal")).toHaveAttribute(
      "data-filter-type",
      "group",
    );
    expect(screen.getByTestId("bulk-modal")).toHaveAttribute(
      "data-filter-value",
      "17",
    );
  });

  it("uses the unfiltered cohort for the bulk preview and class trips", () => {
    render(
      <StudentsList
        {...baseProps}
        students={[makeStudent("1", { school_class: "3a" })]}
        bulkStudents={[
          makeStudent("1", { school_class: "3a" }),
          makeStudent("2", { school_class: "3a" }),
        ]}
        grouping="class"
        studentsWithArrival={new Set(["1"])}
      />,
    );

    fireEvent.click(screen.getByLabelText("Aktionen für 3a"));
    fireEvent.click(
      screen.getByRole("menuitem", { name: /Ankunftszeit bearbeiten/ }),
    );
    expect(screen.getByTestId("bulk-modal")).toHaveAttribute(
      "data-student-count",
      "2",
    );
    fireEvent.click(screen.getByTestId("bulk-close"));
    expect(screen.queryByTestId("bulk-modal")).not.toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("Aktionen für 3a"));
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Klassenfahrt planen" }),
    );
    expect(screen.getByTestId("class-trip-selection-modal")).toHaveAttribute(
      "data-student-ids",
      "1,2",
    );
  });

  it("invokes onArrivalDataChanged when bulk modal succeeds", () => {
    const onArrivalDataChanged = vi.fn();
    render(
      <StudentsList
        {...baseProps}
        onArrivalDataChanged={onArrivalDataChanged}
        students={[makeStudent("1", { school_class: "3a" })]}
        grouping="class"
        studentsWithArrival={new Set(["1"])}
      />,
    );

    fireEvent.click(screen.getByLabelText("Aktionen für 3a"));
    fireEvent.click(
      screen.getByRole("menuitem", { name: /Ankunftszeit bearbeiten/ }),
    );
    fireEvent.click(screen.getByTestId("bulk-success"));

    expect(onArrivalDataChanged).toHaveBeenCalled();
  });

  it("uses the controlled selection for bulk actions and selection controls", () => {
    const onClearSelection = vi.fn();
    const onFinishSelection = vi.fn();
    render(
      <StudentsList
        {...baseProps}
        students={[makeStudent("1"), makeStudent("2"), makeStudent("3")]}
        grouping="none"
        studentsWithArrival={new Set(["1", "2", "3"])}
        selectionMode
        selectedStudentIds={new Set(["1", "3"])}
        onClearSelection={onClearSelection}
        onFinishSelection={onFinishSelection}
      />,
    );

    expect(screen.getByText("2 ausgewählt")).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Auswahl" })).toBeInTheDocument();
    expect(screen.getByTestId("student-1")).toHaveAttribute(
      "data-selection-mode",
      "true",
    );
    expect(screen.getByTestId("student-1")).toHaveAttribute(
      "data-checked",
      "true",
    );
    fireEvent.click(screen.getByRole("button", { name: "Aufheben" }));
    fireEvent.click(screen.getByRole("button", { name: "Fertig" }));
    expect(onClearSelection).toHaveBeenCalledOnce();
    expect(onFinishSelection).toHaveBeenCalledOnce();

    fireEvent.click(screen.getByRole("button", { name: "Ankunftszeiten" }));
    expect(screen.getByTestId("bulk-modal")).toHaveAttribute(
      "data-filter-value",
      "1,3",
    );
    fireEvent.click(screen.getByTestId("bulk-close"));

    fireEvent.click(screen.getByRole("button", { name: "Gehzeiten" }));
    expect(screen.getByTestId("pickup-selection-modal")).toHaveAttribute(
      "data-student-ids",
      "1,3",
    );
    fireEvent.click(screen.getByRole("button", { name: "close pickup" }));

    fireEvent.click(screen.getByRole("button", { name: "Klassenfahrt" }));
    expect(screen.getByTestId("class-trip-selection-modal")).toHaveAttribute(
      "data-student-ids",
      "1,3",
    );
  });

  describe("Betreuung beenden (#2487)", () => {
    const selectionProps = {
      ...baseProps,
      grouping: "class" as const,
      studentsWithArrival: new Set<string>(["1", "2", "3"]),
    };

    it("selects exactly the children currently shown", () => {
      const onSelectAllVisible = vi.fn();
      render(
        <StudentsList
          {...selectionProps}
          students={[makeStudent("1"), makeStudent("2")]}
          selectionMode
          selectedStudentIds={new Set()}
          onSelectAllVisible={onSelectAllVisible}
        />,
      );

      fireEvent.click(screen.getByRole("button", { name: "Alle 2 auswählen" }));
      expect(onSelectAllVisible).toHaveBeenCalledWith(["1", "2"]);
    });

    it("offers 'Betreuung beenden' only with the delete permission", () => {
      const { rerender } = render(
        <StudentsList
          {...selectionProps}
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
        <StudentsList
          {...selectionProps}
          students={[makeStudent("1")]}
          selectionMode
          selectedStudentIds={new Set(["1"])}
          onEndCare={onEndCare}
        />,
      );
      fireEvent.click(
        screen.getByRole("button", { name: "Betreuung beenden" }),
      );
      expect(onEndCare).toHaveBeenCalled();
    });

    it("disables the bulk action while nothing is selected", () => {
      render(
        <StudentsList
          {...selectionProps}
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
        <StudentsList
          {...selectionProps}
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
      render(
        <StudentsList
          {...selectionProps}
          students={[
            makeStudent("1", { care_ends_on: "2027-07-31", care_ended: false }),
          ]}
        />,
      );
      expect(
        screen.getByTestId("subtitle-First1 Last1").textContent,
      ).not.toContain("Betreuung endet");
    });
  });
});

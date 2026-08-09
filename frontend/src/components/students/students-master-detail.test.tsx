import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { Student } from "~/lib/student-helpers";

// ─────────────────────────────────────────────────────────────────────────────
// Mocks
// ─────────────────────────────────────────────────────────────────────────────

vi.mock("~/components/database/master-detail-layout", () => ({
  MasterDetailLayout: (props: {
    list: React.ReactNode;
    detail: React.ReactNode;
    onDeselect: () => void;
    selectedId: string | null;
  }) => (
    <div>
      <button type="button" data-testid="deselect" onClick={props.onDeselect}>
        deselect
      </button>
      <div data-testid="list" data-selected={props.selectedId ?? "none"}>
        {props.list}
      </div>
      <div data-testid="detail">{props.detail}</div>
    </div>
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

vi.mock("~/components/database/detail-panel", () => ({
  DetailPanel: (props: {
    header: React.ReactNode;
    tabs: { id: string; label: string; content: React.ReactNode }[];
    activeTab: string;
    onTabChange: (id: string) => void;
  }) => (
    <div>
      <div data-testid="detail-header">{props.header}</div>
      <div>
        {props.tabs.map((tab) => (
          <button
            type="button"
            key={tab.id}
            data-testid={`tab-trigger-${tab.id}`}
            onClick={() => props.onTabChange(tab.id)}
          >
            {tab.label}
          </button>
        ))}
      </div>
      {props.tabs.map((tab) =>
        tab.id === props.activeTab ? (
          <div key={tab.id} data-testid={`tab-content-${tab.id}`}>
            {tab.content}
          </div>
        ) : null,
      )}
    </div>
  ),
}));

vi.mock("~/components/database/empty-detail-state", () => ({
  EmptyDetailState: (props: { title?: string; description?: string }) => (
    <div data-testid="empty-detail">{props.title}</div>
  ),
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

vi.mock("./arrival-schedule-manager", () => ({
  ArrivalScheduleManager: (props: { studentId: string }) => (
    <div data-testid="arrival-manager">{props.studentId}</div>
  ),
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

vi.mock("./student-abholung-tab", () => ({
  StudentAbholungTab: (props: { studentId: string }) => (
    <div data-testid="abholung-tab">{props.studentId}</div>
  ),
}));

vi.mock("~/components/database/database-detail-header", () => ({
  DatabaseDetailHeader: (props: {
    avatar: string;
    title: string;
    subtitle: string;
    warning?: string | null;
    actions?: React.ReactNode;
  }) => (
    <div data-testid="detail-header-inner" data-warning={props.warning ?? ""}>
      {props.title}
      {props.actions}
    </div>
  ),
}));

vi.mock("./student-guardians-tab", () => ({
  StudentGuardiansTab: () => <div data-testid="guardians-tab" />,
}));

vi.mock("./student-historie-tab", () => ({
  StudentHistorieTab: () => <div data-testid="historie-tab" />,
}));

vi.mock("./student-enrollments-tab", () => ({
  StudentEnrollmentsTab: () => <div data-testid="enrollments-tab" />,
}));

vi.mock("~/components/database/database-list-item", () => ({
  DatabaseListItem: (props: {
    title: string;
    subtitle: React.ReactNode;
    isSelected: boolean;
    onSelect: () => void;
    trailingAccessory?: React.ReactNode;
  }) => {
    // Test fixtures use names like "First1 Last1"; derive the id back so
    // assertions written against the pre-migration `student-1` testid keep
    // working without changing the assertion shape.
    const idMatch = /First(\S+)/.exec(props.title);
    const idSlug = idMatch ? idMatch[1] : props.title;
    return (
      <button
        type="button"
        data-testid={`student-${idSlug}`}
        data-selected={props.isSelected}
        data-trailing={props.trailingAccessory ? "true" : ""}
        onClick={props.onSelect}
      >
        {props.title}
        <span data-testid="list-item-subtitle">{props.subtitle}</span>
      </button>
    );
  },
}));

vi.mock("./student-stammdaten-tab", () => ({
  StudentStammdatenTab: (props: {
    student: Student;
    onSave: (data: Partial<Student>) => Promise<void>;
  }) => (
    <button
      type="button"
      data-testid="stammdaten-tab"
      onClick={() => void props.onSave({ first_name: "Updated" })}
    >
      stammdaten
    </button>
  ),
}));

import { StudentsMasterDetail } from "./students-master-detail";

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

describe("StudentsMasterDetail", () => {
  const baseProps = {
    selectedId: null,
    onSelect: vi.fn(),
    onArrivalDataChanged: vi.fn(),
    groups: [] as Array<{ value: string; label: string }>,
    onUpdateStudent: vi.fn(),
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows empty state when students list is empty", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[]}
        grouping="class"
        studentsWithArrival={new Set()}
        arrivalSummaryById={new Map()}
      />,
    );

    expect(screen.getByTestId("groups-empty")).toBeInTheDocument();
    expect(screen.getByText("Keine Kinder gefunden.")).toBeInTheDocument();
  });

  it("keeps the student list constrained to the scrollable master pane", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1"), makeStudent("2")]}
        grouping="class"
        studentsWithArrival={new Set(["1", "2"])}
        arrivalSummaryById={new Map()}
      />,
    );

    expect(screen.getByTestId("list").firstElementChild).toHaveClass(
      "flex",
      "min-h-0",
      "flex-1",
      "flex-col",
    );
  });

  it("renders a flat single-group list when grouping is 'none'", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1"), makeStudent("2")]}
        grouping="none"
        studentsWithArrival={new Set(["1", "2"])}
        arrivalSummaryById={new Map()}
      />,
    );

    expect(screen.getByTestId("group-title-__flat__")).toHaveTextContent(
      "Alle Kinder",
    );
  });

  it("groups by class and shows warning variant when some miss arrival", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[
          makeStudent("1", { school_class: "3a" }),
          makeStudent("2", { school_class: "3a" }),
          makeStudent("3", { school_class: "3b" }),
        ]}
        grouping="class"
        studentsWithArrival={new Set(["1"])}
        arrivalSummaryById={new Map()}
      />,
    );

    expect(screen.getByTestId("group-title-3a")).toHaveTextContent("3a");
    expect(screen.getByTestId("group-variant-3a")).toHaveTextContent("warning");
    expect(screen.getByTestId("group-suffix-3a")).toHaveTextContent(
      "· 1 offen",
    );
  });

  it("labels fallback group when student has no class", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1", { school_class: "" })]}
        grouping="class"
        studentsWithArrival={new Set(["1"])}
        arrivalSummaryById={new Map()}
      />,
    );

    expect(screen.getByTestId("group-title-Ohne Klasse")).toBeInTheDocument();
  });

  it("groups by group_name and falls back on missing name", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[
          makeStudent("1", { group_name: "Füchse" }),
          makeStudent("2", { group_name: "", group_id: undefined }),
        ]}
        grouping="group"
        studentsWithArrival={new Set(["1", "2"])}
        arrivalSummaryById={new Map()}
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
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1"), makeStudent("2")]}
        grouping="class"
        studentsWithArrival={new Set(["1"])}
        arrivalSummaryById={new Map([["1", "Mo-Fr 08:00"]])}
      />,
    );

    // With-arrival student: subtitle carries the formatted summary, no
    // trailing warning icon.
    const one = screen.getByTestId("student-1");
    expect(one).toHaveAttribute("data-trailing", "");
    expect(one).toHaveTextContent("Mo-Fr 08:00");

    // Without-arrival student: subtitle says "keine Ankunft" and the
    // trailing accessory (AlertCircle) is set.
    const two = screen.getByTestId("student-2");
    expect(two).toHaveAttribute("data-trailing", "true");
    expect(two).toHaveTextContent("keine Ankunft");
  });

  it("calls onSelect when a list item is clicked", () => {
    const onSelect = vi.fn();
    render(
      <StudentsMasterDetail
        {...baseProps}
        onSelect={onSelect}
        students={[makeStudent("1")]}
        grouping="class"
        studentsWithArrival={new Set()}
        arrivalSummaryById={new Map()}
      />,
    );

    fireEvent.click(screen.getByTestId("student-1"));
    expect(onSelect).toHaveBeenCalledWith("1");
  });

  it("calls onSelect(null) on deselect", () => {
    const onSelect = vi.fn();
    render(
      <StudentsMasterDetail
        {...baseProps}
        onSelect={onSelect}
        students={[makeStudent("1")]}
        grouping="class"
        studentsWithArrival={new Set()}
        arrivalSummaryById={new Map()}
      />,
    );

    fireEvent.click(screen.getByTestId("deselect"));
    expect(onSelect).toHaveBeenCalledWith(null);
  });

  it("renders empty detail when no student is selected", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1")]}
        grouping="class"
        studentsWithArrival={new Set()}
        arrivalSummaryById={new Map()}
      />,
    );

    expect(screen.getByTestId("empty-detail")).toHaveTextContent(
      "Kein Kind ausgewählt",
    );
  });

  it("renders detail panel with Stammdaten tab active by default", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        selectedId="1"
        students={[makeStudent("1")]}
        grouping="class"
        studentsWithArrival={new Set(["1"])}
        arrivalSummaryById={new Map()}
      />,
    );

    expect(screen.getByTestId("detail-header-inner")).toHaveAttribute(
      "data-warning",
      "",
    );
    expect(screen.getByTestId("tab-content-master-data")).toBeInTheDocument();
  });

  it("shows Anmeldungen tab only when allowed", () => {
    const { rerender } = render(
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1")]}
        selectedId="1"
        grouping="none"
        studentsWithArrival={new Set(["1"])}
        arrivalSummaryById={new Map()}
      />,
    );

    expect(screen.queryByText("Anmeldungen")).not.toBeInTheDocument();

    rerender(
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1")]}
        selectedId="1"
        grouping="none"
        studentsWithArrival={new Set(["1"])}
        arrivalSummaryById={new Map()}
        canViewEnrollments
      />,
    );

    expect(screen.getByText("Anmeldungen")).toBeInTheDocument();
  });

  it("shows warning in header when student has no arrival", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        selectedId="1"
        students={[makeStudent("1")]}
        grouping="class"
        studentsWithArrival={new Set()}
        arrivalSummaryById={new Map()}
      />,
    );

    expect(screen.getByTestId("detail-header-inner")).toHaveAttribute(
      "data-warning",
      "Ankunft offen",
    );
  });

  it("switches tab when a tab trigger is clicked", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        selectedId="1"
        students={[makeStudent("1")]}
        grouping="class"
        studentsWithArrival={new Set()}
        arrivalSummaryById={new Map()}
      />,
    );

    fireEvent.click(screen.getByTestId("tab-trigger-betreuungszeiten"));
    expect(screen.getByTestId("arrival-manager")).toBeInTheDocument();
    expect(screen.getByTestId("abholung-tab")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("tab-trigger-history"));
    expect(screen.getByTestId("historie-tab")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("tab-trigger-guardians"));
    expect(screen.getByTestId("guardians-tab")).toBeInTheDocument();
  });

  it("propagates Stammdaten onSave to onUpdateStudent", () => {
    const onUpdateStudent = vi.fn();
    render(
      <StudentsMasterDetail
        {...baseProps}
        onUpdateStudent={onUpdateStudent}
        selectedId="1"
        students={[makeStudent("1")]}
        grouping="class"
        studentsWithArrival={new Set(["1"])}
        arrivalSummaryById={new Map()}
      />,
    );

    fireEvent.click(screen.getByTestId("stammdaten-tab"));
    expect(onUpdateStudent).toHaveBeenCalledWith("1", {
      first_name: "Updated",
    });
  });

  it("renders class actions menu only for known class groups", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[
          makeStudent("1", { school_class: "3a" }),
          makeStudent("2", { school_class: "" }),
        ]}
        grouping="class"
        studentsWithArrival={new Set(["1", "2"])}
        arrivalSummaryById={new Map()}
      />,
    );

    expect(screen.getByLabelText("Aktionen für 3a")).toBeInTheDocument();
    expect(
      screen.queryByLabelText("Aktionen für Klasse Ohne Klasse"),
    ).not.toBeInTheDocument();
  });

  it("renders arrival actions when grouping by a known group", () => {
    render(
      <StudentsMasterDetail
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
        arrivalSummaryById={new Map()}
      />,
    );

    expect(screen.getByLabelText("Aktionen für Füchse")).toBeInTheDocument();
  });

  it("opens bulk arrival modal from class actions menu", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1", { school_class: "3a" })]}
        grouping="class"
        studentsWithArrival={new Set(["1"])}
        arrivalSummaryById={new Map()}
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
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1", { group_id: "17", group_name: "Füchse" })]}
        grouping="group"
        studentsWithArrival={new Set(["1"])}
        arrivalSummaryById={new Map()}
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

  it("uses the unfiltered cohort for the bulk preview", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1", { school_class: "3a" })]}
        bulkStudents={[
          makeStudent("1", { school_class: "3a" }),
          makeStudent("2", { school_class: "3a" }),
        ]}
        grouping="class"
        studentsWithArrival={new Set(["1"])}
        arrivalSummaryById={new Map()}
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
  });

  it("uses the unfiltered cohort for class-trip planning", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1", { school_class: "3a" })]}
        bulkStudents={[
          makeStudent("1", { school_class: "3a" }),
          makeStudent("2", { school_class: "3a" }),
        ]}
        grouping="class"
        studentsWithArrival={new Set(["1"])}
        arrivalSummaryById={new Map()}
      />,
    );

    fireEvent.click(screen.getByLabelText("Aktionen für 3a"));
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Klassenfahrt planen" }),
    );

    expect(screen.getByTestId("class-trip-selection-modal")).toHaveAttribute(
      "data-student-ids",
      "1,2",
    );
  });

  it("closes bulk modal via onClose", () => {
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1", { school_class: "3a" })]}
        grouping="class"
        studentsWithArrival={new Set(["1"])}
        arrivalSummaryById={new Map()}
      />,
    );

    fireEvent.click(screen.getByLabelText("Aktionen für 3a"));
    fireEvent.click(
      screen.getByRole("menuitem", { name: /Ankunftszeit bearbeiten/ }),
    );

    fireEvent.click(screen.getByTestId("bulk-close"));
    expect(screen.queryByTestId("bulk-modal")).not.toBeInTheDocument();
  });

  it("invokes onArrivalDataChanged when bulk modal succeeds", () => {
    const onArrivalDataChanged = vi.fn();
    render(
      <StudentsMasterDetail
        {...baseProps}
        onArrivalDataChanged={onArrivalDataChanged}
        students={[makeStudent("1", { school_class: "3a" })]}
        grouping="class"
        studentsWithArrival={new Set(["1"])}
        arrivalSummaryById={new Map()}
      />,
    );

    fireEvent.click(screen.getByLabelText("Aktionen für 3a"));
    fireEvent.click(
      screen.getByRole("menuitem", { name: /Ankunftszeit bearbeiten/ }),
    );
    fireEvent.click(screen.getByTestId("bulk-success"));

    expect(onArrivalDataChanged).toHaveBeenCalled();
  });

  it("actions menu closes on outside click", () => {
    render(
      <div>
        <div data-testid="outside">outside</div>
        <StudentsMasterDetail
          {...baseProps}
          students={[makeStudent("1", { school_class: "3a" })]}
          grouping="class"
          studentsWithArrival={new Set(["1"])}
          arrivalSummaryById={new Map()}
        />
      </div>,
    );

    fireEvent.click(screen.getByLabelText("Aktionen für 3a"));
    expect(screen.getByRole("menu")).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByTestId("outside"));
    expect(screen.queryByRole("menu")).not.toBeInTheDocument();
  });

  it("uses the controlled selection for bulk actions and selection controls", () => {
    const onClearSelection = vi.fn();
    const onFinishSelection = vi.fn();
    render(
      <StudentsMasterDetail
        {...baseProps}
        students={[makeStudent("1"), makeStudent("2"), makeStudent("3")]}
        grouping="none"
        studentsWithArrival={new Set(["1", "2", "3"])}
        arrivalSummaryById={new Map()}
        selectionMode
        selectedStudentIds={new Set(["1", "3"])}
        onClearSelection={onClearSelection}
        onFinishSelection={onFinishSelection}
      />,
    );

    expect(screen.getByText("2 ausgewählt")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Aufheben" }));
    fireEvent.click(screen.getByRole("button", { name: "Fertig" }));
    expect(onClearSelection).toHaveBeenCalledOnce();
    expect(onFinishSelection).toHaveBeenCalledOnce();

    fireEvent.click(screen.getByLabelText("Aktion für Auswahl"));
    expect(
      screen.getByRole("menuitem", { name: "Gehzeiten ändern" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Klassenfahrt planen" }),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Ankunftszeiten ändern" }),
    );
    expect(screen.getByTestId("bulk-modal")).toHaveAttribute(
      "data-filter-value",
      "1,3",
    );
    fireEvent.click(screen.getByTestId("bulk-success"));
    expect(screen.getByText("2 ausgewählt")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("bulk-close"));

    fireEvent.click(screen.getByLabelText("Aktion für Auswahl"));
    fireEvent.click(screen.getByRole("menuitem", { name: "Gehzeiten ändern" }));
    expect(screen.getByTestId("pickup-selection-modal")).toHaveAttribute(
      "data-student-ids",
      "1,3",
    );
    fireEvent.click(screen.getByRole("button", { name: "close pickup" }));

    fireEvent.click(screen.getByLabelText("Aktion für Auswahl"));
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Klassenfahrt planen" }),
    );
    expect(screen.getByTestId("class-trip-selection-modal")).toHaveAttribute(
      "data-student-ids",
      "1,3",
    );
  });
});

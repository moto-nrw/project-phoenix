import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import type { Student } from "~/lib/api";
import { StudentSelectionScope } from "./student-selection-scope";
import { StudentTable, classColumn, nameColumn } from "./student-table";

const mocks = vi.hoisted(() => ({
  attendanceWebEnabled: true,
  batch: vi.fn(),
}));

vi.mock("~/lib/tenant-context", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/tenant-context")>()),
  useAttendanceWebEnabled: () => mocks.attendanceWebEnabled,
}));

vi.mock("~/lib/student-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/student-api")>()),
  schoolCheckinStudentsBatch: mocks.batch,
}));

function student(id: string, first: string, location: string): Student {
  return {
    id,
    name: first,
    first_name: first,
    second_name: "Kaya",
    school_class: "3a",
    current_location: location,
  } as Student;
}

const STUDENTS = [
  student("1", "Mia", "Zuhause"),
  student("2", "Ben", "Zuhause"),
];

function renderScope(
  props: Partial<{ scopeKey: string; checkinAllowed: boolean }> = {},
) {
  const columns = [
    nameColumn<Student>((row) => `/students/${row.id}`, false),
    classColumn<Student>(),
  ];
  const ui = (scopeKey: string) => (
    <ToastProvider>
      <StudentSelectionScope
        visibleStudents={STUDENTS}
        scopeKey={scopeKey}
        checkinAllowed={props.checkinAllowed ?? true}
      >
        {(selection) => (
          <StudentTable
            rows={STUDENTS}
            columns={columns}
            hiddenColumns={new Set()}
            onOpen={vi.fn()}
            selection={selection}
          />
        )}
      </StudentSelectionScope>
    </ToastProvider>
  );
  const view = render(ui(props.scopeKey ?? "a"));
  return { ...view, rerenderScope: (key: string) => view.rerender(ui(key)) };
}

// The DataTable keeps its phone list in the DOM next to the table (CSS picks
// one), so row checkboxes exist twice in jsdom; the tests use the table.
function table() {
  return within(screen.getByTestId("data-table-table"));
}

describe("StudentSelectionScope (#3834)", () => {
  beforeEach(() => {
    mocks.attendanceWebEnabled = true;
    mocks.batch.mockReset();
  });

  it("shows the selection bar only once a child is marked", () => {
    renderScope();
    expect(
      screen.queryByRole("region", { name: "Ausgewählte Kinder" }),
    ).not.toBeInTheDocument();

    fireEvent.click(
      table().getByRole("checkbox", { name: "Mia Kaya auswählen" }),
    );

    expect(
      screen.getByRole("region", { name: "Ausgewählte Kinder" }),
    ).toHaveTextContent("1 ausgewählt");
  });

  it("checks in exactly the marked children", async () => {
    mocks.batch.mockResolvedValue({
      action: "in",
      results: [
        { studentId: "1", ok: true, changed: true },
        { studentId: "2", ok: true, changed: true },
      ],
      succeeded: 2,
      failed: 0,
    });
    renderScope();

    fireEvent.click(table().getByRole("checkbox", { name: "Alle auswählen" }));
    fireEvent.click(screen.getByRole("button", { name: "Anmelden" }));

    await waitFor(() => expect(mocks.batch).toHaveBeenCalledTimes(1));
    expect(mocks.batch).toHaveBeenCalledWith(["1", "2"], "in");
  });

  it("offers no Anmelden/Abmelden where the school records attendance elsewhere", () => {
    mocks.attendanceWebEnabled = false;
    renderScope();

    fireEvent.click(
      table().getByRole("checkbox", { name: "Ben Kaya auswählen" }),
    );

    expect(
      screen.queryByRole("button", { name: "Anmelden" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Exportieren" }),
    ).toBeInTheDocument();
  });

  it("offers no Anmelden/Abmelden when the page does not allow it", () => {
    renderScope({ checkinAllowed: false });

    fireEvent.click(
      table().getByRole("checkbox", { name: "Ben Kaya auswählen" }),
    );

    expect(
      screen.queryByRole("button", { name: "Abmelden" }),
    ).not.toBeInTheDocument();
  });

  it("drops the marks when the list on screen changes", () => {
    const { rerenderScope } = renderScope({ scopeKey: "a" });
    fireEvent.click(
      table().getByRole("checkbox", { name: "Mia Kaya auswählen" }),
    );
    expect(
      table().getByRole("checkbox", { name: "Mia Kaya auswählen" }),
    ).toBeChecked();

    rerenderScope("b");

    expect(
      table().getByRole("checkbox", { name: "Mia Kaya auswählen" }),
    ).not.toBeChecked();
    expect(
      screen.queryByRole("region", { name: "Ausgewählte Kinder" }),
    ).not.toBeInTheDocument();
  });
});

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import {
  afterAll,
  afterEach,
  beforeAll,
  describe,
  expect,
  it,
  vi,
} from "vitest";
import { ClassTripBulkStatusModal } from "./class-trip-bulk-status-modal";
import {
  bulkCreateStudentStatusDays,
  StudentStatusDayConflictError,
} from "~/lib/student-status-days-api";
import type { Student } from "~/lib/api";

// The factory is hoisted above the imports, so the mock helper has to be
// pulled in inside it rather than at the top of the file.
vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

vi.mock("~/components/ui/form-modal", () => ({
  FormModal: ({
    isOpen,
    title,
    children,
    footer,
    onClose,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
    onClose: () => void;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        <h2>{title}</h2>
        <button type="button" onClick={onClose}>
          Modal schließen
        </button>
        {children}
        <div>{footer}</div>
      </div>
    ) : null,
}));

const toastError = vi.fn();

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: vi.fn(),
    error: toastError,
  }),
}));

vi.mock("~/lib/student-status-days-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/student-status-days-api")>();
  return {
    ...actual,
    bulkCreateStudentStatusDays: vi.fn(),
  };
});

const students = [
  {
    id: "42",
    name: "Kevin Anders",
    first_name: "Kevin",
    second_name: "Anders",
    school_class: "3a",
    current_location: "Zuhause",
  },
] satisfies Student[];

describe("ClassTripBulkStatusModal", () => {
  const originalTZ = process.env.TZ;

  beforeAll(() => {
    process.env.TZ = "Europe/Berlin";
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  afterAll(() => {
    process.env.TZ = originalTZ;
  });

  it("defaults the bulk range to the local Berlin date after midnight", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-05-26T22:30:00Z"));
    vi.mocked(bulkCreateStudentStatusDays).mockResolvedValue({
      student_count: 1,
      date_count: 1,
    });

    render(
      <ClassTripBulkStatusModal
        isOpen
        onClose={vi.fn()}
        targetLabel="Klasse 3a"
        students={students}
      />,
    );

    expect(screen.getByLabelText("Von")).toHaveValue("2026-05-27");
    expect(screen.getByLabelText("Bis")).toHaveValue("2026-05-27");

    vi.useRealTimers();

    fireEvent.click(
      screen.getByRole("button", { name: "Für 1 Schüler speichern" }),
    );

    await waitFor(() => {
      expect(bulkCreateStudentStatusDays).toHaveBeenCalledWith(
        ["42"],
        "class_trip",
        "2026-05-27",
        "2026-05-27",
        undefined,
      );
    });
  });

  it("lists each conflicting student, date, and status from a bulk 409", async () => {
    vi.mocked(bulkCreateStudentStatusDays).mockRejectedValueOnce(
      new StudentStatusDayConflictError([
        {
          id: "7",
          student_id: "42",
          date: "2026-05-26",
          status: "sick",
          label: "Krank",
          reported_at: "2026-05-25T08:00:00Z",
          cleared_at: null,
          source: "planned",
          created_at: "2026-05-25T08:00:00Z",
          updated_at: "2026-05-25T08:00:00Z",
        },
      ]),
    );

    render(
      <ClassTripBulkStatusModal
        isOpen
        onClose={vi.fn()}
        targetLabel="Klasse 3a"
        students={students}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Für 1 Schüler speichern" }),
    );

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Kevin Anders: 26.05.2026 (krank)",
      );
    });
    expect(toastError).toHaveBeenCalledWith(
      "Bestehende Status-Tage verhindern die Speicherung. Es wurde nichts überschrieben.",
    );
  });

  it("caps bulk conflict details and falls back to student.name", async () => {
    const manyConflicts = Array.from({ length: 12 }, (_, index) => ({
      id: String(index + 1),
      student_id: "99",
      date: `2026-05-${String(index + 1).padStart(2, "0")}`,
      status: "sick" as const,
      label: "Krank",
      reported_at: "2026-05-01T08:00:00Z",
      cleared_at: null,
      source: "planned",
      created_at: "2026-05-01T08:00:00Z",
      updated_at: "2026-05-01T08:00:00Z",
    }));
    vi.mocked(bulkCreateStudentStatusDays).mockRejectedValueOnce(
      new StudentStatusDayConflictError(manyConflicts),
    );

    render(
      <ClassTripBulkStatusModal
        isOpen
        onClose={vi.fn()}
        targetLabel="Klasse 3a"
        students={[
          {
            id: "99",
            name: "Nur Name",
            school_class: "3a",
            current_location: "Zuhause",
          } as Student,
        ]}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Für 1 Schüler speichern" }),
    );

    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent("12 Konflikte");
      expect(alert).toHaveTextContent("Nur Name: 01.05.2026 (krank)");
      expect(alert).toHaveTextContent("und 4 weitere");
      expect(alert).not.toHaveTextContent("undefined");
      expect(alert).not.toHaveTextContent("13.05.2026");
    });
  });
});

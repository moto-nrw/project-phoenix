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
import { releaseFakeTimers } from "~/test/clock";
import { ApiError } from "~/lib/api-error";
import { ClassTripBulkStatusModal } from "./class-trip-bulk-status-modal";
import {
  bulkCreateStudentStatusDays,
  StudentStatusDayConflictError,
} from "~/lib/student-status-days-api";
import type { Student } from "~/lib/api";

// The factory is hoisted above the imports, so the mock helper has to be
// pulled in inside it rather than at the top of the file.
vi.mock("~/components/ui/date-picker", async () =>
  (await import("~/test/mocks/date-picker")).datePickerModuleMock(),
);

vi.mock("~/components/ui/form-modal", () => ({
  FormModal: ({
    isOpen,
    title,
    children,
    footer,
    onClose,
    error,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
    onClose: () => void;
    error?: string | { message: string } | null;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        <h2>{title}</h2>
        <button type="button" onClick={onClose}>
          Modal schließen
        </button>
        {error ? (
          <div role="alert">
            {typeof error === "string" ? error : error.message}
          </div>
        ) : null}
        {children}
        <div>{footer}</div>
      </div>
    ) : null,
}));

const toastError = vi.fn();

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
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

    releaseFakeTimers();

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

    // Two alerts: the form error at the top of the modal says nothing was
    // written, the warning below lists the conflicting days.
    await waitFor(() => {
      const alerts = screen.getAllByRole("alert");
      expect(alerts[0]).toHaveTextContent(
        "Die Klassenfahrt konnte nicht geändert werden. Bitte prüfen Sie den aktuellen Stand.",
      );
      expect(alerts[1]).toHaveTextContent("Kevin Anders: 26.05.2026 (krank)");
    });
    expect(toastError).not.toHaveBeenCalled();
  });

  it("caps bulk conflict details and falls back to student.name", async () => {
    const manyConflicts = Array.from({ length: 8 }, (_, index) => ({
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
    // Backend may return a capped sample with a higher total count.
    vi.mocked(bulkCreateStudentStatusDays).mockRejectedValueOnce(
      new StudentStatusDayConflictError(manyConflicts, 48),
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
      // The conflict list is the warning below the form error.
      const alert = screen.getAllByRole("alert").at(-1)!;
      expect(alert).toHaveTextContent("48 Konflikte");
      expect(alert).toHaveTextContent("Nur Name: 01.05.2026 (krank)");
      expect(alert).toHaveTextContent("und 40 weitere");
      expect(alert).not.toHaveTextContent("undefined");
      expect(alert).not.toHaveTextContent("09.05.2026");
    });
  });

  it("names the class trip in a failed save without the server sentence", async () => {
    vi.mocked(bulkCreateStudentStatusDays).mockRejectedValueOnce(
      new ApiError("db down", 500, { code: "general.server" }),
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

    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Die Klassenfahrt konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
      ),
    );
    expect(screen.queryByText(/db down/)).not.toBeInTheDocument();
    expect(toastError).not.toHaveBeenCalled();
  });
});

describe("ClassTripBulkStatusModal in moto schule (#4799)", () => {
  const classChildren = [
    {
      id: "11",
      name: "Anna Ausflug",
      first_name: "Anna",
      second_name: "Ausflug",
    },
    {
      id: "12",
      name: "Ben Ausflug",
      first_name: "Ben",
      second_name: "Ausflug",
    },
  ];

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("preselects the whole class, lets the teacher deselect a child and saves through the school seam", async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    const onSuccess = vi.fn();
    const onClose = vi.fn();
    render(
      <ClassTripBulkStatusModal
        isOpen
        onClose={onClose}
        onSuccess={onSuccess}
        targetLabel="Klasse 3b"
        students={classChildren}
        schoolReport={{ save }}
      />,
    );

    expect(
      screen.getByRole("dialog", {
        name: "Ausflug oder Klassenfahrt melden: Klasse 3b",
      }),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Anna Ausflug")).toBeChecked();
    expect(screen.getByLabelText("Ben Ausflug")).toBeChecked();

    fireEvent.click(screen.getByLabelText("Ben Ausflug"));
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-09-11" },
    });
    // A trip over several days has no return time.
    expect(screen.queryByLabelText(/Rückkehrzeit/)).not.toBeInTheDocument();
    expect(screen.getByLabelText("Von")).toBeInTheDocument();
    fireEvent.change(screen.getByPlaceholderText("z. B. Zoo"), {
      target: { value: "Jugendherberge" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Für 1 Kind melden" }));

    await waitFor(() => {
      expect(save).toHaveBeenCalledWith({
        studentIds: ["11"],
        from: "2026-09-09",
        to: "2026-09-11",
        reason: "Jugendherberge",
        returnTime: undefined,
      });
    });
    expect(onSuccess).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
    expect(bulkCreateStudentStatusDays).not.toHaveBeenCalled();
  });

  it("offers a return time for one day and sends it", async () => {
    const save = vi.fn().mockResolvedValue(undefined);
    render(
      <ClassTripBulkStatusModal
        isOpen
        onClose={vi.fn()}
        targetLabel="Klasse 3b"
        students={classChildren}
        schoolReport={{ save }}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Rückkehrzeit/), {
      target: { value: "13:30" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Für 2 Kinder melden" }),
    );

    await waitFor(() => {
      expect(save).toHaveBeenCalledWith(
        expect.objectContaining({
          studentIds: ["11", "12"],
          from: "2026-09-09",
          to: "2026-09-09",
          returnTime: "13:30",
        }),
      );
    });
  });

  it("blocks the save without a child and shows a refusal in the form", async () => {
    const save = vi.fn().mockRejectedValue(
      new ApiError("conflict", 409, {
        code: "classday.class_trip_status_conflict",
      }),
    );
    const onClose = vi.fn();
    render(
      <ClassTripBulkStatusModal
        isOpen
        onClose={onClose}
        targetLabel="Klasse 3b"
        students={classChildren}
        schoolReport={{ save }}
      />,
    );

    fireEvent.click(screen.getByLabelText("Alle auswählen"));
    expect(
      screen.getByRole("button", { name: "Für 0 Kinder melden" }),
    ).toBeDisabled();

    fireEvent.click(screen.getByLabelText("Alle auswählen"));
    fireEvent.click(
      screen.getByRole("button", { name: "Für 2 Kinder melden" }),
    );
    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Für ein Kind steht an einem dieser Tage schon etwas",
      );
    });
    expect(onClose).not.toHaveBeenCalled();
  });
});

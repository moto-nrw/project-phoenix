import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { Student } from "~/lib/student-helpers";

const {
  mockFetchBulkArrivalScheduleStatus,
  mockFetchClassArrivalTimes,
  mockBulkUpsert,
  mockToastSuccess,
  mockToastError,
} = vi.hoisted(() => ({
  mockFetchBulkArrivalScheduleStatus: vi.fn(),
  mockFetchClassArrivalTimes: vi.fn(),
  mockBulkUpsert: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
}));

vi.mock("~/lib/student-arrival-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/student-arrival-api")
  >("~/lib/student-arrival-api");
  return {
    ...actual,
    fetchBulkArrivalScheduleStatus: mockFetchBulkArrivalScheduleStatus,
    fetchClassArrivalTimes: mockFetchClassArrivalTimes,
    bulkUpsertArrivalSchedules: mockBulkUpsert,
  };
});

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
    info: vi.fn(),
  }),
}));

vi.mock("~/components/ui/form-modal", () => ({
  FormModal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    footer: React.ReactNode;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        <h2>{title}</h2>
        <div>{children}</div>
        <div>{footer}</div>
      </div>
    ) : null,
}));

vi.mock("~/components/ui/button", () => ({
  Button: ({
    children,
    ...props
  }: React.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: string }) => (
    <button type="button" {...props}>
      {children}
    </button>
  ),
}));

import { FilteredBulkArrivalModal } from "./class-bulk-arrival-modal";

function makeStudent(id: string): Student {
  return {
    id,
    name: `S ${id}`,
    first_name: "S",
    second_name: id,
    school_class: "3a",
    current_location: "class",
  } as Student;
}

describe("FilteredBulkArrivalModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchBulkArrivalScheduleStatus.mockResolvedValue(0);
    mockFetchClassArrivalTimes.mockResolvedValue({
      school_class: "3a",
      times: {},
    });
  });

  it("does not render when isOpen is false", () => {
    render(
      <FilteredBulkArrivalModal
        isOpen={false}
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("renders all 5 weekday inputs when open", () => {
    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1"), makeStudent("2")]}
      />,
    );

    expect(screen.getByLabelText("Montag")).toBeInTheDocument();
    expect(screen.getByLabelText("Freitag")).toBeInTheDocument();
  });

  it("shows collision warning when some students already have schedules", async () => {
    mockFetchBulkArrivalScheduleStatus.mockResolvedValueOnce(1);

    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1"), makeStudent("2")]}
      />,
    );

    await waitFor(() =>
      expect(
        screen.getByText(
          "1 Kind hat eigene Ankunftszeiten. Diese Zeiten bleiben bestehen.",
        ),
      ).toBeInTheDocument(),
    );
  });

  it("warns that group updates can replace own arrival times", async () => {
    mockFetchBulkArrivalScheduleStatus.mockResolvedValueOnce(1);

    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "group", groupId: "7" }}
        filterLabel="Sonnen"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    await waitFor(() =>
      expect(
        screen.getByText(
          "1 Kind hat eigene Ankunftszeiten. Die gewählten Zeiten können sie ersetzen.",
        ),
      ).toBeInTheDocument(),
    );
  });

  it("uses one bulk lookup for every selected child", async () => {
    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    await waitFor(() =>
      expect(mockFetchBulkArrivalScheduleStatus).toHaveBeenCalledWith(["1"]),
    );
    expect(mockFetchBulkArrivalScheduleStatus).toHaveBeenCalledTimes(1);
  });

  it("hides collision warning when no students have schedules", async () => {
    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    await waitFor(() =>
      expect(mockFetchBulkArrivalScheduleStatus).toHaveBeenCalled(),
    );
    expect(screen.queryByText(/Kinder haben bereits/)).not.toBeInTheDocument();
  });

  it("gracefully treats fetch errors as no-existing-schedule", async () => {
    mockFetchBulkArrivalScheduleStatus.mockRejectedValueOnce(new Error("boom"));

    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    await waitFor(() =>
      expect(mockFetchBulkArrivalScheduleStatus).toHaveBeenCalled(),
    );
    expect(screen.queryByText(/Kinder haben bereits/)).not.toBeInTheDocument();
  });

  it("disables submit until a valid time is entered", async () => {
    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    const submit = screen.getByRole("button", { name: "Speichern" });
    expect(submit).toBeDisabled();

    await waitFor(() => expect(screen.getByLabelText("Montag")).toBeEnabled());

    fireEvent.change(screen.getByLabelText("Montag"), {
      target: { value: "08:00" },
    });

    expect(submit).not.toBeDisabled();
  });

  it("shows 'nicht ändern' hint for empty rows", () => {
    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    // All 5 rows start empty
    expect(screen.getAllByText("nicht ändern")).toHaveLength(5);
  });

  it("submits a class filter with populated rows", async () => {
    mockBulkUpsert.mockResolvedValueOnce({});
    const onSuccess = vi.fn();
    const onClose = vi.fn();

    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={onClose}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1")]}
        onSuccess={onSuccess}
      />,
    );

    await waitFor(() => expect(screen.getByLabelText("Montag")).toBeEnabled());

    fireEvent.change(screen.getByLabelText("Montag"), {
      target: { value: "08:00" },
    });
    fireEvent.change(screen.getByLabelText("Mittwoch"), {
      target: { value: "09:30" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(mockBulkUpsert).toHaveBeenCalled());
    expect(mockBulkUpsert).toHaveBeenCalledWith(
      { type: "school_class", schoolClass: "3a" },
      [
        { weekday: 1, expected_arrival: "08:00" },
        { weekday: 3, expected_arrival: "09:30" },
      ],
    );
    expect(onSuccess).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
    expect(mockToastSuccess).toHaveBeenCalled();
  });

  it("submits a group filter and names the group in the dialog", async () => {
    mockBulkUpsert.mockResolvedValueOnce({});

    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "group", groupId: "17" }}
        filterLabel="Füchse"
        studentsInFilter={[makeStudent("1"), makeStudent("2")]}
      />,
    );

    expect(
      screen.getByRole("dialog", {
        name: "Ankunftszeiten für Gruppe Füchse",
      }),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Dienstag"), {
      target: { value: "09:15" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockBulkUpsert).toHaveBeenCalledWith(
        { type: "group", groupId: "17" },
        [{ weekday: 2, expected_arrival: "09:15" }],
      ),
    );
  });

  it("shows error toast when bulk upsert fails", async () => {
    mockBulkUpsert.mockRejectedValueOnce(new Error("Save failed"));

    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    await waitFor(() => expect(screen.getByLabelText("Montag")).toBeEnabled());

    fireEvent.change(screen.getByLabelText("Montag"), {
      target: { value: "08:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        expect.stringContaining("Save failed"),
      ),
    );
  });

  it("cancel button calls onClose", () => {
    const onClose = vi.fn();
    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={onClose}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(onClose).toHaveBeenCalled();
  });

  // The class timetable is what the dialog edits, so it has to open with what
  // is already there — otherwise a school retypes it blind every time (#2414).
  it("opens prefilled with the class's current times and names the last change", async () => {
    mockFetchClassArrivalTimes.mockResolvedValue({
      school_class: "3a",
      times: { mon: "11:45", wed: "13:30" },
      updated_at: "2026-08-20T09:00:00Z",
    });

    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    await waitFor(() => {
      expect(screen.getByLabelText("Montag")).toHaveValue("11:45");
    });
    expect(screen.getByLabelText("Mittwoch")).toHaveValue("13:30");
    expect(screen.getByLabelText("Dienstag")).toHaveValue("");
    expect(screen.getByText(/Zuletzt geändert am/)).toBeInTheDocument();
  });

  it("stays empty when the class carries no times yet", async () => {
    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    await waitFor(() => expect(mockFetchClassArrivalTimes).toHaveBeenCalled());
    expect(screen.getByLabelText("Montag")).toHaveValue("");
    expect(screen.queryByText(/Zuletzt geändert am/)).not.toBeInTheDocument();
  });

  it("keeps class inputs disabled until their current times arrive", async () => {
    let resolveTimes:
      | ((value: {
          school_class: string;
          times: Record<string, string>;
        }) => void)
      | undefined;
    mockFetchClassArrivalTimes.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveTimes = resolve;
        }),
    );

    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    expect(screen.getByLabelText("Montag")).toBeDisabled();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
    expect(
      screen.getByText("Klassenzeiten werden geladen."),
    ).toBeInTheDocument();

    resolveTimes?.({ school_class: "3a", times: { mon: "11:45" } });
    await waitFor(() => expect(screen.getByLabelText("Montag")).toBeEnabled());
    expect(screen.getByLabelText("Montag")).toHaveValue("11:45");
  });

  it("keeps loaded class times when the collision list refreshes", async () => {
    mockFetchClassArrivalTimes.mockResolvedValue({
      school_class: "3a",
      times: { mon: "11:45" },
    });

    const { rerender } = render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    await waitFor(() =>
      expect(screen.getByLabelText("Montag")).toHaveValue("11:45"),
    );

    rerender(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1"), makeStudent("2")]}
      />,
    );

    expect(screen.getByLabelText("Montag")).toHaveValue("11:45");
    expect(mockFetchClassArrivalTimes).toHaveBeenCalledTimes(1);
  });

  it("shows a retry action when class times cannot be loaded", async () => {
    mockFetchClassArrivalTimes
      .mockRejectedValueOnce(new Error("offline"))
      .mockResolvedValueOnce({
        school_class: "3a",
        times: { mon: "11:45" },
      });

    render(
      <FilteredBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        filter={{ type: "school_class", schoolClass: "3a" }}
        filterLabel="3a"
        studentsInFilter={[makeStudent("1")]}
      />,
    );

    expect(
      await screen.findByText(
        "Die Klassenzeiten konnten nicht geladen werden. Bitte versuchen Sie es noch einmal.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Montag")).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "Erneut laden" }));
    await waitFor(() =>
      expect(screen.getByLabelText("Montag")).toHaveValue("11:45"),
    );
    expect(mockFetchClassArrivalTimes).toHaveBeenCalledTimes(2);
  });
});

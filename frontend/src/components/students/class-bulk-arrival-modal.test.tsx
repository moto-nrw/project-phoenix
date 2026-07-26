import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { Student } from "~/lib/student-helpers";

const {
  mockFetchArrivalData,
  mockBulkUpsert,
  mockToastSuccess,
  mockToastError,
} = vi.hoisted(() => ({
  mockFetchArrivalData: vi.fn(),
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
    fetchArrivalData: mockFetchArrivalData,
    bulkUpsertArrivalByClass: mockBulkUpsert,
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

import { ClassBulkArrivalModal } from "./class-bulk-arrival-modal";

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

describe("ClassBulkArrivalModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchArrivalData.mockResolvedValue({
      schedules: [],
      exceptions: [],
      notes: [],
    });
  });

  it("does not render when isOpen is false", () => {
    render(
      <ClassBulkArrivalModal
        isOpen={false}
        onClose={vi.fn()}
        schoolClass="3a"
        studentsInClass={[makeStudent("1")]}
      />,
    );

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("renders all 5 weekday inputs when open", () => {
    render(
      <ClassBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        schoolClass="3a"
        studentsInClass={[makeStudent("1"), makeStudent("2")]}
      />,
    );

    expect(screen.getByLabelText("Montag")).toBeInTheDocument();
    expect(screen.getByLabelText("Freitag")).toBeInTheDocument();
  });

  it("shows collision warning when some students already have schedules", async () => {
    mockFetchArrivalData
      .mockResolvedValueOnce({
        schedules: [{ id: 1 }],
        exceptions: [],
        notes: [],
      })
      .mockResolvedValueOnce({
        schedules: [],
        exceptions: [],
        notes: [],
      });

    render(
      <ClassBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        schoolClass="3a"
        studentsInClass={[makeStudent("1"), makeStudent("2")]}
      />,
    );

    await waitFor(() =>
      expect(
        screen.getByText(/1 Kind hat bereits Ankunftszeiten/),
      ).toBeInTheDocument(),
    );
  });

  it("hides collision warning when no students have schedules", async () => {
    render(
      <ClassBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        schoolClass="3a"
        studentsInClass={[makeStudent("1")]}
      />,
    );

    await waitFor(() => expect(mockFetchArrivalData).toHaveBeenCalled());
    expect(screen.queryByText(/Kinder haben bereits/)).not.toBeInTheDocument();
  });

  it("gracefully treats fetch errors as no-existing-schedule", async () => {
    mockFetchArrivalData.mockRejectedValueOnce(new Error("boom"));

    render(
      <ClassBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        schoolClass="3a"
        studentsInClass={[makeStudent("1")]}
      />,
    );

    await waitFor(() => expect(mockFetchArrivalData).toHaveBeenCalled());
    expect(screen.queryByText(/Kinder haben bereits/)).not.toBeInTheDocument();
  });

  it("disables submit until a valid time is entered", () => {
    render(
      <ClassBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        schoolClass="3a"
        studentsInClass={[makeStudent("1")]}
      />,
    );

    const submit = screen.getByRole("button", { name: /Für 1 Kind setzen/ });
    expect(submit).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Montag"), {
      target: { value: "08:00" },
    });

    expect(submit).not.toBeDisabled();
  });

  it("shows 'nicht ändern' hint for empty rows", () => {
    render(
      <ClassBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        schoolClass="3a"
        studentsInClass={[makeStudent("1")]}
      />,
    );

    // All 5 rows start empty
    expect(screen.getAllByText("nicht ändern")).toHaveLength(5);
  });

  it("calls bulkUpsertArrivalByClass on submit with populated rows", async () => {
    mockBulkUpsert.mockResolvedValueOnce({});
    const onSuccess = vi.fn();
    const onClose = vi.fn();

    render(
      <ClassBulkArrivalModal
        isOpen={true}
        onClose={onClose}
        schoolClass="3a"
        studentsInClass={[makeStudent("1")]}
        onSuccess={onSuccess}
      />,
    );

    fireEvent.change(screen.getByLabelText("Montag"), {
      target: { value: "08:00" },
    });
    fireEvent.change(screen.getByLabelText("Mittwoch"), {
      target: { value: "09:30" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Für 1 Kind setzen/ }));

    await waitFor(() => expect(mockBulkUpsert).toHaveBeenCalled());
    expect(mockBulkUpsert).toHaveBeenCalledWith("3a", [
      { weekday: 1, expected_arrival: "08:00" },
      { weekday: 3, expected_arrival: "09:30" },
    ]);
    expect(onSuccess).toHaveBeenCalled();
    expect(onClose).toHaveBeenCalled();
    expect(mockToastSuccess).toHaveBeenCalled();
  });

  it("shows error toast when bulk upsert fails", async () => {
    mockBulkUpsert.mockRejectedValueOnce(new Error("Save failed"));

    render(
      <ClassBulkArrivalModal
        isOpen={true}
        onClose={vi.fn()}
        schoolClass="3a"
        studentsInClass={[makeStudent("1")]}
      />,
    );

    fireEvent.change(screen.getByLabelText("Montag"), {
      target: { value: "08:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Für 1 Kind setzen/ }));

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        expect.stringContaining("Save failed"),
      ),
    );
  });

  it("cancel button calls onClose", () => {
    const onClose = vi.fn();
    render(
      <ClassBulkArrivalModal
        isOpen={true}
        onClose={onClose}
        schoolClass="3a"
        studentsInClass={[makeStudent("1")]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(onClose).toHaveBeenCalled();
  });
});

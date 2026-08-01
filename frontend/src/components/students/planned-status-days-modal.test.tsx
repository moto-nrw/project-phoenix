import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { PlannedStatusDaysModal } from "./planned-status-days-modal";
import type { StudentStatusDay } from "~/lib/student-status-days-api";

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

vi.mock("~/components/ui/date-picker", async () => ({
  DatePicker: ({
    onChangeDates,
  }: {
    onChangeDates: (dates: Date[]) => void;
  }) => (
    <button
      type="button"
      onClick={() =>
        onChangeDates([
          new Date("2026-05-27T00:00:00"),
          new Date("2026-05-26T00:00:00"),
        ])
      }
    >
      DatePicker Auswahl
    </button>
  ),
  // The class-trip range fields moved from native date inputs to the kit
  // picker; this stub keeps them settable via fireEvent.change. Imported
  // inside the factory because vi.mock is hoisted above the imports.
  ...(await import("~/test/mocks/date-picker")).isoDatePickerMock(),
}));

const existingDays: StudentStatusDay[] = [
  {
    id: "1",
    student_id: "42",
    date: "2026-05-26",
    status: "excused",
    label: "Entschuldigt",
    reported_at: "2026-05-25T08:00:00Z",
    cleared_at: null,
    source: "planned",
    created_at: "2026-05-25T08:00:00Z",
    updated_at: "2026-05-25T08:00:00Z",
  },
  {
    id: "2",
    student_id: "42",
    date: "2026-05-29",
    status: "sick",
    label: "Krank",
    reported_at: "2026-05-25T08:00:00Z",
    cleared_at: null,
    source: "planned",
    created_at: "2026-05-25T08:00:00Z",
    updated_at: "2026-05-25T08:00:00Z",
  },
];

describe("PlannedStatusDaysModal", () => {
  const originalTZ = process.env.TZ;

  beforeAll(() => {
    process.env.TZ = "Europe/Berlin";
  });

  afterAll(() => {
    process.env.TZ = originalTZ;
  });

  it("preselects today and submits selected sick dates", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="sick"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    expect(
      screen.getByRole("dialog", { name: "Krankmeldung planen" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Kevin Anders")).toBeInTheDocument();

    fireEvent.click(screen.getByText("DatePicker Auswahl"));
    fireEvent.click(screen.getByRole("button", { name: "Krankmelden" }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(["2026-05-26", "2026-05-27"]);
    });
  });

  it("shows existing status labels and deletes one planned day", async () => {
    const onDeleteStatusDay = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={existingDays}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
        onDeleteStatusDay={onDeleteStatusDay}
      />,
    );

    expect(screen.getAllByText("Bereits entschuldigt").length).toBeGreaterThan(
      0,
    );
    expect(screen.getAllByText(/bereits/i).length).toBeGreaterThan(0);

    fireEvent.click(screen.getByRole("button", { name: "Entfernen" }));

    await waitFor(() => {
      expect(onDeleteStatusDay).toHaveBeenCalledWith("1");
    });
  });

  it("warns when only already planned days are selected", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={existingDays}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByText("DatePicker Auswahl"));

    expect(
      screen.getByText(/Dienstag, 26.05.2026 ist bereits entschuldigt./),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Entschuldigen" }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(["2026-05-27"]);
    });
  });

  it("removes selected date chips before submitting", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="sick"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByText("DatePicker Auswahl"));
    fireEvent.click(screen.getByRole("button", { name: "26.05.2026" }));
    fireEvent.click(screen.getByRole("button", { name: "Krankmelden" }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(["2026-05-27"]);
    });
  });

  it("does not close while submitting", () => {
    const onClose = vi.fn();

    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting
        existingDays={[]}
        onClose={onClose}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByText("Modal schließen"));

    expect(onClose).not.toHaveBeenCalled();
  });

  it("submits each class-trip calendar day across DST changes", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="class_trip"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-03-28" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-03-30" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Klassenfahrt speichern" }),
    );

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith([
        "2026-03-28",
        "2026-03-29",
        "2026-03-30",
      ]);
    });
  });
});

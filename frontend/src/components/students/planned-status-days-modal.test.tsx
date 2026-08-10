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
    <div>
      <button
        type="button"
        onClick={() => onChangeDates([new Date("2026-05-27T00:00:00")])}
      >
        Einzeltag auswählen
      </button>
      <button
        type="button"
        onClick={() =>
          onChangeDates([
            new Date("2026-05-28T00:00:00"),
            new Date("2026-05-26T00:00:00"),
          ])
        }
      >
        Nicht zusammenhängende Tage auswählen
      </button>
      <button
        type="button"
        onClick={() =>
          onChangeDates([
            new Date("2026-05-27T00:00:00"),
            new Date("2026-05-26T00:00:00"),
          ])
        }
      >
        Auswahl mit Konflikt
      </button>
    </div>
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

const loadNoExistingDays = () => Promise.resolve([] as StudentStatusDay[]);
const loadKnownExistingDays = (from: string, to: string) =>
  Promise.resolve(
    existingDays.filter((day) => day.date >= from && day.date <= to),
  );

async function clickEnabledButton(name: string): Promise<void> {
  const button = screen.getByRole("button", { name });
  await waitFor(() => expect(button).toBeEnabled());
  fireEvent.click(button);
}

describe("PlannedStatusDaysModal", () => {
  const originalTZ = process.env.TZ;

  beforeAll(() => {
    process.env.TZ = "Europe/Berlin";
  });

  afterAll(() => {
    process.env.TZ = originalTZ;
  });

  it("starts without an implicit day and submits one selected day", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="sick"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        onClose={vi.fn()}
        loadExistingDays={loadKnownExistingDays}
        onSubmit={onSubmit}
      />,
    );

    expect(
      screen.getByRole("dialog", { name: "Krankmeldung planen" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Kevin Anders")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Krankmelden" })).toBeDisabled();

    fireEvent.click(screen.getByText("Einzeltag auswählen"));
    await clickEnabledButton("Krankmelden");

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(["2026-05-27"]);
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
        loadExistingDays={loadKnownExistingDays}
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

  it("warns and skips already planned days in an individual selection", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={existingDays}
        onClose={vi.fn()}
        loadExistingDays={loadKnownExistingDays}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByText("Auswahl mit Konflikt"));

    expect(
      screen.getByText(/Dienstag, 26. Mai 2026 ist bereits entschuldigt./),
    ).toBeInTheDocument();
    await clickEnabledButton("Entschuldigen");

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
        loadExistingDays={loadNoExistingDays}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByText("Nicht zusammenhängende Tage auswählen"));
    fireEvent.click(screen.getByRole("button", { name: "26.05.2026" }));
    await clickEnabledButton("Krankmelden");

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(["2026-05-28"]);
    });
  });

  it("submits multiple non-contiguous individual days", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        onClose={vi.fn()}
        loadExistingDays={loadNoExistingDays}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByText("Nicht zusammenhängende Tage auswählen"));
    expect(screen.getByText("2 Tage ausgewählt")).toBeInTheDocument();
    await clickEnabledButton("Entschuldigen");

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(["2026-05-26", "2026-05-28"]);
    });
  });

  it("submits an inclusive sick-date range and shows its affected-day count", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="sick"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        onClose={vi.fn()}
        loadExistingDays={loadNoExistingDays}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zeitraum" }));
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-08-17" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-08-19" },
    });

    expect(
      screen.getByText("17.08.2026 bis 19.08.2026 · 3 Tage"),
    ).toBeInTheDocument();
    await clickEnabledButton("Krankmelden");

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith([
        "2026-08-17",
        "2026-08-18",
        "2026-08-19",
      ]);
    });
  });

  it("accepts a one-day range", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        onClose={vi.fn()}
        loadExistingDays={loadNoExistingDays}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zeitraum" }));
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-08-17" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-08-17" },
    });

    expect(screen.getByText("17.08.2026 · 1 Tag")).toBeInTheDocument();
    await clickEnabledButton("Entschuldigen");

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(["2026-08-17"]);
    });
  });

  it("blocks and explains an end date before the start date", () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="sick"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        onClose={vi.fn()}
        loadExistingDays={loadNoExistingDays}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zeitraum" }));
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-08-19" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-08-17" },
    });

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Das Bis-Datum darf nicht vor dem Von-Datum liegen.",
    );
    expect(screen.getByRole("button", { name: "Krankmelden" })).toBeDisabled();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("rejects a range longer than 366 days before expanding or loading it", () => {
    const loadExistingDays = vi.fn().mockResolvedValue([]);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="sick"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        onClose={vi.fn()}
        loadExistingDays={loadExistingDays}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zeitraum" }));
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-01-01" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2027-01-02" },
    });

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Ein Zeitraum darf höchstens 366 Tage umfassen.",
    );
    expect(screen.getByRole("button", { name: "Krankmelden" })).toBeDisabled();
    expect(loadExistingDays).not.toHaveBeenCalled();
  });

  it("keeps all input after a failed submission", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error("network failed"));
    const loadExistingDays = vi.fn().mockResolvedValue([]);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="sick"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        onClose={vi.fn()}
        loadExistingDays={loadExistingDays}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zeitraum" }));
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-08-17" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-08-19" },
    });
    fireEvent.change(screen.getByLabelText("Grund (optional)"), {
      target: { value: "Arzttermin" },
    });

    await clickEnabledButton("Krankmelden");

    await waitFor(() => expect(loadExistingDays).toHaveBeenCalledTimes(2));
    expect(onSubmit).toHaveBeenCalledWith(
      ["2026-08-17", "2026-08-18", "2026-08-19"],
      "Arzttermin",
    );
    expect(
      screen.getByText("17.08.2026 bis 19.08.2026 · 3 Tage"),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Grund (optional)")).toHaveValue("Arzttermin");
  });

  it("refreshes and shows a conflict that appears during submission", async () => {
    const raceConflict: StudentStatusDay = {
      ...existingDays[1]!,
      date: "2026-08-18",
      status: "sick",
    };
    const loadExistingDays = vi
      .fn()
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([raceConflict]);
    const onSubmit = vi.fn().mockRejectedValue(new Error("conflict"));

    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        onClose={vi.fn()}
        loadExistingDays={loadExistingDays}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zeitraum" }));
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-08-17" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-08-19" },
    });
    await clickEnabledButton("Entschuldigen");

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "18.08.2026 (krank): 1 von 3 Tagen hat bereits einen Status",
      );
    });
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("fails closed when existing status days cannot be checked", async () => {
    render(
      <PlannedStatusDaysModal
        isOpen
        status="sick"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        onClose={vi.fn()}
        loadExistingDays={() => Promise.reject(new Error("network failed"))}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByText("Einzeltag auswählen"));

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Vorhandene Status-Tage konnten nicht geprüft werden",
      );
    });
    expect(screen.getByRole("button", { name: "Krankmelden" })).toBeDisabled();
  });

  it("does not overwrite existing status days inside a range", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={existingDays}
        onClose={vi.fn()}
        loadExistingDays={loadKnownExistingDays}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zeitraum" }));
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-05-25" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-05-27" },
    });

    expect(screen.getByRole("alert")).toHaveTextContent(
      "26.05.2026 (entschuldigt): 1 von 3 Tagen hat bereits einen Status und wird nicht überschrieben. 2 Tage werden gespeichert.",
    );
    await clickEnabledButton("Entschuldigen");

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(["2026-05-25", "2026-05-27"]);
    });
  });

  it("clears a stale conflict after an existing status day is deleted", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    let knownDays = [...existingDays];
    const loadExistingDays = vi.fn((from: string, to: string) =>
      Promise.resolve(
        knownDays.filter((day) => day.date >= from && day.date <= to),
      ),
    );

    const { rerender } = render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={knownDays}
        onClose={vi.fn()}
        loadExistingDays={loadExistingDays}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zeitraum" }));
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-05-25" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-05-27" },
    });

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "26.05.2026 (entschuldigt): 1 von 3 Tagen hat bereits einen Status",
      );
    });
    expect(loadExistingDays).toHaveBeenCalled();

    knownDays = knownDays.filter((day) => day.id !== "1");
    loadExistingDays.mockClear();
    rerender(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={knownDays}
        onClose={vi.fn()}
        loadExistingDays={loadExistingDays}
        onSubmit={onSubmit}
      />,
    );

    await waitFor(() => {
      expect(loadExistingDays).toHaveBeenCalled();
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    await clickEnabledButton("Entschuldigen");
    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith([
        "2026-05-25",
        "2026-05-26",
        "2026-05-27",
      ]);
    });
  });

  it("checks the exact range and identifies an unseen cross-status conflict", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    const unseenConflict: StudentStatusDay = {
      ...existingDays[1]!,
      id: "3",
      date: "2027-01-02",
      status: "sick",
    };
    const loadExistingDays = vi.fn().mockResolvedValue([unseenConflict]);
    const onDeleteStatusDay = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        onClose={vi.fn()}
        loadExistingDays={loadExistingDays}
        onSubmit={onSubmit}
        onDeleteStatusDay={onDeleteStatusDay}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Zeitraum" }));
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2027-01-01" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2027-01-03" },
    });

    await waitFor(() => {
      expect(loadExistingDays).toHaveBeenLastCalledWith(
        "2027-01-01",
        "2027-01-03",
      );
      expect(screen.getByRole("alert")).toHaveTextContent(
        "02.01.2027 (krank): 1 von 3 Tagen hat bereits einen Status",
      );
    });

    expect(screen.getByText("Bereits vorhanden")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Entfernen" }));
    await waitFor(() => {
      expect(onDeleteStatusDay).toHaveBeenCalledWith("3");
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    // Out-of-window delete must unblock the date immediately without reopening.
    await clickEnabledButton("Entschuldigen");
    expect(onSubmit).toHaveBeenCalledWith([
      "2027-01-01",
      "2027-01-02",
      "2027-01-03",
    ]);
  });

  it("does not list intervening statuses for non-contiguous individual picks", async () => {
    // Selected mock dates are 26 + 28; a status only on 27 must not surface.
    const intervening: StudentStatusDay = {
      ...existingDays[0]!,
      id: "9",
      date: "2026-05-27",
      status: "sick",
    };
    const loadExistingDays = vi.fn().mockResolvedValue([intervening]);
    const onDeleteStatusDay = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        onClose={vi.fn()}
        loadExistingDays={loadExistingDays}
        onSubmit={vi.fn()}
        onDeleteStatusDay={onDeleteStatusDay}
      />,
    );

    fireEvent.click(screen.getByText("Nicht zusammenhängende Tage auswählen"));

    await waitFor(() => {
      expect(loadExistingDays).toHaveBeenCalledWith("2026-05-26", "2026-05-28");
    });

    expect(screen.queryByText("Bereits vorhanden")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Entfernen" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
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
        loadExistingDays={loadNoExistingDays}
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
        loadExistingDays={loadNoExistingDays}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-03-28" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-03-30" },
    });
    await clickEnabledButton("Klassenfahrt speichern");

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith([
        "2026-03-28",
        "2026-03-29",
        "2026-03-30",
      ]);
    });
  });
});

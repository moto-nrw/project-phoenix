import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import { PlannedStatusDaysModal } from "./planned-status-days-modal";
import type { StudentPartialAbsence } from "~/lib/student-partial-absences-api";
import type { StudentStatusDay } from "~/lib/student-status-days-api";

// Das Panel laeuft als SlideOver (Vaul). Vaul rendert in jsdom nicht, deshalb
// steht hier dieselbe Struktur ohne Animationsschicht.
vi.mock("~/components/ui/slide-over", () => ({
  SlideOver: ({
    open,
    onOpenChange,
    children,
  }: {
    open: boolean;
    onOpenChange: (open: boolean) => void;
    children: React.ReactNode;
  }) =>
    open ? (
      <div role="dialog">
        <button type="button" onClick={() => onOpenChange(false)}>
          Modal schließen
        </button>
        {children}
      </div>
    ) : null,
  SlideOverContent: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SlideOverHeader: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SlideOverFooter: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  SlideOverTitle: ({ children }: { children: React.ReactNode }) => (
    <h2>{children}</h2>
  ),
  SlideOverDescription: ({ children }: { children: React.ReactNode }) => (
    <p>{children}</p>
  ),
  SlideOverCloseButton: (
    props: React.ButtonHTMLAttributes<HTMLButtonElement>,
  ) => <button type="button" {...props} />,
}));

vi.mock("~/components/ui/date-picker", async () => ({
  DatePicker: (
    props:
      | {
          mode?: "single";
          onChange: (date: Date | null) => void;
        }
      | {
          mode: "multiple";
          onChangeDates: (dates: Date[]) => void;
        },
  ) => (
    <div data-testid="date-picker" data-mode={props.mode ?? "single"}>
      <button
        type="button"
        onClick={() => {
          const date = new Date("2026-05-27T00:00:00");
          if (props.mode === "multiple") {
            props.onChangeDates([date]);
          } else {
            props.onChange(date);
          }
        }}
      >
        Einzeltag auswählen
      </button>
      {props.mode === "multiple" ? (
        <>
          <button
            type="button"
            onClick={() =>
              props.onChangeDates([
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
              props.onChangeDates([
                new Date("2026-05-27T00:00:00"),
                new Date("2026-05-26T00:00:00"),
              ])
            }
          >
            Auswahl mit Konflikt
          </button>
        </>
      ) : null}
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

const existingPartialAbsence: StudentPartialAbsence = {
  id: "9",
  studentId: "42",
  date: "2026-05-27",
  fromTime: "13:30",
  auto: false,
  reason: "Arzttermin",
  pickupTime: "13:30",
  createdBy: "5",
  createdAt: "2026-05-20T08:00:00Z",
  updatedAt: "2026-05-20T08:00:00Z",
};

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

  it("submits a one-day excusal from a time and warns about an overlapping block", async () => {
    const onSubmitPartialAbsence = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        existingPartialAbsences={[]}
        onClose={vi.fn()}
        loadExistingDays={loadNoExistingDays}
        loadCarePlanDay={vi.fn().mockResolvedValue({
          studentId: "42",
          date: "2026-05-27",
          weekday: 3,
          arrival: { expectedTime: "08:00", source: "schedule" },
          pickup: { expectedTime: "16:00", source: "schedule" },
          instances: [
            {
              id: "11",
              title: "Lernzeit",
              startTime: "12:30",
              endTime: "14:00",
              roomId: "2",
              status: "planned",
              activeGroupId: null,
              attendance: {
                status: "expected",
                substatus: null,
                note: null,
                checkedInAt: null,
                checkedOutAt: null,
                isUnplanned: false,
              },
            },
          ],
        })}
        onSubmit={vi.fn()}
        onSubmitPartialAbsence={onSubmitPartialAbsence}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Ab Uhrzeit" }));
    expect(screen.getByTestId("date-picker")).toHaveAttribute(
      "data-mode",
      "single",
    );
    fireEvent.click(screen.getByText("Einzeltag auswählen"));
    fireEvent.change(screen.getByLabelText("Entschuldigt ab"), {
      target: { value: "13:30" },
    });

    expect(
      await screen.findByText(/Lernzeit.*überschneidet/i),
    ).toBeInTheDocument();
    await clickEnabledButton("Entschuldigen");
    expect(onSubmitPartialAbsence).toHaveBeenCalledWith(
      null,
      "2026-05-27",
      "13:30",
      undefined,
    );
  });

  it("still submits a partial excusal when the care-plan load fails", async () => {
    const onSubmitPartialAbsence = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        existingPartialAbsences={[]}
        onClose={vi.fn()}
        loadExistingDays={loadNoExistingDays}
        loadCarePlanDay={vi.fn().mockRejectedValue(new Error("forbidden"))}
        onSubmit={vi.fn()}
        onSubmitPartialAbsence={onSubmitPartialAbsence}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Ab Uhrzeit" }));
    fireEvent.click(screen.getByText("Einzeltag auswählen"));
    fireEvent.change(screen.getByLabelText("Entschuldigt ab"), {
      target: { value: "13:30" },
    });

    await clickEnabledButton("Entschuldigen");
    expect(onSubmitPartialAbsence).toHaveBeenCalledWith(
      null,
      "2026-05-27",
      "13:30",
      undefined,
    );
  });

  it("edits and deletes an existing partial excusal", async () => {
    const onSubmitPartialAbsence = vi.fn().mockResolvedValue(undefined);
    const onDeletePartialAbsence = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        existingPartialAbsences={[existingPartialAbsence]}
        onClose={vi.fn()}
        loadExistingDays={loadNoExistingDays}
        loadCarePlanDay={vi.fn().mockResolvedValue({
          studentId: "42",
          date: "2026-05-27",
          weekday: 3,
          arrival: { expectedTime: null, source: "none" },
          pickup: { expectedTime: "13:30", source: "exception" },
          instances: [],
        })}
        onSubmit={vi.fn()}
        onSubmitPartialAbsence={onSubmitPartialAbsence}
        onDeletePartialAbsence={onDeletePartialAbsence}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Ab Uhrzeit" }));
    fireEvent.click(
      screen.getByRole("button", {
        name: "Teilentschuldigung vom Mittwoch, 27. Mai 2026 entfernen",
      }),
    );
    expect(onDeletePartialAbsence).not.toHaveBeenCalled();
    fireEvent.click(
      screen.getByRole("button", { name: "Entfernen bestätigen" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Teilentschuldigung entfernen" }),
    );
    await waitFor(() =>
      expect(onDeletePartialAbsence).toHaveBeenCalledWith("9"),
    );
    fireEvent.click(
      screen.getByRole("button", {
        name: "Teilentschuldigung vom Mittwoch, 27. Mai 2026 bearbeiten",
      }),
    );
    expect(screen.getByLabelText("Entschuldigt ab")).toHaveValue("13:30");
    fireEvent.change(screen.getByLabelText("Entschuldigt ab"), {
      target: { value: "14:00" },
    });
    await clickEnabledButton("Entschuldigung aktualisieren");
    expect(onSubmitPartialAbsence).toHaveBeenCalledWith(
      "9",
      "2026-05-27",
      "14:00",
      "Arzttermin",
    );
  });

  it("keeps the edited date when another day is selected during edit", async () => {
    const onSubmitPartialAbsence = vi.fn().mockResolvedValue(undefined);

    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        existingPartialAbsences={[existingPartialAbsence]}
        onClose={vi.fn()}
        loadExistingDays={loadNoExistingDays}
        loadCarePlanDay={vi.fn().mockResolvedValue({
          studentId: "42",
          date: "2026-05-27",
          weekday: 3,
          arrival: { expectedTime: null, source: "none" },
          pickup: { expectedTime: "13:30", source: "exception" },
          instances: [],
        })}
        onSubmit={vi.fn()}
        onSubmitPartialAbsence={onSubmitPartialAbsence}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Ab Uhrzeit" }));
    fireEvent.click(
      screen.getByRole("button", {
        name: "Teilentschuldigung vom Mittwoch, 27. Mai 2026 bearbeiten",
      }),
    );
    expect(screen.getByLabelText("Entschuldigt ab")).toHaveValue("13:30");

    // Selecting another day must not drop edit mode into a create-on-new-date.
    fireEvent.click(screen.getByText("Einzeltag auswählen"));
    fireEvent.change(screen.getByLabelText("Entschuldigt ab"), {
      target: { value: "15:00" },
    });
    await clickEnabledButton("Entschuldigung aktualisieren");
    expect(onSubmitPartialAbsence).toHaveBeenCalledWith(
      "9",
      "2026-05-27",
      "15:00",
      "Arzttermin",
    );
  });

  it("keeps the delete confirmation open when partial excusal deletion fails", async () => {
    const onDeletePartialAbsence = vi
      .fn()
      .mockRejectedValue(new Error("delete failed"));

    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        existingPartialAbsences={[existingPartialAbsence]}
        onClose={vi.fn()}
        loadExistingDays={loadNoExistingDays}
        onSubmit={vi.fn()}
        onDeletePartialAbsence={onDeletePartialAbsence}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Ab Uhrzeit" }));
    fireEvent.click(
      screen.getByRole("button", {
        name: "Teilentschuldigung vom Mittwoch, 27. Mai 2026 entfernen",
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Entfernen bestätigen" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Teilentschuldigung entfernen" }),
    );

    expect(
      await screen.findByText(
        "Die Teilentschuldigung konnte nicht entfernt werden. Bitte erneut versuchen.",
      ),
    ).toBeInTheDocument();
    expect(onDeletePartialAbsence).toHaveBeenCalledWith("9");
    expect(
      screen.getByRole("heading", { name: "Teilentschuldigung entfernen?" }),
    ).toBeInTheDocument();
  });

  it("enters edit mode when a distant partial excusal loads after date selection", async () => {
    const onSubmitPartialAbsence = vi.fn().mockResolvedValue(undefined);
    let resolvePartialLookup!: (rows: StudentPartialAbsence[]) => void;
    const loadPartialAbsences = vi.fn(
      () =>
        new Promise<StudentPartialAbsence[]>((resolve) => {
          resolvePartialLookup = resolve;
        }),
    );
    const commonProps = {
      isOpen: true,
      status: "excused" as const,
      studentName: "Kevin Anders",
      isSubmitting: false,
      existingDays: [],
      onClose: vi.fn(),
      loadExistingDays: loadNoExistingDays,
      loadCarePlanDay: vi.fn().mockResolvedValue({
        studentId: "42",
        date: "2026-05-27",
        weekday: 3,
        arrival: { expectedTime: null, source: "none" as const },
        pickup: { expectedTime: "13:45", source: "exception" as const },
        instances: [],
      }),
      loadPartialAbsences,
      onSubmit: vi.fn(),
      onSubmitPartialAbsence,
    };
    render(
      <PlannedStatusDaysModal {...commonProps} existingPartialAbsences={[]} />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Ab Uhrzeit" }));
    fireEvent.click(screen.getByText("Einzeltag auswählen"));

    await waitFor(() => {
      expect(loadPartialAbsences).toHaveBeenCalledWith(
        "2026-05-27",
        "2026-05-27",
      );
    });
    expect(
      screen.getByRole("button", { name: "Entschuldigen" }),
    ).toBeDisabled();

    resolvePartialLookup([
      {
        id: "outside-window",
        studentId: "42",
        date: "2026-05-27",
        fromTime: "13:45",
        auto: false,
        reason: "Therapie",
        pickupTime: "13:45",
        createdBy: "5",
        createdAt: "2026-05-20T08:00:00Z",
        updatedAt: "2026-05-20T08:00:00Z",
      },
    ]);

    await waitFor(() => {
      expect(screen.getByLabelText("Entschuldigt ab")).toHaveValue("13:45");
      expect(screen.getByLabelText("Grund (optional)")).toHaveValue("Therapie");
    });
    await clickEnabledButton("Entschuldigung aktualisieren");
    expect(onSubmitPartialAbsence).toHaveBeenCalledWith(
      "outside-window",
      "2026-05-27",
      "13:45",
      "Therapie",
    );
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
      screen.getByRole("heading", { name: "Krankmeldung planen" }),
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

  it("keeps out-of-window conflicts when delete fails", async () => {
    const unseenConflict: StudentStatusDay = {
      ...existingDays[1]!,
      id: "3",
      date: "2027-01-02",
      status: "sick",
    };
    const loadExistingDays = vi.fn().mockResolvedValue([unseenConflict]);
    const onDeleteStatusDay = vi
      .fn()
      .mockRejectedValue(new Error("delete failed"));
    const onSubmit = vi.fn().mockResolvedValue(undefined);

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
      expect(screen.getByRole("alert")).toHaveTextContent(
        "02.01.2027 (krank): 1 von 3 Tagen hat bereits einen Status",
      );
    });

    fireEvent.click(screen.getByRole("button", { name: "Entfernen" }));
    await waitFor(() => {
      expect(onDeleteStatusDay).toHaveBeenCalledWith("3");
    });

    expect(screen.getByRole("alert")).toHaveTextContent(
      "02.01.2027 (krank): 1 von 3 Tagen hat bereits einen Status",
    );

    await clickEnabledButton("Entschuldigen");
    expect(onSubmit).toHaveBeenCalledWith(["2027-01-01", "2027-01-03"]);
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

  // A partial excusal is a pickup exception behind users:update + full care
  // access. Staff who only hold the absence permission of a school ohne feste
  // Gruppen must not be offered it (#2232).
  it("hides the scope switch when partial excusals are not permitted", () => {
    render(
      <PlannedStatusDaysModal
        isOpen
        status="excused"
        studentName="Kevin Anders"
        isSubmitting={false}
        existingDays={[]}
        canPlanPartialExcusal={false}
        onClose={vi.fn()}
        loadExistingDays={loadNoExistingDays}
        onSubmit={vi.fn()}
      />,
    );

    expect(
      screen.queryByRole("button", { name: "Ab Uhrzeit" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Ganzer Tag" }),
    ).not.toBeInTheDocument();
    // The full-day planning surface stays intact.
    expect(
      screen.getByRole("button", { name: "Einzelne Tage" }),
    ).toBeInTheDocument();
  });
});

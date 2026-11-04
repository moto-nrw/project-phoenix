import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { HolidayPeriod, TraegerSchool } from "~/lib/traeger-api";

const { mockCreate, mockUpdate } = vi.hoisted(() => ({
  mockCreate: vi.fn(),
  mockUpdate: vi.fn(),
}));

vi.mock("~/lib/traeger-api", () => ({
  createHolidayPeriod: (input: unknown) => mockCreate(input),
  updateHolidayPeriod: (id: string, input: unknown) => mockUpdate(id, input),
}));

import { HolidayPeriodForm } from "./holiday-period-form";

const schools: TraegerSchool[] = [
  {
    id: "31",
    name: "OGS Am Bach",
    slug: "am-bach",
    subdomain: "am-bach",
    city: "",
  },
  {
    id: "32",
    name: "OGS Lindenweg",
    slug: "linden",
    subdomain: "linden",
    city: "",
  },
];

const stored: HolidayPeriod = {
  id: "5",
  name: "Osterferien",
  registrationOpensOn: "2027-01-11",
  registrationClosesOn: "2027-02-26",
  binding: true,
  notes: "",
  status: "draft",
  publishedAt: null,
  publicToken: "token",
  registrationCount: 0,
  weeks: [
    {
      id: "7",
      startsOn: "2027-03-22",
      endsOn: "2027-03-26",
      hostSchoolId: "31",
      timeModels: [
        {
          id: "9",
          name: "bis 13 Uhr",
          startsAt: "08:00",
          endsAt: "13:00",
          priceCents: 4500,
        },
      ],
    },
  ],
};

describe("HolidayPeriodForm", () => {
  beforeEach(() => {
    mockCreate.mockReset();
    mockUpdate.mockReset();
  });

  it.each([false, true])(
    "reconciles a refreshed period only when the draft is pristine (edited=%s)",
    async (edited) => {
      const onSaved = vi.fn();
      const view = render(
        <ToastProvider>
          <HolidayPeriodForm
            period={stored}
            schools={schools}
            onSaved={onSaved}
            onCancel={vi.fn()}
          />
        </ToastProvider>,
      );
      if (edited)
        fireEvent.change(screen.getByLabelText("Name des Ferienzeitraums"), {
          target: { value: "Mein Entwurf" },
        });
      const refreshed = {
        ...stored,
        name: "Aktueller Servername",
        notes: "Neue Hinweise",
      };
      view.rerender(
        <ToastProvider>
          <HolidayPeriodForm
            period={refreshed}
            schools={schools}
            onSaved={onSaved}
            onCancel={vi.fn()}
          />
        </ToastProvider>,
      );
      expect(screen.getByLabelText("Name des Ferienzeitraums")).toHaveValue(
        edited ? "Mein Entwurf" : refreshed.name,
      );
      mockUpdate.mockResolvedValue(refreshed);
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
      await waitFor(() =>
        expect(mockUpdate).toHaveBeenCalledWith(
          stored.id,
          expect.objectContaining({
            name: edited ? "Mein Entwurf" : refreshed.name,
            notes: edited ? stored.notes : refreshed.notes,
          }),
        ),
      );
    },
  );

  it("marks the missing fields of a new period and sends nothing", async () => {
    render(
      <ToastProvider>
        <HolidayPeriodForm
          schools={schools}
          onSaved={vi.fn()}
          onCancel={vi.fn()}
        />
      </ToastProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Bitte prüfen Sie die markierten Felder."),
    ).toBeInTheDocument();
    expect(
      screen.getAllByText("Bitte geben Sie einen Namen ein."),
    ).toHaveLength(2);
    expect(
      screen.getByText("Bitte wählen Sie die ausrichtende OGS."),
    ).toBeInTheDocument();
    expect(mockCreate).not.toHaveBeenCalled();
  });

  it("saves a changed price in whole cent and keeps the IDs", async () => {
    const onSaved = vi.fn();
    mockUpdate.mockResolvedValue(stored);
    render(
      <ToastProvider>
        <HolidayPeriodForm
          period={stored}
          schools={schools}
          onSaved={onSaved}
          onCancel={vi.fn()}
        />
      </ToastProvider>,
    );

    fireEvent.change(screen.getByLabelText("Preis in Euro"), {
      target: { value: "50,25" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(stored));
    expect(mockUpdate).toHaveBeenCalledWith(
      "5",
      expect.objectContaining({
        weeks: [
          expect.objectContaining({
            id: "7",
            hostSchoolId: "31",
            timeModels: [
              expect.objectContaining({ id: "9", priceCents: 5025 }),
            ],
          }),
        ],
      }),
    );
  });

  it("does not save while a typed date is no valid date", async () => {
    render(
      <ToastProvider>
        <HolidayPeriodForm
          period={stored}
          schools={schools}
          onSaved={vi.fn()}
          onCancel={vi.fn()}
        />
      </ToastProvider>,
    );

    const opens = screen.getByLabelText("Anmeldebeginn");
    fireEvent.change(opens, { target: { value: "31.02.2027" } });
    fireEvent.blur(opens);
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Bitte prüfen Sie die markierten Felder."),
    ).toBeInTheDocument();
    expect(mockUpdate).not.toHaveBeenCalled();
  });

  it("reports a date text that is no valid date as an unsaved change", () => {
    const onDirtyChange = vi.fn();
    render(
      <ToastProvider>
        <HolidayPeriodForm
          period={stored}
          schools={schools}
          onSaved={vi.fn()}
          onCancel={vi.fn()}
          onDirtyChange={onDirtyChange}
        />
      </ToastProvider>,
    );

    const opens = screen.getByLabelText("Anmeldebeginn");
    fireEvent.change(opens, { target: { value: "31.02.2027" } });
    fireEvent.blur(opens);

    expect(onDirtyChange).toHaveBeenLastCalledWith(true);
  });

  it("moves the focus to a missing time and locks the form while saving", async () => {
    let finish: (value: HolidayPeriod) => void = () => undefined;
    mockUpdate.mockReturnValue(
      new Promise<HolidayPeriod>((resolve) => {
        finish = resolve;
      }),
    );
    render(
      <ToastProvider>
        <HolidayPeriodForm
          period={stored}
          schools={schools}
          onSaved={vi.fn()}
          onCancel={vi.fn()}
        />
      </ToastProvider>,
    );

    const start = screen.getByRole("textbox", { name: /Beginn/ });
    fireEvent.change(start, { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() => expect(start).toHaveFocus());
    expect(
      screen.getByText("Bitte tragen Sie den Beginn ein."),
    ).toBeInTheDocument();

    fireEvent.change(start, { target: { value: "08:00" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() => expect(mockUpdate).toHaveBeenCalled());
    expect(screen.getByLabelText("Name des Ferienzeitraums")).toBeDisabled();
    finish(stored);
    await waitFor(() =>
      expect(screen.getByLabelText("Name des Ferienzeitraums")).toBeEnabled(),
    );
  });

  it("stops adding time models at the limit of a week", () => {
    render(
      <ToastProvider>
        <HolidayPeriodForm
          period={stored}
          schools={schools}
          onSaved={vi.fn()}
          onCancel={vi.fn()}
        />
      </ToastProvider>,
    );

    for (let added = 1; added < 10; added += 1) {
      fireEvent.click(
        screen.getByRole("button", { name: "Zeitmodell hinzufügen" }),
      );
    }

    expect(
      screen.getByText("Eine Woche hat höchstens 10 Zeitmodelle."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Zeitmodell hinzufügen" }),
    ).toBeNull();
  });

  it("forgets the date flags of a removed week", async () => {
    mockUpdate.mockResolvedValue(stored);
    const onDirtyChange = vi.fn();
    render(
      <ToastProvider>
        <HolidayPeriodForm
          period={stored}
          schools={schools}
          onSaved={vi.fn()}
          onCancel={vi.fn()}
          onDirtyChange={onDirtyChange}
        />
      </ToastProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Woche hinzufügen" }));
    const added = screen.getAllByLabelText("Erster Tag")[1]!;
    fireEvent.change(added, { target: { value: "99.99.9999" } });
    fireEvent.blur(added);
    fireEvent.click(
      screen.getAllByRole("button", { name: "Woche entfernen" })[1]!,
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() => expect(onDirtyChange).toHaveBeenLastCalledWith(false));

    // A new invalid date after the save is an unsaved change again.
    const first = screen.getByLabelText("Erster Tag");
    fireEvent.change(first, { target: { value: "31.02.2027" } });
    fireEvent.blur(first);
    expect(onDirtyChange).toHaveBeenLastCalledWith(true);
  });
});

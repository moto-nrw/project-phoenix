import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { OfferingChangeRequestModal } from "./offering-change-request-modal";
import {
  getChildOfferingCatalog,
  type OfferingCatalog,
} from "~/lib/parent-api";

vi.mock("~/lib/parent-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/parent-api")>();
  return { ...actual, getChildOfferingCatalog: vi.fn() };
});

const mockCatalog = vi.mocked(getChildOfferingCatalog);

function catalog(overrides: Partial<OfferingCatalog> = {}): OfferingCatalog {
  return {
    phase_name: "Schuljahr 2026/27",
    selection_mode: "optional",
    earliest_effective_from: "2026-08-14",
    latest_effective_from: "2027-07-31",
    items: [
      {
        id: "5",
        name: "Regelbetreuung",
        days_of_week_mode: "parent_choice",
        available_days: ["mon", "tue", "wed", "thu", "fri"],
        selection_rule: "optional",
        is_required: false,
        includes_lunch: false,
        includes_holiday_care: false,
        selected: true,
        selected_days: ["mon", "tue"],
      },
      {
        id: "6",
        name: "Ferienbetreuung",
        days_of_week_mode: "fixed",
        available_days: ["mon", "tue"],
        selection_rule: "optional",
        is_required: false,
        includes_lunch: false,
        includes_holiday_care: true,
        selected: false,
        selected_days: [],
        capacity: 10,
        free_slots: 3,
      },
    ],
    ...overrides,
  };
}

describe("OfferingChangeRequestModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCatalog.mockResolvedValue(catalog());
  });

  it("prefills the current booking and its days", async () => {
    render(
      <OfferingChangeRequestModal
        studentId="42"
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(await screen.findByText("Regelbetreuung")).toBeInTheDocument();
    const checkboxes = screen.getAllByRole("checkbox");
    // First checkbox is the already-booked offering.
    expect(checkboxes[0]).toBeChecked();
    // Its day rows render because it is parent_choice and selected.
    expect(screen.getByText("Mo")).toBeInTheDocument();
  });

  it("refuses a submission that matches the current booking", async () => {
    const onSubmit = vi.fn();
    render(
      <OfferingChangeRequestModal
        studentId="42"
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Anfrage senden" }),
    );

    expect(
      await screen.findByText(/entspricht der aktuellen Buchung/),
    ).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("explains and disables submission when the catalog is empty", async () => {
    mockCatalog.mockResolvedValue(catalog({ items: [] }));
    const onSubmit = vi.fn();
    render(
      <OfferingChangeRequestModal
        studentId="42"
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    expect(
      await screen.findByText(
        "Für diesen Zeitraum sind keine Betreuungsangebote verfügbar.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Anfrage senden" }),
    ).toBeDisabled();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("sends the complete desired booking with the effective date", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <OfferingChangeRequestModal
        studentId="42"
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    // Add the second (fixed-days) offering: its days are implied by the school.
    // Reached through its own label row — a positional index would hit one of
    // the first offering's day checkboxes.
    const row = (await screen.findByText("Ferienbetreuung")).closest("label");
    const checkbox = row?.querySelector('input[type="checkbox"]');
    fireEvent.click(checkbox!);

    fireEvent.click(screen.getByRole("button", { name: "Anfrage senden" }));

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        offerings: [
          { offering_id: "5", selected_days: ["mon", "tue"] },
          { offering_id: "6", selected_days: [] },
        ],
        effective_from: "2026-08-14",
        note: undefined,
      }),
    );
  });

  it("requires at least one day for a parent-choice offering", async () => {
    mockCatalog.mockResolvedValue(
      catalog({
        items: [
          {
            id: "7",
            name: "Frühbetreuung",
            days_of_week_mode: "parent_choice",
            available_days: ["mon", "tue"],
            selection_rule: "optional",
            is_required: false,
            includes_lunch: false,
            includes_holiday_care: false,
            selected: false,
            selected_days: [],
          },
        ],
      }),
    );
    const onSubmit = vi.fn();
    render(
      <OfferingChangeRequestModal
        studentId="42"
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click((await screen.findAllByRole("checkbox"))[0]!);
    fireEvent.click(screen.getByRole("button", { name: "Anfrage senden" }));

    expect(await screen.findByText(/mindestens einen Tag/)).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("blocks selecting an offering that is full", async () => {
    mockCatalog.mockResolvedValue(
      catalog({
        items: [
          {
            id: "8",
            name: "Volles Angebot",
            days_of_week_mode: "fixed",
            available_days: ["mon"],
            selection_rule: "optional",
            is_required: false,
            includes_lunch: false,
            includes_holiday_care: false,
            selected: false,
            selected_days: [],
            capacity: 5,
            free_slots: 0,
          },
        ],
      }),
    );
    render(
      <OfferingChangeRequestModal
        studentId="42"
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(await screen.findByText("Belegt")).toBeInTheDocument();
    expect(screen.getAllByRole("checkbox")[0]).toBeDisabled();
  });

  it("shows a load error when the catalog cannot be fetched", async () => {
    mockCatalog.mockRejectedValue(new Error("boom"));
    render(
      <OfferingChangeRequestModal
        studentId="42"
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(
      await screen.findByText(/Angebote konnten nicht geladen werden/),
    ).toBeInTheDocument();
  });
});

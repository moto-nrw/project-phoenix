import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
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

vi.mock("~/components/ui/date-picker", () => ({
  ISODatePicker: ({
    max,
    onChange,
  }: {
    max?: string;
    onChange: (date: string) => void;
  }) => (
    <>
      <output data-testid="effective-from-max">{max}</output>
      <button type="button" onClick={() => onChange("2026-09-01")}>
        Datum ändern
      </button>
      <button type="button" onClick={() => onChange("2027-08-14")}>
        Nächsten Zeitraum wählen
      </button>
    </>
  ),
}));

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

  it("submits a selection matching the earliest booking for a later date", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
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

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        offerings: [{ offering_id: "5", selected_days: ["mon", "tue"] }],
        effective_from: "2026-08-14",
        note: undefined,
      }),
    );
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
        "Für diesen Zeitraum ist keine Betreuung buchbar.",
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

  it("uses the catalog for the newly selected effective date without edits", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    mockCatalog.mockResolvedValueOnce(catalog()).mockResolvedValueOnce(
      catalog({
        items: [
          {
            ...catalog().items[1]!,
            selected: true,
          },
        ],
      }),
    );
    render(
      <OfferingChangeRequestModal
        studentId="42"
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    await screen.findByText("Regelbetreuung");
    fireEvent.click(screen.getByRole("button", { name: "Datum ändern" }));
    expect(await screen.findByText("Ferienbetreuung")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Anfrage senden" }));

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        offerings: [{ offering_id: "6", selected_days: [] }],
        effective_from: "2026-09-01",
        note: undefined,
      }),
    );
  });

  it("allows selecting an approved later care period within the enrollment window", async () => {
    mockCatalog
      .mockResolvedValueOnce(catalog({ latest_effective_from: "2028-07-31" }))
      .mockResolvedValueOnce(
        catalog({
          phase_name: "Schuljahr 2027/28",
          earliest_effective_from: "2027-08-01",
          latest_effective_from: "2028-07-31",
          items: [
            {
              ...catalog().items[0]!,
              name: "Angebot nächster Zeitraum",
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

    await screen.findByText("Regelbetreuung");
    expect(screen.getByTestId("effective-from-max")).toHaveTextContent(
      "2028-07-31",
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Nächsten Zeitraum wählen" }),
    );

    await waitFor(() =>
      expect(mockCatalog).toHaveBeenLastCalledWith("42", "2027-08-14"),
    );
    expect(
      await screen.findByText("Angebot nächster Zeitraum"),
    ).toBeInTheDocument();
  });

  it("keeps only selections the guardian edited when changing the date", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    mockCatalog.mockResolvedValueOnce(catalog()).mockResolvedValueOnce(
      catalog({
        items: [
          {
            ...catalog().items[0]!,
            selected: true,
          },
          {
            ...catalog().items[1]!,
            selected: true,
          },
        ],
      }),
    );
    render(
      <OfferingChangeRequestModal
        studentId="42"
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    const ruleOffering = (await screen.findByText("Regelbetreuung")).closest(
      "label",
    );
    fireEvent.click(ruleOffering?.querySelector('input[type="checkbox"]')!);
    fireEvent.click(screen.getByRole("button", { name: "Datum ändern" }));
    await screen.findByText("Ferienbetreuung");
    fireEvent.click(screen.getByRole("button", { name: "Anfrage senden" }));

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        offerings: [{ offering_id: "6", selected_days: [] }],
        effective_from: "2026-09-01",
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

describe("Mitbuchungs-Regel preview (#2366)", () => {
  function catalogWithAutoRule(): OfferingCatalog {
    const base = catalog();
    return {
      ...base,
      items: [
        ...base.items,
        {
          id: "7",
          name: "Ganztagsbetreuung",
          days_of_week_mode: "parent_choice",
          available_days: ["mon", "tue", "wed", "thu", "fri"],
          selection_rule: "optional",
          is_required: false,
          includes_lunch: false,
          includes_holiday_care: false,
          counts_as_care: true,
          auto_add_trigger_offering_ids: ["5"],
          selected: false,
          selected_days: [],
        },
      ],
    };
  }

  it("shows which days the rule books automatically while the trigger is selected", async () => {
    mockCatalog.mockResolvedValue(catalogWithAutoRule());
    render(
      <OfferingChangeRequestModal
        studentId="42"
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    // Regelbetreuung (the trigger) is booked mon+tue, so the rule target
    // announces the automatic booking before anything is submitted.
    expect(
      await screen.findByText(
        "Automatisch mitgebucht: Mo, Di, weil Regelbetreuung gewählt ist.",
      ),
    ).toBeInTheDocument();
  });

  it("drops the preview when the trigger is deselected", async () => {
    mockCatalog.mockResolvedValue(catalogWithAutoRule());
    render(
      <OfferingChangeRequestModal
        studentId="42"
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    const trigger = (await screen.findByText("Regelbetreuung")).closest(
      "label",
    );
    fireEvent.click(trigger!.querySelector("input")!);

    await waitFor(() =>
      expect(
        screen.queryByText(/Automatisch mitgebucht:/),
      ).not.toBeInTheDocument(),
    );
  });

  it("removes an existing automatic offering when its trigger is deselected", async () => {
    const base = catalogWithAutoRule();
    mockCatalog.mockResolvedValue({
      ...base,
      items: base.items.map((item) =>
        item.id === "7"
          ? {
              ...item,
              automatic: true,
              selected: true,
              selected_days: ["mon", "tue"],
            }
          : item,
      ),
    });
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <OfferingChangeRequestModal
        studentId="42"
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    const trigger = (await screen.findByText("Regelbetreuung")).closest(
      "label",
    );
    fireEvent.click(trigger!.querySelector("input")!);

    expect(
      screen.getByRole("checkbox", { name: /Ganztagsbetreuung/ }),
    ).not.toBeChecked();
    expect(
      screen.queryByText("Kommt automatisch dazu."),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Anfrage senden" }));

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        offerings: [],
        effective_from: "2026-08-14",
        note: undefined,
      }),
    );
  });

  it("renders a newly auto-added offering as selected with locked days", async () => {
    mockCatalog.mockResolvedValue(catalogWithAutoRule());
    render(
      <OfferingChangeRequestModal
        studentId="42"
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    const offering = await screen.findByRole("checkbox", {
      name: /Ganztagsbetreuung/,
    });
    expect(offering).toBeChecked();
    expect(offering).toBeDisabled();
    const card = offering.closest("label")?.parentElement;
    expect(within(card!).getByRole("checkbox", { name: "Mo" })).toBeDisabled();
    expect(within(card!).getByRole("checkbox", { name: "Di" })).toBeDisabled();
  });

  it("accepts parent-choice offerings whose only days are automatic", async () => {
    const base = catalogWithAutoRule();
    mockCatalog.mockResolvedValue({
      ...base,
      items: base.items.map((item) =>
        item.id === "7" ? { ...item, selected: true } : item,
      ),
    });
    const onSubmit = vi.fn().mockResolvedValue(undefined);
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

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        offerings: [
          { offering_id: "5", selected_days: ["mon", "tue"] },
          { offering_id: "7", selected_days: [] },
        ],
        effective_from: "2026-08-14",
        note: undefined,
      }),
    );
  });

  it("does not submit former automatic days as manual after removing their trigger", async () => {
    const base = catalogWithAutoRule();
    mockCatalog.mockResolvedValue({
      ...base,
      items: base.items.map((item) => {
        if (item.id === "5") {
          return {
            ...item,
            selected_days: ["tue"],
            manual_selected_days: ["tue"],
          };
        }
        if (item.id === "7") {
          return {
            ...item,
            selected: true,
            selected_days: ["mon", "tue"],
            manual_selected_days: ["mon"],
          };
        }
        return item;
      }),
    });
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <OfferingChangeRequestModal
        studentId="42"
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    const trigger = (await screen.findByText("Regelbetreuung")).closest(
      "label",
    );
    fireEvent.click(trigger!.querySelector("input")!);
    fireEvent.click(screen.getByRole("button", { name: "Anfrage senden" }));

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({
        offerings: [{ offering_id: "7", selected_days: ["mon"] }],
        effective_from: "2026-08-14",
        note: undefined,
      }),
    );
  });

  it("explains an existing automatic booking while its trigger remains selected", async () => {
    const base = catalogWithAutoRule();
    mockCatalog.mockResolvedValue({
      ...base,
      items: base.items.map((item) =>
        item.id === "7"
          ? {
              ...item,
              automatic: true,
              selected: true,
              selected_days: ["mon", "tue"],
            }
          : item,
      ),
    });
    render(
      <OfferingChangeRequestModal
        studentId="42"
        onClose={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(
      await screen.findByText(
        "Automatisch mitgebucht: Mo, Di, weil Regelbetreuung gewählt ist.",
      ),
    ).toBeInTheDocument();
  });
});

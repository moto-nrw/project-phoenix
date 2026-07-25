import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

// ============================================================================
// Mocks
// ============================================================================

// The date and datetime fields moved from native inputs to the kit pickers;
// these stubs keep them settable via fireEvent.change. Imported inside the
// factories because vi.mock is hoisted above the imports.
vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

vi.mock("~/components/ui/date-time-picker", async (importOriginal) => {
  const { dateTimePickerMock } = await import("~/test/mocks/date-time-picker");
  return { ...(await importOriginal<object>()), ...dateTimePickerMock() };
});

const { mockCreateRollover } = vi.hoisted(() => ({
  mockCreateRollover: vi.fn(),
}));

vi.mock("~/lib/enrollment-phase-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    createRollover: mockCreateRollover,
  };
});

import { RolloverForm } from "./rollover-form";
import type { Phase, RolloverResult } from "~/lib/enrollment-phase-api";

async function chooseOption(
  label: string | RegExp,
  optionName: string | RegExp,
) {
  fireEvent.click(screen.getByLabelText(label));
  fireEvent.click(await screen.findByRole("option", { name: optionName }));
}

// ============================================================================
// Test data
// ============================================================================

function makeSourcePhase(overrides: Partial<Phase> = {}): Phase {
  return {
    id: "42",
    name: "Schuljahr 2026",
    kind: "school_year",
    service_start_date: "2026-09-01",
    service_end_date: "2027-07-31",
    enrollment_open_at: null,
    enrollment_close_at: null,
    form_schema_id: "7",
    show_status_reason_to_parent: false,
    care_overflow_mode: "waitlist",
    care_offering_selection_mode: "optional",
    is_active: true,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

function makeResult(): RolloverResult {
  return {
    phase: {
      ...makeSourcePhase(),
      id: "99",
      name: "Schuljahr 2027",
      service_start_date: "2027-09-01",
      service_end_date: "2028-07-31",
      rollover_source_phase_id: "42",
      rollover_mode: "opt_out",
    },
    summary: {
      source_child_count: 5,
      rolled_count: 4,
      review_count: 1,
      review_by_reason: { grade_above_max: 1 },
      request_count: 3,
      enqueued_emails: 3,
      skipped_empty_email: 0,
    },
  };
}

// ============================================================================
// Tests
// ============================================================================

describe("RolloverForm", () => {
  beforeEach(() => {
    mockCreateRollover.mockReset();
  });

  it("pre-fills the form from the source phase", () => {
    const source = makeSourcePhase();
    render(
      <RolloverForm source={source} onCancel={vi.fn()} onSuccess={vi.fn()} />,
    );

    const nameInput = screen.getByLabelText(
      /Name der neuen Phase/,
    ) as HTMLInputElement;
    expect(nameInput.value).toBe("Schuljahr 2026 (Folgejahr)");

    // service dates bumped one year
    const startInput = screen.getByLabelText(
      /Betreuung von/,
    ) as HTMLInputElement;
    expect(startInput.value).toBe("2027-09-01");
    const endInput = screen.getByLabelText(/Betreuung bis/) as HTMLInputElement;
    expect(endInput.value).toBe("2028-07-31");

    // mode defaults to opt_out (safer default — parent doesn't need to act)
    expect(screen.getByRole("combobox", { name: "Modus" })).toHaveTextContent(
      "Opt-Out: Eltern müssen abmelden",
    );

    // bumps_grade defaults to true (yearly cadence)
    const bumpsCheckbox = screen.getByLabelText(
      /Klassenstufe automatisch um 1 erhöhen/,
    ) as HTMLInputElement;
    expect(bumpsCheckbox.checked).toBe(true);

    // auto_approve defaults to false (admin still reviews in slice 1)
    const autoApproveCheckbox = screen.getByLabelText(
      /Vorgemerkte Anmeldungen automatisch genehmigen/,
    ) as HTMLInputElement;
    expect(autoApproveCheckbox.checked).toBe(false);
  });

  it("submits the prefilled values and reports success", async () => {
    mockCreateRollover.mockResolvedValueOnce(makeResult());
    const onSuccess = vi.fn();
    const source = makeSourcePhase();

    render(
      <RolloverForm source={source} onCancel={vi.fn()} onSuccess={onSuccess} />,
    );

    fireEvent.submit(
      screen.getByRole("button", { name: /Anschlussphase erstellen/ }),
    );

    await waitFor(() => {
      expect(mockCreateRollover).toHaveBeenCalledTimes(1);
    });
    const [sourceID, input] = mockCreateRollover.mock.calls[0] as [
      string,
      Record<string, unknown>,
    ];
    expect(sourceID).toBe("42");
    expect(input.name).toBe("Schuljahr 2026 (Folgejahr)");
    expect(input.rollover_mode).toBe("opt_out");
    expect(input.rollover_bumps_grade).toBe(true);
    expect(input.rollover_auto_approve).toBe(false);

    // onSuccess fires with the full result.
    expect(onSuccess).toHaveBeenCalledTimes(1);
    expect(onSuccess.mock.calls[0]?.[0]).toMatchObject({
      phase: { name: "Schuljahr 2027" },
      summary: { rolled_count: 4 },
    });
  });

  it("toggles opt-in hint when the mode changes", async () => {
    render(
      <RolloverForm
        source={makeSourcePhase()}
        onCancel={vi.fn()}
        onSuccess={vi.fn()}
      />,
    );

    // opt_out is the default — full sentence appears below the select.
    expect(
      screen.getByText(
        /Anmeldungen werden automatisch übernommen\. Ohne aktive Abmeldung/,
      ),
    ).toBeInTheDocument();

    await chooseOption("Modus", "Opt-In: Eltern müssen aktiv bestätigen");

    // The opt_in description ends with "wird die Anmeldung zurückgezogen" —
    // matching the unique tail keeps us off the dropdown <option> text.
    expect(
      screen.getByText(
        /Ohne Bestätigung bis zur Frist wird die Anmeldung zurückgezogen/,
      ),
    ).toBeInTheDocument();
  });

  it("surfaces API errors and stays open", async () => {
    mockCreateRollover.mockRejectedValueOnce(new Error("nichts zu übernehmen"));
    const onSuccess = vi.fn();
    render(
      <RolloverForm
        source={makeSourcePhase()}
        onCancel={vi.fn()}
        onSuccess={onSuccess}
      />,
    );

    fireEvent.submit(
      screen.getByRole("button", { name: /Anschlussphase erstellen/ }),
    );

    await waitFor(() => {
      expect(screen.getByText("nichts zu übernehmen")).toBeInTheDocument();
    });
    expect(onSuccess).not.toHaveBeenCalled();
  });

  it("calls onCancel when the cancel button is clicked", () => {
    const onCancel = vi.fn();
    render(
      <RolloverForm
        source={makeSourcePhase()}
        onCancel={onCancel}
        onSuccess={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: /Abbrechen/ }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("respects an unchecked bumps_grade", async () => {
    mockCreateRollover.mockResolvedValueOnce(makeResult());
    render(
      <RolloverForm
        source={makeSourcePhase()}
        onCancel={vi.fn()}
        onSuccess={vi.fn()}
      />,
    );

    fireEvent.click(
      screen.getByLabelText(/Klassenstufe automatisch um 1 erhöhen/),
    );
    fireEvent.submit(
      screen.getByRole("button", { name: /Anschlussphase erstellen/ }),
    );

    await waitFor(() => {
      expect(mockCreateRollover).toHaveBeenCalled();
    });
    const input = mockCreateRollover.mock.calls[0]?.[1] as Record<
      string,
      unknown
    >;
    expect(input.rollover_bumps_grade).toBe(false);
  });
});

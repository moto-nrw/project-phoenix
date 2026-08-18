import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ChildCareOfferingsSection } from "./child-care-offerings";
import {
  getChildCareOfferings,
  withdrawOfferingChangeRequest,
  type ChildCareOfferings,
} from "~/lib/parent-api";

vi.mock("~/lib/parent-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/parent-api")>();
  return {
    ...actual,
    getChildCareOfferings: vi.fn(),
    getChildOfferingCatalog: vi.fn(),
    submitOfferingChangeRequest: vi.fn(),
    withdrawOfferingChangeRequest: vi.fn(),
  };
});

const mockGet = vi.mocked(getChildCareOfferings);
const mockWithdraw = vi.mocked(withdrawOfferingChangeRequest);

function view(overrides: Partial<ChildCareOfferings> = {}): ChildCareOfferings {
  return {
    period_name: "Schuljahr 2026/27",
    period_start: "2026-09-01",
    period_end: "2027-07-31",
    offerings: [
      {
        id: "5",
        name: "Regelbetreuung",
        weekdays: [1, 2, 3],
        includes_lunch: true,
        includes_holiday_care: false,
      },
    ],
    groups: [
      {
        id: "9",
        name: "Gruppe Sonne",
        weekdays: [1, 2, 3],
        valid_from: "2026-09-01",
        valid_until: "2027-07-31",
        from_offering: true,
        starts_later: false,
      },
    ],
    can_request: true,
    earliest_effective_from: "2026-08-14",
    ...overrides,
  };
}

const pending: ChildCareOfferings["pending_request"] = {
  id: "77",
  created_at: "2026-07-30T10:00:00Z",
  effective_from: "2027-02-01",
  diff: [
    {
      label: "Regelbetreuung",
      old_state: "booked",
      old_days: ["mon", "tue", "wed"],
      new_state: "booked",
      new_days: ["mon", "tue"],
    },
  ],
  submitted_by_self: true,
};

describe("ChildCareOfferingsSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGet.mockResolvedValue(view());
  });

  it("renders the booked offerings and groups", async () => {
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(await screen.findByText("Betreuungsangebote")).toBeInTheDocument();
    expect(screen.getByText("Regelbetreuung")).toBeInTheDocument();
    expect(screen.getByText("Gruppe Sonne")).toBeInTheDocument();
    // Weekdays render as short labels, not raw ISO numbers.
    expect(screen.getAllByText("Mo, Di, Mi").length).toBeGreaterThan(0);
    expect(
      screen.getByRole("button", { name: "Änderung anfragen" }),
    ).toBeInTheDocument();
  });

  it("refreshes when the parent SSE bridge invalidates this child's care state", async () => {
    mockGet.mockResolvedValueOnce(view()).mockResolvedValueOnce(
      view({
        offerings: [
          {
            id: "6",
            name: "Spätbetreuung",
            weekdays: [4, 5],
            includes_lunch: false,
            includes_holiday_care: false,
          },
        ],
      }),
    );
    render(<ChildCareOfferingsSection studentId="42" />);

    await screen.findByText("Regelbetreuung");
    fireEvent(
      window,
      new CustomEvent("parent-conversation-refresh", {
        detail: { studentId: "42" },
      }),
    );

    expect(await screen.findByText("Spätbetreuung")).toBeInTheDocument();
    expect(mockGet).toHaveBeenCalledTimes(2);
  });

  it("shows an offering with no weekday restriction as covering all care days", async () => {
    mockGet.mockResolvedValue(
      view({
        offerings: [
          {
            id: "5",
            name: "Mittagessen",
            weekdays: [],
            includes_lunch: true,
            includes_holiday_care: false,
          },
        ],
      }),
    );
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(await screen.findByText("alle Betreuungstage")).toBeInTheDocument();
  });

  it("marks a membership that has not started yet", async () => {
    mockGet.mockResolvedValue(
      view({
        groups: [
          {
            id: "9",
            name: "Gruppe Mond",
            weekdays: [4],
            valid_from: "2027-02-01",
            from_offering: true,
            starts_later: true,
          },
        ],
      }),
    );
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(await screen.findByText("Gruppe Mond")).toBeInTheDocument();
    expect(screen.getByText("ab 01.02.2027")).toBeInTheDocument();
  });

  it("shows the pending request with its effective date, diff and withdraw button", async () => {
    mockGet.mockResolvedValue(view({ pending_request: pending }));
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(await screen.findByText("In Prüfung")).toBeInTheDocument();
    expect(screen.getByText("Gewünscht ab 01.02.2027")).toBeInTheDocument();
    expect(screen.getByText("Mo, Di")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Anfrage zurückziehen" }),
    ).toBeInTheDocument();
    // No second request while one is open.
    expect(
      screen.queryByRole("button", { name: "Änderung anfragen" }),
    ).not.toBeInTheDocument();
  });

  it("hides the withdraw button for another guardian's request", async () => {
    mockGet.mockResolvedValue(
      view({ pending_request: { ...pending, submitted_by_self: false } }),
    );
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(await screen.findByText("In Prüfung")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Anfrage zurückziehen" }),
    ).not.toBeInTheDocument();
  });

  it("explains a disabled change option instead of only hiding the button", async () => {
    mockGet.mockResolvedValue(
      view({ can_request: false, changes_disabled_reason: "school_disabled" }),
    );
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(
      await screen.findByText(/nimmt Änderungswünsche nicht über die App an/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Änderung anfragen" }),
    ).not.toBeInTheDocument();
  });

  it("stays silent about a missing permission", async () => {
    mockGet.mockResolvedValue(
      view({ can_request: false, changes_disabled_reason: "no_permission" }),
    );
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(await screen.findByText("Betreuungsangebote")).toBeInTheDocument();
    expect(screen.queryByText(/keine Berechtigung/)).not.toBeInTheDocument();
  });

  it("shows an approved decision with the date it applies from", async () => {
    mockGet.mockResolvedValue(
      view({
        last_decision: {
          id: "77",
          status: "approved",
          decided_at: "2026-07-30T10:00:00Z",
          effective_from: "2027-02-01",
          requested: [{ id: "5", name: "Regelbetreuung", weekdays: [1, 2] }],
        },
      }),
    );
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(await screen.findByText("Änderungsanfragen")).toBeInTheDocument();
    expect(screen.getByText("Änderung übernommen")).toBeInTheDocument();
    expect(screen.getByText("Gültig ab 01.02.2027")).toBeInTheDocument();
    // A new request stays possible right away.
    expect(
      screen.getByRole("button", { name: "Änderung anfragen" }),
    ).toBeInTheDocument();
  });

  it("shows a rejection with the reason the office gave", async () => {
    mockGet.mockResolvedValue(
      view({
        last_decision: {
          id: "78",
          status: "rejected",
          decided_at: "2026-07-30T10:00:00Z",
          effective_from: "2027-02-01",
          reason: "Kein Platz in der Gruppe",
          requested: [{ id: "5", name: "Regelbetreuung", weekdays: [1, 2] }],
        },
      }),
    );
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(await screen.findByText("Anfrage abgelehnt")).toBeInTheDocument();
    expect(screen.getByText(/Kein Platz in der Gruppe/)).toBeInTheDocument();
    // The request itself stays readable next to the rejection. Scoped to the
    // recap block, because the offering name also appears in the booking list.
    const recap = screen.getByText("Beantragt war:").parentElement;
    expect(recap?.textContent).toContain("Regelbetreuung");
    expect(recap?.textContent).toContain("Mo, Di");
    // A rejected request carries no effective date claim.
    expect(screen.queryByText(/Gültig ab/)).not.toBeInTheDocument();
  });

  it("labels a rejected frozen diff as requested rather than applied", async () => {
    mockGet.mockResolvedValue(
      view({
        last_decision: {
          id: "79",
          status: "rejected",
          decided_at: "2026-07-30T10:00:00Z",
          effective_from: "2027-02-01",
          requested: [{ id: "5", name: "Regelbetreuung", weekdays: [1, 2] }],
          applied: [
            {
              label: "Regelbetreuung",
              old_state: "booked",
              old_days: ["mon", "tue", "wed"],
              new_state: "booked",
              new_days: ["mon", "tue"],
            },
          ],
        },
      }),
    );
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(await screen.findByText("Angefragte Änderung:")).toBeInTheDocument();
    expect(screen.queryByText("Das wurde geändert:")).not.toBeInTheDocument();
  });

  it("does not fall back to the request when the frozen diff is empty", async () => {
    mockGet.mockResolvedValue(
      view({
        last_decision: {
          id: "80",
          status: "approved",
          decided_at: "2026-07-30T10:00:00Z",
          effective_from: "2027-02-01",
          requested: [{ id: "5", name: "Regelbetreuung", weekdays: [1, 2] }],
          applied: [],
        },
      }),
    );
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(await screen.findByText("Änderung übernommen")).toBeInTheDocument();
    expect(screen.queryByText("Beantragt war:")).not.toBeInTheDocument();
  });

  it("prefers the open request over an older decision", async () => {
    mockGet.mockResolvedValue(
      view({
        pending_request: pending,
        last_decision: {
          id: "77",
          status: "rejected",
          decided_at: "2026-07-20T10:00:00Z",
          effective_from: "2026-12-01",
          reason: "Alte Ablehnung",
          requested: [],
        },
      }),
    );
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(await screen.findByText("In Prüfung")).toBeInTheDocument();
    expect(screen.queryByText("Anfrage abgelehnt")).not.toBeInTheDocument();
  });

  it("says so when there is no request at all", async () => {
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(
      await screen.findByText("Derzeit liegt keine Änderungsanfrage vor."),
    ).toBeInTheDocument();
  });

  it("withdraws the pending request through the confirmation modal", async () => {
    mockGet.mockResolvedValue(view({ pending_request: pending }));
    mockWithdraw.mockResolvedValue(view());
    render(<ChildCareOfferingsSection studentId="42" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Anfrage zurückziehen" }),
    );
    const confirmButtons = await screen.findAllByRole("button", {
      name: "Anfrage zurückziehen",
    });
    // The modal's confirm button is the last one rendered.
    fireEvent.click(confirmButtons[confirmButtons.length - 1]!);

    await waitFor(() => expect(mockWithdraw).toHaveBeenCalledWith("42", "77"));
  });

  it("shows a load error without breaking the page", async () => {
    mockGet.mockRejectedValue(new Error("boom"));
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(
      await screen.findByText(/konnten nicht geladen werden/),
    ).toBeInTheDocument();
  });

  // Mitbuchungs-Regeln (#2365/#2370): rule-added lines are explained, and an
  // override by the school is named in the decided recap.
  it("explains a rule-added line in the pending request", async () => {
    mockGet.mockResolvedValue(
      view({
        pending_request: {
          ...pending,
          diff: [
            ...pending!.diff,
            {
              label: "Ganztagsbetreuung bis 14.30 Uhr",
              old_state: "not_booked",
              old_days: [],
              new_state: "booked",
              new_days: ["mon", "tue"],
              new_automatic_days: ["mon", "tue"],
              auto_trigger_names: ["Randstunde"],
            },
          ],
        },
      }),
    );
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(
      await screen.findByText(
        /Kommt automatisch dazu, weil „Randstunde“ gewählt ist\./,
      ),
    ).toBeInTheDocument();
  });

  it("shows the applied result and a school override in the decided recap", async () => {
    mockGet.mockResolvedValue(
      view({
        last_decision: {
          id: "88",
          status: "approved",
          decided_at: "2026-08-17T09:00:00Z",
          effective_from: "2027-02-01",
          requested: [{ id: "5", name: "Randstunde", weekdays: [1, 2] }],
          applied: [
            {
              label: "Randstunde",
              old_state: "not_booked",
              old_days: [],
              new_state: "booked",
              new_days: ["mon", "tue"],
            },
          ],
          overridden_names: ["Ganztagsbetreuung bis 14.30 Uhr"],
        },
      }),
    );
    render(<ChildCareOfferingsSection studentId="42" />);

    expect(await screen.findByText("Das wurde geändert:")).toBeInTheDocument();
    expect(screen.getByText("Randstunde")).toBeInTheDocument();
    expect(
      screen.getByText(
        /Die Betreuung hat „Ganztagsbetreuung bis 14\.30 Uhr“ nicht mitgebucht\./,
      ),
    ).toBeInTheDocument();
  });
});

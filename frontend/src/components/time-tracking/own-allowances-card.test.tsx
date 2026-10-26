import { act, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ABSENCES_REFRESH_EVENT } from "~/lib/absence-helpers";
import { ApiError } from "~/lib/api-error";
import type { OwnAbsenceAllowance } from "~/lib/time-tracking-api";
import { catalogText } from "~/test/error-catalog-text";

import { OwnAllowancesCard } from "./own-allowances-card";

const mocks = vi.hoisted(() => ({
  getOwnAbsenceAllowances: vi.fn(),
}));

vi.mock("~/lib/time-tracking-api", () => ({
  timeTrackingService: {
    getOwnAbsenceAllowances: mocks.getOwnAbsenceAllowances,
  },
}));

function regeneration(): OwnAbsenceAllowance {
  return {
    name: "Regenerationstag",
    summary: {
      staffId: "42",
      absenceTypeId: "4",
      year: 2026,
      entitledDays: 3,
      takenDays: 1,
      reservedDays: 0.5,
      remainingDays: 1.5,
      expiresOn: "2027-03-31",
      expiredDays: 0,
      bookingDays: 0,
      carriedIn: {
        year: 2025,
        remainingDays: 1,
        expiredDays: 0,
        expiresOn: "2026-03-31",
      },
    },
  };
}

describe("OwnAllowancesCard", () => {
  beforeEach(() => {
    mocks.getOwnAbsenceAllowances.mockReset();
  });

  it("shows each Kontingent read-only with who changes the claim", async () => {
    mocks.getOwnAbsenceAllowances.mockResolvedValue([regeneration()]);

    render(<OwnAllowancesCard />);

    const card = await screen.findByRole("region", {
      name: "Regenerationstag 2026",
    });
    expect(
      screen.getByText(
        "Ihre Tage je Abwesenheitsart. Den Anspruch trägt Ihre Leitung ein.",
      ),
    ).toBeInTheDocument();
    expect(within(card).getByText("Anspruch")).toBeInTheDocument();
    expect(within(card).getByText("3 Tage")).toBeInTheDocument();
    expect(within(card).getByText("1 Tag")).toBeInTheDocument();
    expect(within(card).getByText("0,5 Tage")).toBeInTheDocument();
    expect(within(card).getByText("1,5 Tage")).toBeInTheDocument();
    expect(
      within(card).getByText(
        "Dazu noch 1 Tag aus 2025, nutzbar bis 31.03.2026. Diese gehen zuerst ab.",
      ),
    ).toBeInTheDocument();
    // Anker: die Karte ist geladen; nur lesen, also kein Knopf darin.
    expect(within(card).getByText("Übrig")).toBeInTheDocument();
    expect(within(card).queryByRole("button")).not.toBeInTheDocument();
  });

  it("stays away when the school has no type with an allowance", async () => {
    mocks.getOwnAbsenceAllowances.mockResolvedValue([]);
    const { container } = render(<OwnAllowancesCard />);
    await waitFor(() =>
      expect(mocks.getOwnAbsenceAllowances).toHaveBeenCalledTimes(1),
    );
    await act(async () => undefined);
    expect(container).toBeEmptyDOMElement();

    // Dieselbe Karte erscheint, sobald die Schule eine Art mit Kontingent
    // hat: sie war geladen und leer, nicht im Ladezustand hängen geblieben.
    mocks.getOwnAbsenceAllowances.mockResolvedValue([regeneration()]);
    act(() => {
      window.dispatchEvent(new Event(ABSENCES_REFRESH_EVENT));
    });
    expect(await screen.findByText("Kontingente")).toBeInTheDocument();
  });

  it("shows a failed load in the card instead of zero days", async () => {
    mocks.getOwnAbsenceAllowances.mockRejectedValue(
      new ApiError("down", 503, {
        code: "general.unavailable",
        instance: "req-allowances",
      }),
    );

    render(<OwnAllowancesCard />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Übersicht der Kontingente"),
      ),
    ).toBeInTheDocument();
    // Anker: der Fehler steht im Block; Zahlen erscheinen nicht.
    expect(screen.getByText("Kontingente")).toBeInTheDocument();
    expect(screen.queryByText("Übrig")).not.toBeInTheDocument();
  });

  it("reloads when an absence changed", async () => {
    mocks.getOwnAbsenceAllowances.mockResolvedValue([regeneration()]);
    render(<OwnAllowancesCard />);
    expect(await screen.findByText("1,5 Tage")).toBeInTheDocument();

    const changed = regeneration();
    mocks.getOwnAbsenceAllowances.mockResolvedValue([
      { ...changed, summary: { ...changed.summary, remainingDays: 2 } },
    ]);
    act(() => {
      window.dispatchEvent(new Event(ABSENCES_REFRESH_EVENT));
    });

    expect(await screen.findByText("2 Tage")).toBeInTheDocument();
  });
});

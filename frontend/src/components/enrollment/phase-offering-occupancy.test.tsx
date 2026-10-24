import { render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { CareOffering } from "~/lib/care-offering-api";
import type { CareOfferingBookingStats } from "~/lib/care-offering-booking-stats";

const mocks = vi.hoisted(() => ({
  listCareOfferings: vi.fn(),
  fetchCareOfferingBookingStats: vi.fn(),
}));

vi.mock("~/lib/care-offering-api", () => ({
  listCareOfferings: mocks.listCareOfferings,
}));

vi.mock("~/lib/care-offering-booking-stats", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  fetchCareOfferingBookingStats: mocks.fetchCareOfferingBookingStats,
}));

import { PhaseOfferingOccupancy } from "./phase-offering-occupancy";

function offering(overrides: Partial<CareOffering>): CareOffering {
  return {
    id: "1",
    phase_id: "7",
    name: "Regelbetreuung",
    available_days: ["mon"],
    includes_holiday_care: false,
    includes_lunch: false,
    capacity: 10,
    is_active: true,
    ...overrides,
  } as CareOffering;
}

function stats(
  offeringId: string,
  capacity: number | null,
  booked: number,
): CareOfferingBookingStats {
  return {
    offering_id: offeringId,
    capacity,
    booked,
    grade_levels: {},
    unknown_grade_count: 0,
  };
}

function rowOf(text: string) {
  return within(screen.getByText(text).closest("tr") as HTMLElement);
}

beforeEach(() => {
  mocks.listCareOfferings.mockReset();
  mocks.fetchCareOfferingBookingStats.mockReset();
});

describe("PhaseOfferingOccupancy (#4782)", () => {
  it("shows the same numbers per offering as the offering list", async () => {
    mocks.listCareOfferings.mockResolvedValue([
      offering({ id: "1", name: "Regelbetreuung", capacity: 10 }),
      offering({ id: "2", name: "Randstunde", capacity: 5 }),
      offering({ id: "3", name: "Frühbetreuung", capacity: null }),
    ]);
    mocks.fetchCareOfferingBookingStats.mockResolvedValue({
      "1": stats("1", 10, 7),
      "2": stats("2", 5, 5),
      "3": stats("3", null, 4),
    });

    render(<PhaseOfferingOccupancy phaseId="7" />);

    expect(
      await screen.findByText("Freie Plätze je Angebot"),
    ).toBeInTheDocument();
    expect(mocks.listCareOfferings).toHaveBeenCalledWith("7");
    expect(mocks.fetchCareOfferingBookingStats).toHaveBeenCalledWith("7");

    const regular = rowOf("Regelbetreuung");
    expect(regular.getByText("7 von 10")).toBeInTheDocument();
    expect(regular.getByText("3 frei")).toBeInTheDocument();
    expect(regular.getByText("Aktiv")).toBeInTheDocument();
    expect(regular.queryByText("Voll")).not.toBeInTheDocument();

    const full = rowOf("Randstunde");
    expect(full.getByText("5 von 5")).toBeInTheDocument();
    expect(full.getByText("Voll")).toBeInTheDocument();

    const unlimited = rowOf("Frühbetreuung");
    expect(unlimited.getByText("Unbegrenzt")).toBeInTheDocument();
    expect(unlimited.getByText("4")).toBeInTheDocument();

    // Free places of the phase next to the heading: 3 + 0.
    const heading = screen.getByRole("heading", {
      name: "Freie Plätze je Angebot",
    });
    expect(heading.parentElement).toHaveTextContent("3 frei");
  });

  it("never shows child names, only sums", async () => {
    mocks.listCareOfferings.mockResolvedValue([offering({ id: "1" })]);
    mocks.fetchCareOfferingBookingStats.mockResolvedValue({
      "1": stats("1", 10, 7),
    });

    const { container } = render(<PhaseOfferingOccupancy phaseId="7" />);

    expect(await screen.findByText("7 von 10")).toBeInTheDocument();
    // Booking stats carry no identities; the section renders four columns.
    expect(container.querySelectorAll("tbody tr td")).toHaveLength(4);
  });

  it("hides the section when the occupancy fails to load", async () => {
    mocks.listCareOfferings.mockResolvedValue([offering({ id: "1" })]);
    mocks.fetchCareOfferingBookingStats.mockRejectedValue(
      new Error("stats down"),
    );

    const { container } = render(
      <div data-testid="anchor">
        <PhaseOfferingOccupancy phaseId="7" />
      </div>,
    );

    await waitFor(() =>
      expect(mocks.fetchCareOfferingBookingStats).toHaveBeenCalled(),
    );
    expect(screen.getByTestId("anchor")).toBeInTheDocument();
    await waitFor(() =>
      expect(
        screen.queryByText("Freie Plätze je Angebot"),
      ).not.toBeInTheDocument(),
    );
    expect(container.querySelector("table")).toBeNull();
  });

  it("renders nothing for a phase without offerings", async () => {
    mocks.listCareOfferings.mockResolvedValue([]);
    mocks.fetchCareOfferingBookingStats.mockResolvedValue({});

    render(
      <div data-testid="anchor">
        <PhaseOfferingOccupancy phaseId="7" />
      </div>,
    );

    await waitFor(() => expect(mocks.listCareOfferings).toHaveBeenCalled());
    expect(screen.getByTestId("anchor")).toBeEmptyDOMElement();
  });
});

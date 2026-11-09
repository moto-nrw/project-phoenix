import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { HolidayWeekPlaces } from "~/lib/traeger-holiday-registrations-api";

const { mockSetCapacity } = vi.hoisted(() => ({ mockSetCapacity: vi.fn() }));

vi.mock("~/lib/traeger-holiday-registrations-api", () => ({
  setHolidayWeekCapacity: (
    periodId: string,
    weekId: string,
    capacity: number | null,
  ) => mockSetCapacity(periodId, weekId, capacity),
}));

vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

import {
  HolidayCapacityModal,
  HolidayWeekPlacesCard,
  holidayWeekFill,
} from "./holiday-week-places";

const FULL_WEEK: HolidayWeekPlaces = {
  weekId: "51",
  startsOn: "2027-07-05",
  endsOn: "2027-07-09",
  hostSchoolId: "31",
  capacity: 2,
  placed: 2,
  waitlist: [
    {
      position: 1,
      registrationId: "14",
      registrationStatus: "received",
      submittedAt: "2026-11-09T13:12:00Z",
      childId: "21",
      childFirstName: "Tom",
      childLastName: "Platz",
      timeModelName: "bis 13 Uhr",
    },
  ],
};

describe("holidayWeekFill", () => {
  it("names free places, a full week and a week without limit", () => {
    expect(holidayWeekFill({ ...FULL_WEEK, capacity: 5 })).toEqual({
      label: "3 Plätze frei",
      tone: "green",
    });
    expect(holidayWeekFill({ ...FULL_WEEK, capacity: 3 })).toEqual({
      label: "1 Platz frei",
      tone: "green",
    });
    expect(holidayWeekFill(FULL_WEEK)).toEqual({
      label: "Voll",
      tone: "warning",
    });
    expect(holidayWeekFill({ ...FULL_WEEK, capacity: 1 })).toEqual({
      label: "Voll",
      tone: "warning",
    });
    expect(holidayWeekFill({ ...FULL_WEEK, capacity: null })).toEqual({
      label: "Ohne Grenze",
      tone: "gray",
    });
  });
});

describe("HolidayWeekPlacesCard", () => {
  it("shows the filled places and the waiting list in its order, each child linking to its registration", () => {
    const onChange = vi.fn();
    render(
      <HolidayWeekPlacesCard
        periodId="7"
        week={FULL_WEEK}
        hostSchoolName="OGS am Berg"
        onChangeCapacity={onChange}
      />,
    );
    expect(screen.getByText("Voll")).toBeInTheDocument();
    expect(screen.getByText("OGS am Berg")).toBeInTheDocument();
    expect(
      screen.getByRole("progressbar", { name: "2 von 2 Plätzen vergeben" }),
    ).toBeInTheDocument();
    const entry = screen.getByRole("link", {
      name: /Platz 1 der Warteliste: Tom Platz/,
    });
    expect(entry.getAttribute("href")).toContain(
      "/traeger/ferien/7/anmeldungen/14",
    );
    fireEvent.click(
      screen.getByRole("button", { name: /Plätze für die Woche .* ändern/ }),
    );
    expect(onChange).toHaveBeenCalledWith(FULL_WEEK);
  });

  it("says that nobody waits in a week without a waiting list", () => {
    render(
      <HolidayWeekPlacesCard
        periodId="7"
        week={{ ...FULL_WEEK, capacity: null, waitlist: [] }}
        hostSchoolName="OGS am Berg"
        onChangeCapacity={vi.fn()}
      />,
    );
    expect(
      screen.getByText("Niemand steht auf der Warteliste."),
    ).toBeInTheDocument();
    expect(screen.getByText("Ohne Grenze")).toBeInTheDocument();
  });
});

describe("HolidayCapacityModal", () => {
  beforeEach(() => {
    mockSetCapacity.mockReset();
  });

  it("saves a higher capacity, tells who moved up and warns before taking no place away", async () => {
    mockSetCapacity.mockResolvedValue({ movedUp: ["14"] });
    const onSaved = vi.fn();
    render(
      <ToastProvider>
        <HolidayCapacityModal
          periodId="7"
          week={FULL_WEEK}
          onClose={vi.fn()}
          onSaved={onSaved}
        />
      </ToastProvider>,
    );
    expect(screen.queryByText(/1 Kind wartet auf der Warteliste/)).toBeNull();
    expect(
      screen.getByRole("textbox", { name: "Plätze in dieser Woche" }),
    ).toHaveValue("2");
    fireEvent.click(
      screen.getByRole("button", { name: "Einen Platz weniger" }),
    );
    expect(
      screen.getByText(/2 Kinder haben schon einen Platz\. Sie behalten ihn\./),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Einen Platz mehr" }));
    fireEvent.click(screen.getByRole("button", { name: "Einen Platz mehr" }));
    expect(
      screen.getByText(/1 Kind wartet auf der Warteliste/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Plätze speichern" }));
    await waitFor(() => expect(onSaved).toHaveBeenCalledWith(1));
    expect(mockSetCapacity).toHaveBeenCalledWith("7", "51", 3);
  });

  it("lifts the limit with the switch", async () => {
    mockSetCapacity.mockResolvedValue({ movedUp: [] });
    render(
      <ToastProvider>
        <HolidayCapacityModal
          periodId="7"
          week={FULL_WEEK}
          onClose={vi.fn()}
          onSaved={vi.fn()}
        />
      </ToastProvider>,
    );
    fireEvent.click(screen.getByRole("switch", { name: /Plätze begrenzen/ }));
    expect(
      screen.queryByRole("button", { name: "Einen Platz mehr" }),
    ).toBeNull();
    expect(
      screen.getByRole("switch", { name: /Plätze begrenzen/ }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Plätze speichern" }));
    await waitFor(() =>
      expect(mockSetCapacity).toHaveBeenCalledWith("7", "51", null),
    );
  });
});

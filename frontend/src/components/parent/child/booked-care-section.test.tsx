import { render, screen } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.json";
import { getChildCareOfferings, getChildCareSchedule } from "~/lib/parent-api";
import { BookedCareSection } from "./booked-care-section";

vi.mock("~/lib/parent-api", () => ({
  getChildCareOfferings: vi.fn(),
  getChildCareSchedule: vi.fn(),
  submitOfferingChangeRequest: vi.fn(),
  withdrawOfferingChangeRequest: vi.fn(),
}));

vi.mock("~/lib/hooks/use-messages-activity", () => ({
  useMessagesActivity: () => undefined,
}));

vi.mock("~/components/parent/offering-change-request-modal", () => ({
  OfferingChangeRequestModal: () => <div data-testid="offering-modal" />,
}));

const mockedOfferings = vi.mocked(getChildCareOfferings);
const mockedSchedule = vi.mocked(getChildCareSchedule);

function renderSection() {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <BookedCareSection studentId="42" />
    </NextIntlClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockedOfferings.mockResolvedValue({
    offerings: [
      {
        id: "o1",
        name: "Ganztag bis 16 Uhr",
        weekdays: [1, 2, 3, 4, 5],
        includes_lunch: true,
        includes_holiday_care: false,
      },
    ],
    groups: [{ id: "g1", name: "Fußball-AG", weekdays: [3] }],
    can_request: false,
  } as unknown as Awaited<ReturnType<typeof getChildCareOfferings>>);
  mockedSchedule.mockResolvedValue({
    weekdays: [
      { weekday: 1, arrival: "08:00", pickup: "16:00", modes: ["pickup"] },
    ],
    can_request: false,
    request_capabilities: {
      arrival: false,
      pickup: false,
      departure_mode: false,
    },
    today_absent: false,
  } as unknown as Awaited<ReturnType<typeof getChildCareSchedule>>);
});

describe("BookedCareSection", () => {
  it("zeigt die gebuchte Betreuung mit Wochentagen", async () => {
    renderSection();
    expect(await screen.findByText("Ganztag bis 16 Uhr")).toBeInTheDocument();
    expect(screen.getByText("Mo, Di, Mi, Do, Fr")).toBeInTheDocument();
  });

  // #2303: der Block irritiert, Eltern konnten dort ohnehin nichts anmelden.
  it("zeigt keine AGs und Gruppen mehr", async () => {
    renderSection();
    await screen.findByText("Ganztag bis 16 Uhr");
    expect(screen.queryByText("Fußball-AG")).not.toBeInTheDocument();
    expect(screen.queryByText(/AGs und Gruppen/)).not.toBeInTheDocument();
  });

  // #2250: die aenderbare Ankunftszeit war die Ursache falscher Elternaenderungen.
  it("zeigt die Bringzeit als reine Anzeige, nie als Eingabefeld", async () => {
    renderSection();
    expect(await screen.findByText("08:00")).toBeInTheDocument();
    expect(document.querySelectorAll("input")).toHaveLength(0);
    expect(
      screen.getByText(/Die Bringzeit ergibt sich aus dem Stundenplan/),
    ).toBeInTheDocument();
  });
});

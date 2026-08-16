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

function pendingSchedule() {
  return {
    weekdays: [
      {
        weekday: 1,
        status: "scheduled",
        arrival: "08:00",
        pickup: "16:00",
        modes: ["pickup"],
      },
      {
        weekday: 2,
        status: "not_scheduled",
        modes: [],
      },
    ],
    can_request: true,
    request_capabilities: {
      arrival: false,
      pickup: true,
      departure_mode: true,
    },
    pending_request: {
      id: "r1",
      created_at: "2026-08-16T08:00:00Z",
      diff: [],
      submitted_by_self: true,
    },
    today_absent: false,
  } as unknown as Awaited<ReturnType<typeof getChildCareSchedule>>;
}

function renderSection() {
  return render(
    <NextIntlClientProvider locale="de" messages={deMessages}>
      <BookedCareSection studentId="42" childFirstName="Hannah" />
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
      {
        weekday: 1,
        status: "scheduled",
        arrival: "08:00",
        pickup: "16:00",
        modes: ["pickup"],
      },
      {
        weekday: 2,
        status: "not_scheduled",
        modes: [],
      },
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
    expect(
      screen.getByRole("heading", { name: "Gebuchte Betreuung" }),
    ).toHaveClass("text-xl", "min-h-9", "items-center", "sm:min-h-10");
    expect(screen.getByText("Mo, Di, Mi, Do, Fr")).toBeInTheDocument();
  });

  it("trennt Betreuungsangebote von der allgemeinen OGS-Anmeldung", async () => {
    mockedOfferings.mockResolvedValue({
      offerings: [],
      groups: [],
      can_request: true,
    } as unknown as Awaited<ReturnType<typeof getChildCareOfferings>>);

    renderSection();

    expect(
      await screen.findByRole("heading", { name: "Gebuchte Betreuung" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Der Wochenplan ist hinterlegt."),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Betreuung ändern" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Ihre Buchungen")).not.toBeInTheDocument();
  });

  it("zeigt fehlende Änderungsgrundlage nur in der Betreuungskarte", async () => {
    mockedOfferings.mockResolvedValue({
      offerings: [],
      groups: [],
      can_request: false,
      changes_disabled_reason: "no_enrollment",
    } as unknown as Awaited<ReturnType<typeof getChildCareOfferings>>);

    renderSection();

    expect(
      await screen.findByText("Der Wochenplan ist hinterlegt."),
    ).toBeInTheDocument();
    expect(
      screen.getAllByRole("heading", { name: "Gebuchte Betreuung" }),
    ).toHaveLength(1);
    expect(
      screen.queryByRole("heading", {
        name: "Anfragen zur gebuchten Betreuung",
      }),
    ).not.toBeInTheDocument();
  });

  it("widerspricht einem gebuchten Wochenplan nicht mit einer fehlenden Anmeldung", async () => {
    mockedOfferings.mockResolvedValue({
      offerings: [],
      groups: [],
      can_request: false,
      changes_disabled_reason: "no_enrollment",
    } as unknown as Awaited<ReturnType<typeof getChildCareOfferings>>);

    renderSection();

    expect(
      await screen.findByText("Hannah geht in die OGS"),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/keine Anmeldung hinterlegt/i),
    ).not.toBeInTheDocument();

    const emptyStateTitle = screen.getByText("Der Wochenplan ist hinterlegt.");
    expect(
      screen.getByText(
        "Hannah besucht die OGS zu den oben angegebenen Zeiten. Weitere Buchungsdetails sind nicht hinterlegt.",
      ),
    ).toBeInTheDocument();
    expect(emptyStateTitle.parentElement).toHaveClass(
      "flex-col",
      "items-center",
      "text-center",
      "border-t",
    );
    expect(
      emptyStateTitle.parentElement?.querySelector("svg"),
    ).toBeInTheDocument();
  });

  it("ordnet eine offene Änderung der gebuchten Betreuung unter", async () => {
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
      groups: [],
      can_request: false,
      pending_request: {
        id: "p1",
        created_at: "2026-08-16T08:00:00Z",
        effective_from: "2026-09-01",
        diff: [],
        submitted_by_self: true,
      },
    } as unknown as Awaited<ReturnType<typeof getChildCareOfferings>>);

    renderSection();

    const bookedCare = await screen.findByRole("heading", {
      name: "Gebuchte Betreuung",
    });
    const requestedChange = screen.getByRole("heading", {
      name: "Beantragte Änderung",
      level: 3,
    });
    expect(bookedCare).toBeInTheDocument();
    expect(requestedChange).toBeInTheDocument();
    expect(bookedCare.closest("section")).toContainElement(requestedChange);
    expect(screen.getByText("In Prüfung")).toBeInTheDocument();
  });

  it("ordnet die letzte Entscheidung der gebuchten Betreuung unter", async () => {
    mockedOfferings.mockResolvedValue({
      offerings: [],
      groups: [],
      can_request: true,
      last_decision: {
        id: "d1",
        status: "rejected",
        decided_at: "2026-08-16T08:00:00Z",
        effective_from: "2026-09-01",
        reason: "Der Zeitraum ist ausgebucht.",
        requested: [],
      },
    } as unknown as Awaited<ReturnType<typeof getChildCareOfferings>>);

    renderSection();

    expect(
      await screen.findByRole("heading", {
        name: "Beantragte Änderung",
        level: 3,
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Anfrage abgelehnt")).toBeInTheDocument();
    expect(screen.getByText(/Der Zeitraum ist ausgebucht/)).toBeInTheDocument();
  });

  // #2303: der Block irritiert, Eltern konnten dort ohnehin nichts anmelden.
  it("zeigt keine AGs und Gruppen mehr", async () => {
    renderSection();
    await screen.findByText("Ganztag bis 16 Uhr");
    expect(screen.queryByText("Fußball-AG")).not.toBeInTheDocument();
    expect(screen.queryByText(/AGs und Gruppen/)).not.toBeInTheDocument();
  });

  it("zeigt im Wochenplan keine geplante Ankunftszeit", async () => {
    renderSection();
    await screen.findByText("Montag");

    expect(screen.queryByText("Ankunft")).not.toBeInTheDocument();
    expect(screen.queryByText("08:00 Uhr")).not.toBeInTheDocument();
    expect(
      screen.queryByText(/Diese Zeiten pflegt die OGS/),
    ).not.toBeInTheDocument();
    expect(screen.getByText("16:00 Uhr")).toBeInTheDocument();
  });

  it("trennt den Tageskopf von den Angaben", async () => {
    renderSection();

    const monday = await screen.findByText("Montag");
    expect(monday.closest("dt")).toHaveClass("flex-col", "items-start");
    expect(screen.getByText("Hannah geht in die OGS")).toBeInTheDocument();
    expect(screen.getByText("Dienstag")).toBeInTheDocument();
    expect(screen.getByText("Keine Betreuung")).toBeInTheDocument();
    expect(screen.getByText("16:00 Uhr")).toBeInTheDocument();
  });

  it("bietet im Wochenplan keine dauerhafte Änderungsanfrage an", async () => {
    mockedSchedule.mockResolvedValue({
      weekdays: [
        {
          weekday: 1,
          status: "scheduled",
          arrival: "08:00",
          pickup: "16:00",
          modes: ["bus"],
        },
      ],
      can_request: true,
      request_capabilities: {
        arrival: true,
        pickup: true,
        departure_mode: true,
      },
      today_absent: false,
    } as unknown as Awaited<ReturnType<typeof getChildCareSchedule>>);

    renderSection();
    await screen.findByText("Montag");
    expect(
      screen.queryByRole("button", { name: "Änderungen anfragen" }),
    ).not.toBeInTheDocument();
  });

  it("zeigt alte offene Wochenplananfragen nicht als Elternaktion", async () => {
    mockedSchedule.mockResolvedValue(pendingSchedule());
    renderSection();

    await screen.findByText("Montag");
    expect(screen.queryByText("In Prüfung")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Anfrage zurückziehen" }),
    ).not.toBeInTheDocument();
  });
});

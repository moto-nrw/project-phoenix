import { render, screen, within } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import deMessages from "~/i18n/messages/de.generated";
import { listHolidayOffers, type HolidayOffer } from "~/lib/parent-api";
import { setTestClock } from "~/test/clock";
import { IsolatedSWRCache } from "~/lib/swr";
import { HolidayOffers } from "./holiday-offers";

// The real formatter: the deadline counts days with a plural.
vi.unmock("next-intl");
vi.unmock("swr");
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => null,
  useTenantSafe: () => null,
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "parent-test", scope: "parent" } },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/parent-api", () => ({
  listHolidayOffers: vi.fn(),
}));

const mockedOffers = vi.mocked(listHolidayOffers);

// The frozen test clock is Wednesday 2026-09-09 in Berlin.
const winter: HolidayOffer = {
  token: "winter-key",
  name: "Weihnachtsferien 2026",
  traeger_name: "Gemeinde Bissendorf",
  registration_opens_on: "2026-09-01",
  registration_closes_on: "2026-09-30",
  first_day: "2027-01-04",
  last_day: "2027-01-08",
  open: true,
  children: [
    { student_id: "42", first_name: "Mia" },
    { student_id: "43", first_name: "Ben" },
  ],
};

function renderOffers() {
  return render(
    <IsolatedSWRCache>
      <NextIntlClientProvider locale="de" messages={deMessages}>
        <h1>Startseite</h1>
        <HolidayOffers />
      </NextIntlClientProvider>
    </IsolatedSWRCache>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("HolidayOffers", () => {
  it("names the period, the care days, the deadline and the children and leads to the form", async () => {
    mockedOffers.mockResolvedValue([winter]);
    renderOffers();

    const card = await screen.findByTestId("parent-holiday-offer");
    expect(
      within(card).getByRole("heading", { name: "Weihnachtsferien 2026" }),
    ).toBeInTheDocument();
    expect(within(card).getByText("Anmeldung offen")).toBeInTheDocument();
    expect(
      within(card).getByText("Angebot von Gemeinde Bissendorf"),
    ).toBeInTheDocument();
    expect(
      within(card).getByText("04.01.2027 bis 08.01.2027"),
    ).toBeInTheDocument();
    expect(within(card).getByText("bis 30.09.2026")).toBeInTheDocument();
    expect(within(card).getByText("Mia und Ben")).toBeInTheDocument();
    expect(
      within(card).getByRole("link", {
        name: "Weihnachtsferien 2026: Jetzt anmelden",
      }),
    ).toHaveAttribute("href", "/ferienanmeldung/winter-key");
  });

  it("names the start instead of a button before the registration opens", async () => {
    mockedOffers.mockResolvedValue([
      { ...winter, registration_opens_on: "2026-09-16", open: false },
    ]);
    renderOffers();

    const card = await screen.findByTestId("parent-holiday-offer");
    expect(
      within(card).getByText("Anmeldung ab 16.09.2026"),
    ).toBeInTheDocument();
    expect(
      within(card).getByText("16.09.2026 bis 30.09.2026"),
    ).toBeInTheDocument();
    expect(within(card).queryByRole("link")).not.toBeInTheDocument();
  });

  it("counts the days once the deadline is a week away, and names the last day", async () => {
    mockedOffers.mockResolvedValue([
      { ...winter, registration_closes_on: "2026-09-12" },
    ]);
    const { unmount } = renderOffers();
    expect(await screen.findByText("noch 3 Tage")).toBeInTheDocument();
    unmount();

    mockedOffers.mockResolvedValue([
      { ...winter, registration_closes_on: "2026-09-09" },
    ]);
    renderOffers();
    expect(
      await screen.findByText("Heute ist der letzte Tag"),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Jetzt anmelden/ })).toBeVisible();
  });

  it("disappears after the deadline, also on a page left open over midnight", async () => {
    mockedOffers.mockResolvedValue([
      { ...winter, registration_closes_on: "2026-09-09" },
    ]);
    setTestClock("2026-09-10T08:00:00+02:00");
    renderOffers();

    // Anchor: the page around the card rendered and the offers were read.
    expect(
      await screen.findByRole("heading", { name: "Startseite" }),
    ).toBeInTheDocument();
    await vi.waitFor(() => expect(mockedOffers).toHaveBeenCalled());
    expect(
      screen.queryByTestId("parent-holiday-offer"),
    ).not.toBeInTheDocument();
  });

  it("shows nothing for a parent without a matching child or when the read fails", async () => {
    mockedOffers.mockResolvedValue([]);
    const { unmount } = renderOffers();
    await vi.waitFor(() => expect(mockedOffers).toHaveBeenCalledTimes(1));
    expect(screen.getByRole("heading", { name: "Startseite" })).toBeVisible();
    expect(
      screen.queryByTestId("parent-holiday-offer"),
    ).not.toBeInTheDocument();
    unmount();

    mockedOffers.mockRejectedValue(new Error("offline"));
    renderOffers();
    await vi.waitFor(() => expect(mockedOffers).toHaveBeenCalledTimes(2));
    expect(screen.getByRole("heading", { name: "Startseite" })).toBeVisible();
    expect(
      screen.queryByTestId("parent-holiday-offer"),
    ).not.toBeInTheDocument();
  });

  it("lists several periods in one card, each with its own button, and says shared facts once", async () => {
    mockedOffers.mockResolvedValue([
      winter,
      {
        ...winter,
        token: "easter-key",
        name: "Osterferien 2027",
        registration_opens_on: "2026-09-20",
        registration_closes_on: "2026-10-20",
        first_day: "2027-03-29",
        last_day: "2027-04-02",
        open: false,
      },
    ]);
    renderOffers();

    const card = await screen.findByTestId("parent-holiday-offer");
    expect(
      within(card).getByRole("heading", { name: "Ferienbetreuung" }),
    ).toBeInTheDocument();
    expect(
      within(card).getAllByText(
        "Für Mia und Ben · Angebot von Gemeinde Bissendorf",
      ),
    ).toHaveLength(1);
    const rows = within(card).getAllByRole("listitem");
    expect(rows).toHaveLength(2);
    expect(
      within(rows[0]!).getByText("Betreuung 04.01.2027 bis 08.01.2027"),
    ).toBeInTheDocument();
    expect(
      within(rows[0]!).getByRole("link", {
        name: "Weihnachtsferien 2026: Jetzt anmelden",
      }),
    ).toHaveAttribute("href", "/ferienanmeldung/winter-key");
    expect(
      within(rows[1]!).getByText("Anmeldung ab 20.09.2026"),
    ).toBeInTheDocument();
    expect(within(rows[1]!).queryByRole("link")).not.toBeInTheDocument();
  });
});

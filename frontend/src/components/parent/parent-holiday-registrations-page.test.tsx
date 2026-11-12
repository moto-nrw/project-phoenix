import { fireEvent, render, screen, within } from "@testing-library/react";
import { NextIntlClientProvider } from "next-intl";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import deMessages from "~/i18n/messages/de.generated";
import { ApiError } from "~/lib/api-error";
import { listHolidayRegistrationEntries } from "~/lib/parent-api";
import { holidayRegistrationEntriesFromBackend } from "~/lib/parent-holiday-registration-entries";
import { IsolatedSWRCache } from "~/lib/swr";
import { catalogText } from "~/test/error-catalog-text";
import { PARENT_HOLIDAY_REGISTRATION_ENTRIES } from "~/test/fixtures/holiday-registrations";
import { ParentHolidayRegistrationsPage } from "./parent-holiday-registrations-page";

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
  listHolidayRegistrationEntries: vi.fn(),
}));

const mockedEntries = vi.mocked(listHolidayRegistrationEntries);
const ENTRIES = holidayRegistrationEntriesFromBackend(
  PARENT_HOLIDAY_REGISTRATION_ENTRIES,
);

function renderPage() {
  return render(
    <IsolatedSWRCache>
      <NextIntlClientProvider locale="de" messages={deMessages}>
        <ToastProvider>
          <ParentHolidayRegistrationsPage />
        </ToastProvider>
      </NextIntlClientProvider>
    </IsolatedSWRCache>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
});

describe("ParentHolidayRegistrationsPage (#6673)", () => {
  it("shows every registration with its state, the weeks and who acts next", async () => {
    mockedEntries.mockResolvedValue(ENTRIES);
    renderPage();

    const received = await screen.findByRole("region", {
      name: "Weihnachtsferien 2026/27",
    });
    expect(within(received).getByText("Eingegangen")).toBeInTheDocument();
    expect(
      within(received).getByText(
        "Träger Bissendorf · 04.01.2027 bis 08.01.2027",
      ),
    ).toBeInTheDocument();
    expect(
      within(received).getByText("Eingegangen am 08.09.2026"),
    ).toBeInTheDocument();
    expect(
      within(received).getByText(
        "Der Träger prüft Ihre Anmeldung und schreibt Ihnen.",
      ),
    ).toBeInTheDocument();
    expect(
      within(received).getByRole("heading", { name: "Mia" }),
    ).toBeInTheDocument();
    expect(
      within(received).getByText("04.01.2027 bis 08.01.2027 · bis 16 Uhr"),
    ).toBeInTheDocument();
    // The same words as the receipt after sending (#4699).
    expect(within(received).getByText("Warteliste")).toBeInTheDocument();
    expect(
      within(received).getByText(/dort steht Ihr Kind auf der Warteliste/i),
    ).toBeInTheDocument();
    // Anker: the received card is rendered; it has no way to the documents.
    expect(
      within(received).queryByRole("link", { name: "Zu den Dokumenten" }),
    ).not.toBeInTheDocument();

    const confirmed = screen.getByRole("region", { name: "Herbstferien 2026" });
    expect(within(confirmed).getByText("Bestätigt")).toBeInTheDocument();
    expect(within(confirmed).getAllByText("Platz reserviert")).toHaveLength(2);
    expect(
      within(confirmed).getByRole("heading", { name: "Ben" }),
    ).toBeInTheDocument();
    expect(
      within(confirmed).getByRole("link", { name: "Zu den Dokumenten" }),
    ).toHaveAttribute("href", "/documents");

    const cancelled = screen.getByRole("region", { name: "Sommerferien 2026" });
    expect(within(cancelled).getByText("Storniert")).toBeInTheDocument();
    // Anker: the cancelled card lists its week, without a place that no
    // longer counts.
    expect(
      within(cancelled).getByText("20.07.2026 bis 24.07.2026 · bis 16 Uhr"),
    ).toBeInTheDocument();
    expect(within(cancelled).queryByText("Warteliste")).not.toBeInTheDocument();
  });

  it("explains where registrations come from when there is none, without a create button", async () => {
    mockedEntries.mockResolvedValue([]);
    renderPage();

    // Anker: the empty state of the loaded page names where entries come from.
    expect(
      await screen.findByText(/Noch keine Ferienanmeldungen/),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/mit der E-Mail-Adresse dieses Kontos an/),
    ).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("shows a failed load with a retry that loads again", async () => {
    mockedEntries.mockRejectedValueOnce(
      new ApiError("boom", 500, { code: "general.server" }),
    );
    renderPage();

    expect(
      await screen.findByText(
        catalogText("general.server", "Ihre Ferienanmeldungen"),
        { exact: false },
      ),
    ).toBeInTheDocument();
    mockedEntries.mockResolvedValueOnce(ENTRIES);
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(
      await screen.findByRole("region", { name: "Herbstferien 2026" }),
    ).toBeInTheDocument();
  });
});

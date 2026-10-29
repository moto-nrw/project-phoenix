import { render, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { HolidayGuestSheet } from "~/lib/holiday-host-api";
import { SWRConfig } from "~/lib/swr";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { HolidayGuestSection } from "./holiday-guest-section";

vi.unmock("swr");

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    status: "authenticated",
    data: { user: { id: "1", token: "test-token" } },
  }),
}));

const { sheetMock } = vi.hoisted(() => ({ sheetMock: vi.fn() }));

vi.mock("~/lib/holiday-host-api", () => ({
  fetchHolidayGuestSheet: sheetMock,
}));

function renderSection(ui: ReactElement) {
  return render(
    <ToastProvider>
      <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
        {ui}
      </SWRConfig>
    </ToastProvider>,
  );
}

const sheet: HolidayGuestSheet = {
  studentId: "91",
  startsOn: "2026-09-14",
  endsOn: "2026-09-18",
  timeModelName: "bis 13 Uhr",
  timeModelStartsAt: "08:00",
  timeModelEndsAt: "13:00",
  homeSchoolId: null,
  guestSchoolName: "Grundschule Melle",
  emergencyContacts: [
    { name: "Eva Muster", phone: "0151 2345678", relationship: "Eltern" },
    { name: "Oma Muster", phone: "05402 1234", relationship: "Großmutter" },
  ],
  pickupContacts: [{ name: "Opa Muster", phone: "", relationship: "" }],
  readOnly: false,
};

describe("HolidayGuestSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useTenantSlugSafe).mockReturnValue("test-tenant");
  });

  it("shows only care, emergency contacts and pickup persons", async () => {
    sheetMock.mockResolvedValue(sheet);

    renderSection(<HolidayGuestSection studentId="91" canRead />);

    expect(
      await screen.findByText("bis 13 Uhr (08:00 bis 13:00 Uhr)"),
    ).toBeInTheDocument();
    expect(screen.getByText("14.09.2026 bis 18.09.2026")).toBeInTheDocument();
    expect(
      screen.getByText("Ohne Schule beim Träger (Grundschule Melle)"),
    ).toBeInTheDocument();
    expect(screen.getByText("Eva Muster (Eltern)")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "0151 2345678" })).toHaveAttribute(
      "href",
      "tel:01512345678",
    );
    expect(screen.getByText("Oma Muster (Großmutter)")).toBeInTheDocument();
    expect(
      screen.getByText("Außer den Eltern dürfen abholen:"),
    ).toBeInTheDocument();
    expect(screen.getByText("Opa Muster")).toBeInTheDocument();
    expect(sheetMock).toHaveBeenCalledWith("91");
  });

  it("says the week is over during the read-only days", async () => {
    sheetMock.mockResolvedValue({
      ...sheet,
      readOnly: true,
      pickupContacts: [],
    });

    renderSection(<HolidayGuestSection studentId="91" canRead />);

    expect(
      await screen.findByText(
        "Die Ferienwoche ist vorbei. Die Angaben bleiben noch 7 Tage zum Nachlesen und werden dann gelöscht.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Nur die Eltern dürfen das Kind abholen."),
    ).toBeInTheDocument();
  });

  it("reads nothing without the right to read the children's details", () => {
    renderSection(<HolidayGuestSection studentId="91" canRead={false} />);

    expect(
      screen.getByText(
        "Notfall und Abholung sieht, wer die Angaben der Kinder lesen darf. Bitte fragen Sie Ihre Leitung.",
      ),
    ).toBeInTheDocument();
    expect(sheetMock).not.toHaveBeenCalled();
  });

  it("hides the cached contacts once a reload refuses the sheet", async () => {
    const cache = new Map();
    const renderCached = () =>
      render(
        <ToastProvider>
          <SWRConfig value={{ provider: () => cache, dedupingInterval: 0 }}>
            <HolidayGuestSection studentId="91" canRead />
          </SWRConfig>
        </ToastProvider>,
      );
    sheetMock.mockResolvedValueOnce(sheet);
    const first = renderCached();
    expect(await screen.findByText("Eva Muster (Eltern)")).toBeInTheDocument();
    first.unmount();

    sheetMock.mockRejectedValue(
      new ApiError("holiday guest not found", 404, {
        code: "enrollment.holiday_guest_not_found",
      }),
    );
    renderCached();

    // Anker: der Ladefehler steht da.
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(screen.queryByText("Eva Muster (Eltern)")).not.toBeInTheDocument();
  });
});

import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type { ReactElement } from "react";
import { SWRConfig, type Cache } from "swr";
import { beforeEach, describe, expect, it, vi } from "vitest";

import FerienwochenPage from "./page";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  HolidayParticipantList,
  HostedHolidayWeek,
} from "~/lib/holiday-host-api";
import { useTenantSlugSafe } from "~/lib/tenant-context";

vi.unmock("swr");

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    status: "authenticated",
    data: { user: { token: "test-token" } },
  }),
}));

const { weeksMock, participantsMock, downloadMock } = vi.hoisted(() => ({
  weeksMock: vi.fn(),
  participantsMock: vi.fn(),
  downloadMock: vi.fn(),
}));

vi.mock("~/lib/holiday-host-api", () => ({
  fetchHostedHolidayWeeks: weeksMock,
  fetchHolidayParticipants: participantsMock,
  downloadHolidayParticipants: downloadMock,
}));

vi.mock("~/lib/hooks/use-require-permission", () => ({
  useRequirePermission: () => ({ isReady: true, isLoading: false }),
}));

function renderPage(
  ui: ReactElement = <FerienwochenPage />,
  cache: Cache = new Map(),
) {
  return render(
    <ToastProvider>
      <SWRConfig value={{ provider: () => cache, dedupingInterval: 0 }}>
        {ui}
      </SWRConfig>
    </ToastProvider>,
  );
}

/** The desktop table whose header row names `header`. */
function tableWith(header: string): HTMLElement {
  const table = screen
    .getAllByRole("table")
    .find((candidate) =>
      within(candidate).queryByText(header, { selector: "th, th *" }),
    );
  if (!table) throw new Error(`no table with the column ${header}`);
  return table;
}

const firstWeek: HostedHolidayWeek = {
  id: "51",
  periodName: "Sommerferien 2026",
  startsOn: "2026-09-14",
  endsOn: "2026-09-18",
  participants: 2,
};

const secondWeek: HostedHolidayWeek = {
  id: "52",
  periodName: "Sommerferien 2026",
  startsOn: "2026-09-21",
  endsOn: "2026-09-25",
  participants: 1,
};

function listOf(week: HostedHolidayWeek): HolidayParticipantList {
  return {
    week,
    timeModels: [
      {
        id: "61",
        name: "bis 13 Uhr",
        startsAt: "08:00",
        endsAt: "13:00",
        count: 1,
      },
      {
        id: "62",
        name: "bis 16 Uhr",
        startsAt: "08:00",
        endsAt: "16:00",
        count: 1,
      },
    ],
    homeSchools: [
      { schoolId: "31", name: "OGS Nord", count: 1 },
      { schoolId: null, name: "Feriengäste", count: 1 },
    ],
    participants: [
      {
        childId: "1",
        firstName: "Ben",
        lastName: "Muster",
        birthDate: "2016-09-12",
        homeSchoolId: null,
        homeSchoolName: "Feriengast (Grundschule Melle)",
        timeModelId: "61",
      },
      {
        childId: "2",
        firstName: "Lia",
        lastName: "Muster",
        birthDate: "2018-03-01",
        homeSchoolId: "31",
        homeSchoolName: "OGS Nord",
        timeModelId: "62",
      },
    ],
  };
}

describe("FerienwochenPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useTenantSlugSafe).mockReturnValue("test-tenant");
    downloadMock.mockResolvedValue(undefined);
  });

  it("shows the confirmed children of the hosted week with their counts", async () => {
    weeksMock.mockResolvedValue([firstWeek]);
    participantsMock.mockResolvedValue(listOf(firstWeek));

    renderPage();

    await screen.findByText("Muster, Lia", { selector: "td, td *" });
    const children = tableWith("Geburtsdatum");
    expect(within(children).getByText("Muster, Lia")).toBeInTheDocument();
    expect(within(children).getByText("01.03.2018")).toBeInTheDocument();
    expect(
      within(children).getByText("Feriengast (Grundschule Melle)"),
    ).toBeInTheDocument();
    expect(within(children).getByText("bis 16 Uhr")).toBeInTheDocument();
    const perModel = tableWith("Zeitmodell");
    expect(
      within(perModel).getByText("bis 13 Uhr (08:00 bis 13:00 Uhr)"),
    ).toBeInTheDocument();
    const perSchool = tableWith("Heimatschule");
    expect(within(perSchool).getByText("Feriengäste")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Nur zur Information. Anmeldungen prüft und bestätigt der Träger.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Sommerferien 2026 · 14.09. bis 18.09.2026 · 2 Kinder"),
    ).toBeInTheDocument();
    expect(participantsMock).toHaveBeenCalledWith("51");
  });

  it("switches between several hosted weeks", async () => {
    weeksMock.mockResolvedValue([firstWeek, secondWeek]);
    participantsMock.mockImplementation((id: string) =>
      Promise.resolve(listOf(id === "52" ? secondWeek : firstWeek)),
    );

    renderPage();

    const tab = await screen.findByRole("tab", {
      name: "21.09. bis 25.09.2026",
    });
    fireEvent.click(tab);

    expect(
      await screen.findByText(
        "Sommerferien 2026 · 21.09. bis 25.09.2026 · 2 Kinder",
      ),
    ).toBeInTheDocument();
    expect(participantsMock).toHaveBeenCalledWith("52");
  });

  it("says who plans the weeks when the school hosts none", async () => {
    weeksMock.mockResolvedValue([]);

    renderPage();

    expect(
      await screen.findByText(
        "Ihre OGS richtet gerade keine Ferienwoche aus. Die Ferienwochen plant der Träger.",
      ),
    ).toBeInTheDocument();
    expect(participantsMock).not.toHaveBeenCalled();
  });

  it("never shows the children of another school from the shared cache", async () => {
    const cache: Cache = new Map();
    weeksMock.mockResolvedValueOnce([firstWeek]);
    participantsMock.mockResolvedValueOnce(listOf(firstWeek));
    const firstSchool = renderPage(<FerienwochenPage />, cache);
    await screen.findByText("Muster, Lia", { selector: "td, td *" });
    firstSchool.unmount();

    vi.mocked(useTenantSlugSafe).mockReturnValue("other-school");
    weeksMock.mockReturnValueOnce(new Promise(() => undefined));
    renderPage(<FerienwochenPage />, cache);

    // Anker: die Seite der zweiten Schule fragt ihre eigenen Ferienwochen ab.
    await waitFor(() => expect(weeksMock).toHaveBeenCalledTimes(2));
    expect(
      screen.queryByText("Muster, Lia", { selector: "td, td *" }),
    ).not.toBeInTheDocument();
    expect(participantsMock).toHaveBeenCalledTimes(1);
  });

  it("downloads the list of the selected week", async () => {
    weeksMock.mockResolvedValue([firstWeek]);
    participantsMock.mockResolvedValue(listOf(firstWeek));

    renderPage();

    await screen.findByText("Muster, Lia", { selector: "td, td *" });
    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Als Excel herunterladen" }),
    );

    expect(downloadMock).toHaveBeenCalledWith("51", "xlsx");
  });
});

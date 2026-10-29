import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type { ReactElement } from "react";
import { SWRConfig, type Cache } from "~/lib/swr";
import { beforeEach, describe, expect, it, vi } from "vitest";

import FerienwochenPage from "./page";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  HolidayParticipantList,
  HostedHolidayWeek,
} from "~/lib/holiday-host-api";
import { useTenantSlugSafe } from "~/lib/tenant-context";
import { setTestClock } from "~/test/clock";

vi.unmock("swr");

const {
  weeksMock,
  participantsMock,
  downloadMock,
  assignMock,
  groupsMock,
  sessionPermissions,
} = vi.hoisted(() => ({
  weeksMock: vi.fn(),
  participantsMock: vi.fn(),
  downloadMock: vi.fn(),
  assignMock: vi.fn(),
  groupsMock: vi.fn(),
  sessionPermissions: { current: [] as string[] },
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    status: "authenticated",
    data: {
      user: {
        id: "1",
        token: "test-token",
        permissions: sessionPermissions.current,
      },
    },
  }),
}));

vi.mock("~/lib/holiday-host-api", () => ({
  fetchHostedHolidayWeeks: weeksMock,
  fetchHolidayParticipants: participantsMock,
  downloadHolidayParticipants: downloadMock,
  assignHolidayGuestGroup: assignMock,
}));

vi.mock("~/lib/api", () => ({
  groupService: { getGroups: groupsMock },
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
  guestGroupId: null,
  guests: 0,
};

const secondWeek: HostedHolidayWeek = {
  id: "52",
  periodName: "Sommerferien 2026",
  startsOn: "2026-09-21",
  endsOn: "2026-09-25",
  participants: 1,
  guestGroupId: null,
  guests: 0,
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
    sessionPermissions.current = [];
    groupsMock.mockResolvedValue([
      { id: "7", name: "Feriengruppe" },
      { id: "8", name: "Bären" },
    ]);
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

  it("hides the previous week's children while the chosen week loads", async () => {
    weeksMock.mockResolvedValue([firstWeek, secondWeek]);
    participantsMock.mockImplementation((id: string) =>
      id === "52"
        ? new Promise(() => undefined)
        : Promise.resolve(listOf(firstWeek)),
    );

    renderPage();

    await screen.findByText("Muster, Lia", { selector: "td, td *" });
    fireEvent.click(screen.getByRole("tab", { name: "21.09. bis 25.09.2026" }));

    // Anker: die Überschrift nennt schon die gewählte Woche.
    expect(
      await screen.findByText(
        "Sommerferien 2026 · 21.09. bis 25.09.2026 · 1 Kind",
      ),
    ).toBeInTheDocument();
    expect(participantsMock).toHaveBeenCalledWith("52");
    expect(
      screen.queryByText("Muster, Lia", { selector: "td, td *" }),
    ).not.toBeInTheDocument();
  });

  it("refreshes on focus and drops a week after its last day by itself", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    weeksMock.mockResolvedValue([firstWeek]);
    participantsMock.mockResolvedValue(listOf(firstWeek));

    renderPage();

    await screen.findByText("Muster, Lia", { selector: "td, td *" });

    const full = listOf(firstWeek);
    participantsMock.mockResolvedValue({
      ...full,
      participants: full.participants.filter((child) => child.childId !== "2"),
    });
    // SWR ignores focus for five seconds after the mount.
    await vi.advanceTimersByTimeAsync(6_000);
    fireEvent.focus(window);

    // Anker: das verbliebene Kind steht noch in der Liste.
    await waitFor(() =>
      expect(
        screen.getByText("Sommerferien 2026 · 14.09. bis 18.09.2026 · 1 Kind"),
      ).toBeInTheDocument(),
    );
    expect(
      screen.getByText("Muster, Ben", { selector: "td, td *" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Muster, Lia", { selector: "td, td *" }),
    ).not.toBeInTheDocument();

    weeksMock.mockResolvedValue([]);
    await vi.advanceTimersByTimeAsync(60_000);

    expect(
      await screen.findByText(
        "Ihre OGS richtet gerade keine Ferienwoche aus. Die Ferienwochen plant der Träger.",
      ),
    ).toBeInTheDocument();
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

  it("lets the lead take the holiday guests into a chosen group", async () => {
    sessionPermissions.current = ["config:read", "users:create"];
    weeksMock.mockResolvedValue([firstWeek]);
    participantsMock.mockResolvedValue(listOf(firstWeek));
    assignMock.mockResolvedValue(1);

    renderPage();

    expect(
      await screen.findByText(
        /Noch keine Gruppe gewählt\. Erst mit einer Gruppe/,
      ),
    ).toBeInTheDocument();
    const take = screen.getByRole("button", { name: "Feriengäste übernehmen" });
    expect(take).toBeDisabled();
    await waitFor(() =>
      expect(
        screen.getByRole("combobox", { name: "Gruppe für die Feriengäste" }),
      ).toBeEnabled(),
    );
    fireEvent.click(
      screen.getByRole("combobox", { name: "Gruppe für die Feriengäste" }),
    );
    fireEvent.click(screen.getByRole("option", { name: "Feriengruppe" }));
    weeksMock.mockResolvedValue([
      { ...firstWeek, guestGroupId: "7", guests: 1 },
    ]);
    fireEvent.click(take);

    await waitFor(() => expect(assignMock).toHaveBeenCalledWith("51", "7"));
    expect(
      await screen.findByText(
        "Ab 14.09.2026 steht 1 Feriengast in der Gruppe Feriengruppe. Neu bestätigte Kinder kommen innerhalb einer Stunde dazu.",
      ),
    ).toBeInTheDocument();
    expect(
      await screen.findByText("1 Feriengast in der Gruppe."),
    ).toBeInTheDocument();
  });

  it("shows the chosen group without a choice to a role that cannot add children", async () => {
    weeksMock.mockResolvedValue([
      { ...firstWeek, guestGroupId: "7", guests: 2 },
    ]);
    participantsMock.mockResolvedValue(listOf(firstWeek));

    renderPage();

    expect(
      await screen.findByText(
        "Ab 14.09.2026 stehen 2 Feriengäste in der Gruppe Feriengruppe. Neu bestätigte Kinder kommen innerhalb einer Stunde dazu.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Feriengäste übernehmen" }),
    ).not.toBeInTheDocument();
  });

  it("forgets an unsaved group choice when switching to another week", async () => {
    sessionPermissions.current = ["config:read", "users:create"];
    weeksMock.mockResolvedValue([firstWeek, secondWeek]);
    participantsMock.mockImplementation((id: string) =>
      Promise.resolve(listOf(id === "52" ? secondWeek : firstWeek)),
    );

    renderPage();

    await waitFor(() =>
      expect(
        screen.getByRole("combobox", { name: "Gruppe für die Feriengäste" }),
      ).toBeEnabled(),
    );
    fireEvent.click(
      screen.getByRole("combobox", { name: "Gruppe für die Feriengäste" }),
    );
    fireEvent.click(screen.getByRole("option", { name: "Feriengruppe" }));
    fireEvent.click(
      await screen.findByRole("tab", { name: "21.09. bis 25.09.2026" }),
    );

    // Anker: die Statuszeile nennt schon die zweite Woche.
    expect(
      await screen.findByText(
        "Sommerferien 2026 · 21.09. bis 25.09.2026 · 2 Kinder",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Feriengäste übernehmen" }),
    ).toBeDisabled();
    expect(assignMock).not.toHaveBeenCalled();
  });

  it("says when the groups cannot be loaded", async () => {
    sessionPermissions.current = ["config:read", "users:create"];
    groupsMock.mockRejectedValue(new Error("network down"));
    weeksMock.mockResolvedValue([firstWeek]);
    participantsMock.mockResolvedValue(listOf(firstWeek));

    renderPage();

    expect(await screen.findByRole("alert")).toHaveTextContent(/Gruppen/);
  });

  it("counts the guests in the group once the week has begun", async () => {
    setTestClock("2026-09-15T10:00:00+02:00");
    weeksMock.mockResolvedValue([
      { ...firstWeek, guestGroupId: "7", guests: 2 },
    ]);
    participantsMock.mockResolvedValue(listOf(firstWeek));

    renderPage();

    expect(
      await screen.findByText(
        "2 Feriengäste in der Gruppe Feriengruppe. Neu bestätigte Kinder kommen innerhalb einer Stunde dazu.",
      ),
    ).toBeInTheDocument();
  });
});

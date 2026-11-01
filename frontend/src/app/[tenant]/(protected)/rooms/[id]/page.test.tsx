import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useSession } from "next-auth/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import RoomDetailPage from "./page";

const {
  searchParams,
  pushMock,
  replaceMock,
  useRoomDetailMock,
  presenceModeState,
  serviceUpdateMock,
  serviceDeleteMock,
  toastSuccessMock,
  toastErrorMock,
  exportPlanMock,
  showErrorMock,
  supervisionState,
  weekendOpenState,
} = vi.hoisted(() => ({
  searchParams: new URLSearchParams(),
  pushMock: vi.fn(),
  replaceMock: vi.fn(),
  useRoomDetailMock: vi.fn(),
  presenceModeState: { mode: "detailed" as string },
  serviceUpdateMock: vi.fn(() => Promise.resolve({})),
  serviceDeleteMock: vi.fn(() => Promise.resolve(null)),
  toastSuccessMock: vi.fn(),
  toastErrorMock: vi.fn(),
  exportPlanMock: vi.fn(() => Promise.resolve()),
  showErrorMock: vi.fn(() => Promise.resolve()),
  supervisionState: { overviewEnabled: false },
  weekendOpenState: { value: false },
}));

vi.mock("next-auth/react", () => ({ useSession: vi.fn() }));

vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "7" }),
  usePathname: () => "/rooms/7",
  useRouter: () => ({ replace: replaceMock }),
  useSearchParams: () => searchParams,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: pushMock, replace: vi.fn() }),
}));

vi.mock("~/lib/supervision-context", () => ({
  useOptionalSupervision: () => ({
    overviewEnabled: supervisionState.overviewEnabled,
  }),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("~/lib/tenant-context", () => ({
  usePresenceMode: () => presenceModeState.mode,
  useWeekendFollowsFriday: () => weekendOpenState.value,
  useTenantSafe: () => null,
  useTenantSlugSafe: () => null,
  useTenantRoutingModeSafe: () => "subdomain",
}));

// Nur die Toasts ersetzen; der Fehlerweg (Katalogtexte) bleibt echt.
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: toastSuccessMock, error: toastErrorMock }),
  // Der Exportfehler des Raumplans geht als Toast über den Fehlerweg.
  useApiErrorDisplay: () => ({ show: showErrorMock }),
}));

vi.mock("~/lib/swr", () => ({
  useSWRSeedFrom: () => () => undefined,
  useTenantMutate: () => vi.fn(() => Promise.resolve()),
  useTenantMutateMatching: () => vi.fn(() => Promise.resolve()),
}));

vi.mock("~/lib/database/service-factory", () => ({
  createCrudService: () => ({
    update: serviceUpdateMock,
  }),
}));

// Löschen geht direkt an den Raum-Client: dessen ApiError trägt Code und
// Vorgangskennung in den Löschdialog (#2517).
vi.mock("~/lib/api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api")>()),
  roomService: { deleteRoom: serviceDeleteMock },
}));

vi.mock("~/components/rooms/room-detail-content", () => ({
  useRoomDetail: (roomId: string) => useRoomDetailMock(roomId),
  RoomDetailContent: ({ room }: { room: { name: string } }) => (
    <div data-testid="room-overview">{room.name}</div>
  ),
  RoomDetailSkeleton: () => <div data-testid="room-detail-skeleton" />,
}));

vi.mock("~/components/rooms/room-stammdaten-tab", () => ({
  RoomStammdatenTab: ({
    onSave,
    showOccupancy,
  }: {
    onSave: (data: { name: string }) => Promise<void>;
    showOccupancy: boolean;
  }) => (
    <div
      data-testid="room-stammdaten"
      data-show-occupancy={String(showOccupancy)}
    >
      <button type="button" onClick={() => void onSave({ name: "Neu" })}>
        Speichern
      </button>
    </div>
  ),
}));

// Der Reiter „Plan“ (#4768) hat seine eigenen Tests (room-plan-tab.test.tsx);
// hier zählt, wann die Seite ihn zeigt und was sie ihm übergibt.
vi.mock("~/components/rooms/room-plan-lazy", () => ({
  RoomPlanControls: ({
    planHref,
    planLinkLabel,
    onChange,
  }: {
    planHref: string;
    planLinkLabel: string;
    onChange: (next: { view?: string; d?: string }) => void;
  }) => (
    <div data-testid="room-plan-controls">
      <a href={planHref}>{planLinkLabel}</a>
      <button type="button" onClick={() => onChange({ view: "tag" })}>
        Tag
      </button>
    </div>
  ),
  RoomPlanTab: ({
    roomId,
    view,
    anchorISO,
  }: {
    roomId: string;
    view: string;
    anchorISO: string;
  }) => (
    <div
      data-testid="room-plan"
      data-room={roomId}
      data-view={view}
      data-day={anchorISO}
    />
  ),
}));

vi.mock("~/components/rooms/room-plan-navigation", () => ({
  parseRoomPlanView: (raw: string | null) => (raw === "tag" ? "tag" : "woche"),
  useRoomPlanHref: (
    roomId: string,
    view: string,
    day: string,
    canManage: boolean,
  ) =>
    `${canManage ? "/betreuungsplan" : "/calendar"}?room=${roomId}&d=${day}&view=${view}`,
}));

vi.mock("~/lib/plan-export-api", () => ({
  exportPlan: exportPlanMock,
}));

vi.mock("~/components/ui/confirm-delete-modal", () => ({
  ConfirmDeleteModal: ({
    isOpen,
    onConfirm,
  }: {
    isOpen: boolean;
    onConfirm: () => void;
  }) =>
    isOpen ? (
      <div data-testid="confirm-delete">
        <button type="button" onClick={onConfirm}>
          Bestätigen
        </button>
      </div>
    ) : null,
}));

vi.mock("./page-skeleton", () => ({
  RoomDetailLoadingPage: ({ backLabel }: { backLabel?: string }) => (
    <div data-testid="room-loading" data-back-label={backLabel} />
  ),
}));

const room = {
  id: "7",
  name: "Gruppenraum Sonne",
  building: "Hauptgebäude",
  floor: 1,
  category: "Gruppenraum",
  capacity: 20,
  isOccupied: true,
};

function mockSession(permissions: string[], roles: string[] = []) {
  vi.mocked(useSession).mockReturnValue({
    data: {
      user: { id: "1", token: "t", permissions, roles },
      expires: "2099-01-01",
    },
    status: "authenticated",
    update: vi.fn(),
  } as ReturnType<typeof useSession>);
}

describe("RoomDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchParams.delete("from");
    searchParams.delete("tab");
    searchParams.delete("d");
    searchParams.delete("view");
    presenceModeState.mode = "detailed";
    supervisionState.overviewEnabled = false;
    weekendOpenState.value = false;
    useRoomDetailMock.mockReturnValue({
      room,
      history: [],
      loading: false,
      error: null,
      historyDisabled: false,
    });
    mockSession(["rooms:read"]);
  });

  it("shows the overview with name, status line and status badge", () => {
    render(<RoomDetailPage />);

    expect(
      screen.getByRole("heading", { name: "Gruppenraum Sonne" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Hauptgebäude · Etage 1 · Gruppenraum · 20 Plätze"),
    ).toBeInTheDocument();
    expect(screen.getByText("Belegt")).toBeInTheDocument();
    expect(screen.getByTestId("room-overview")).toBeInTheDocument();
    // Ohne rooms:update gibt es keinen Stammdaten-Reiter und keine
    // Reiterleiste; ohne rooms:delete kein Kebab.
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Weitere Aktionen" }),
    ).not.toBeInTheDocument();
  });

  // Offene Räume stehen seit #4372 nur noch unter „Räume“: ihre gemeinsame
  // Aufsichtsansicht öffnet die Raumseite.
  it("opens the shared supervision view of an open room", () => {
    mockSession(["rooms:read", "groups:read"], ["user"]);
    useRoomDetailMock.mockReturnValue({
      room: { ...room, isOpenRoom: true },
      history: [],
      loading: false,
      error: null,
      historyDisabled: false,
    });
    render(<RoomDetailPage />);

    expect(screen.getByRole("link", { name: "Zur Aufsicht" })).toHaveAttribute(
      "href",
      "/active-supervisions?room=7",
    );
  });

  it("opens the shared supervision view for a custom staff role", () => {
    mockSession(["rooms:read", "groups:read"], ["Honorarkraft"]);
    useRoomDetailMock.mockReturnValue({
      room: { ...room, isOpenRoom: true },
      history: [],
      loading: false,
      error: null,
      historyDisabled: false,
    });
    render(<RoomDetailPage />);

    expect(screen.getByRole("link", { name: "Zur Aufsicht" })).toHaveAttribute(
      "href",
      "/active-supervisions?room=7",
    );
  });

  it("hides the supervision view from a caregiver without groups:read", () => {
    mockSession(["rooms:read"], ["user"]);
    useRoomDetailMock.mockReturnValue({
      room: { ...room, isOpenRoom: true },
      history: [],
      loading: false,
      error: null,
      historyDisabled: false,
    });
    render(<RoomDetailPage />);

    expect(screen.getByText("Belegt")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Zur Aufsicht" }),
    ).not.toBeInTheDocument();
  });

  it("offers the supervision view to staff covered by the school overview", () => {
    supervisionState.overviewEnabled = true;
    useRoomDetailMock.mockReturnValue({
      room: { ...room, isOpenRoom: true },
      history: [],
      loading: false,
      error: null,
      historyDisabled: false,
    });
    render(<RoomDetailPage />);

    expect(
      screen.getByRole("link", { name: "Zur Aufsicht" }),
    ).toBeInTheDocument();
  });

  it("offers no supervision view for a closed room or without access", () => {
    mockSession(["rooms:read"], ["user"]);
    const { unmount } = render(<RoomDetailPage />);
    // Anker: die Raumseite ist geladen.
    expect(screen.getByText("Belegt")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Zur Aufsicht" }),
    ).not.toBeInTheDocument();
    unmount();

    // Ohne Betreuungsrolle und ohne Schulübersicht sperrt die Aufsichtsseite.
    mockSession(["rooms:read"]);
    useRoomDetailMock.mockReturnValue({
      room: { ...room, isOpenRoom: true },
      history: [],
      loading: false,
      error: null,
      historyDisabled: false,
    });
    render(<RoomDetailPage />);
    expect(screen.getByText("Belegt")).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Zur Aufsicht" }),
    ).not.toBeInTheDocument();
  });

  it("offers the Stammdaten tab to editors and saves through the service", async () => {
    mockSession(["rooms:update"]);
    render(<RoomDetailPage />);

    fireEvent.click(screen.getByRole("tab", { name: "Stammdaten" }));
    expect(replaceMock).toHaveBeenCalledWith("/rooms/7?tab=stammdaten", {
      scroll: false,
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(serviceUpdateMock).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ name: "Neu" }),
      );
    });
    expect(toastSuccessMock).toHaveBeenCalled();
  });

  it("keeps the referrer when switching room tabs", () => {
    mockSession(["rooms:update"]);
    searchParams.set("from", "/database/rooms?groupBy=floor");
    render(<RoomDetailPage />);

    fireEvent.click(screen.getByRole("tab", { name: "Stammdaten" }));

    expect(replaceMock).toHaveBeenCalledWith(
      "/rooms/7?from=%2Fdatabase%2Frooms%3FgroupBy%3Dfloor&tab=stammdaten",
      { scroll: false },
    );
  });

  it("opens Stammdaten directly from a deep link", () => {
    mockSession(["rooms:update"]);
    searchParams.set("tab", "stammdaten");
    render(<RoomDetailPage />);

    expect(screen.getByTestId("room-stammdaten")).toBeInTheDocument();
    expect(screen.getByTestId("room-stammdaten")).toHaveAttribute(
      "data-show-occupancy",
      "true",
    );
    expect(screen.queryByTestId("room-overview")).not.toBeInTheDocument();
  });

  it("deletes the room after confirmation and returns to the referrer", async () => {
    mockSession(["rooms:delete"]);
    searchParams.set("from", "/database/rooms?groupBy=floor");
    render(<RoomDetailPage />);

    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    await waitFor(() => {
      expect(serviceDeleteMock).toHaveBeenCalledWith("7");
    });
    await waitFor(() => {
      expect(pushMock).toHaveBeenCalledWith("/database/rooms?groupBy=floor");
    });
  });

  it("does not offer deleting a system room", () => {
    mockSession(["rooms:delete"]);
    useRoomDetailMock.mockReturnValue({
      room: { ...room, name: "Schulhof" },
      history: [],
      loading: false,
      error: null,
      historyDisabled: false,
    });
    render(<RoomDetailPage />);

    // Anker: die Raumakte ist geladen, nur das Aktionsmenü fehlt.
    expect(screen.getAllByText("Schulhof").length).toBeGreaterThan(0);
    expect(
      screen.queryByRole("button", { name: "Weitere Aktionen" }),
    ).not.toBeInTheDocument();
  });

  it("shows only the Stammdaten in binary presence mode", () => {
    mockSession(["rooms:update"]);
    presenceModeState.mode = "binary";
    render(<RoomDetailPage />);

    expect(screen.getByTestId("room-stammdaten")).toBeInTheDocument();
    expect(screen.getByTestId("room-stammdaten")).toHaveAttribute(
      "data-show-occupancy",
      "false",
    );
    expect(screen.queryByTestId("room-overview")).not.toBeInTheDocument();
    expect(screen.queryByText("Belegt")).not.toBeInTheDocument();
  });

  it("shows read-only room information in binary presence mode", () => {
    presenceModeState.mode = "binary";
    render(<RoomDetailPage />);

    expect(screen.getByTestId("room-overview")).toBeInTheDocument();
    expect(screen.queryByTestId("room-stammdaten")).not.toBeInTheDocument();
  });

  it("falls back to the room list for an external referrer", () => {
    searchParams.set("from", "//attacker.example");
    render(<RoomDetailPage />);

    expect(
      screen.getByRole("link", { name: "Zurück zu den Räumen" }),
    ).toHaveAttribute("href", "/rooms");
  });

  it("renders the loading page with the referrer's back label", () => {
    useRoomDetailMock.mockReturnValue({
      room: null,
      history: [],
      loading: true,
      error: null,
      historyDisabled: false,
    });
    searchParams.set("from", "/database/rooms");
    render(<RoomDetailPage />);

    expect(screen.getByTestId("room-loading")).toHaveAttribute(
      "data-back-label",
      "Zurück zu den Räumen der Datenverwaltung",
    );
  });

  it("renders the error state when the room cannot be loaded", () => {
    useRoomDetailMock.mockReturnValue({
      room: null,
      history: [],
      loading: false,
      error: "Fehler beim Laden der Raumdaten.",
      historyDisabled: false,
    });
    render(<RoomDetailPage />);

    expect(
      screen.getByText("Fehler beim Laden der Raumdaten."),
    ).toBeInTheDocument();
  });
  // --- Reiter „Plan“ (#4768) ---

  it("offers the Plan tab with the read permission of the Betreuungsplan", () => {
    mockSession(["rooms:read", "schedules:read"]);
    render(<RoomDetailPage />);

    fireEvent.click(screen.getByRole("tab", { name: "Plan" }));
    expect(replaceMock).toHaveBeenCalledWith("/rooms/7?tab=plan", {
      scroll: false,
    });
  });

  it("hides the Plan tab without schedules:read", () => {
    mockSession(["rooms:read", "rooms:update"]);
    render(<RoomDetailPage />);

    expect(screen.getByRole("tab", { name: "Stammdaten" })).toBeInTheDocument();
    expect(screen.queryByRole("tab", { name: "Plan" })).not.toBeInTheDocument();
  });

  it("opens the room's plan from a deep link and links to the read-only plan", async () => {
    mockSession(["rooms:read", "schedules:read"]);
    searchParams.set("tab", "plan");
    searchParams.set("d", "2026-09-09");
    searchParams.set("view", "tag");
    render(<RoomDetailPage />);

    const plan = await screen.findByTestId("room-plan");
    expect(plan).toHaveAttribute("data-room", "7");
    expect(plan).toHaveAttribute("data-view", "tag");
    expect(plan).toHaveAttribute("data-day", "2026-09-09");
    expect(screen.queryByTestId("room-overview")).not.toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Im Betreuungsplan ansehen" }),
    ).toHaveAttribute("href", "/calendar?room=7&d=2026-09-09&view=tag");
  });

  it("sends planners to the editable Betreuungsplan of the room", () => {
    mockSession(["rooms:read", "schedules:read", "schedules:manage"]);
    searchParams.set("tab", "plan");
    render(<RoomDetailPage />);

    expect(
      screen.getByRole("link", { name: "Im Betreuungsplan bearbeiten" }),
    ).toHaveAttribute(
      "href",
      expect.stringMatching(/^\/betreuungsplan\?room=7&/),
    );
  });

  it("keeps the tab when the plan changes its day or view", () => {
    mockSession(["rooms:read", "schedules:read"]);
    searchParams.set("tab", "plan");
    render(<RoomDetailPage />);

    fireEvent.click(screen.getByRole("button", { name: "Tag" }));
    expect(replaceMock).toHaveBeenCalledWith("/rooms/7?tab=plan&view=tag", {
      scroll: false,
    });
  });

  it("drops the plan parameters when leaving the Plan tab", () => {
    mockSession(["rooms:read", "rooms:update", "schedules:read"]);
    searchParams.set("tab", "plan");
    searchParams.set("d", "2026-09-09");
    searchParams.set("view", "tag");
    render(<RoomDetailPage />);

    fireEvent.click(screen.getByRole("tab", { name: "Stammdaten" }));
    expect(replaceMock).toHaveBeenCalledWith("/rooms/7?tab=stammdaten", {
      scroll: false,
    });
  });

  it("downloads the room's plan of the shown week as PDF", async () => {
    mockSession(["rooms:read", "schedules:read", "users:read"]);
    searchParams.set("tab", "plan");
    searchParams.set("d", "2026-09-09");
    render(<RoomDetailPage />);

    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Raumplan als PDF" }));
    await waitFor(() => expect(exportPlanMock).toHaveBeenCalledTimes(1));
    expect(exportPlanMock).toHaveBeenCalledWith(
      "betreuungsplan",
      {
        from: "2026-09-09",
        to: "2026-09-09",
        template: "offerings",
        variant: "aushang",
        filter: { groupId: null, staffId: null, roomId: "7" },
      },
      "pdf",
      "download",
    );
  });

  it("exports the displayed Monday for a weekend deep link", async () => {
    mockSession(["rooms:read", "schedules:read", "users:read"]);
    searchParams.set("tab", "plan");
    searchParams.set("d", "2026-09-12");
    render(<RoomDetailPage />);

    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Raumplan als PDF" }));
    await waitFor(() =>
      expect(exportPlanMock).toHaveBeenCalledWith(
        "betreuungsplan",
        expect.objectContaining({ from: "2026-09-14", to: "2026-09-14" }),
        "pdf",
        "download",
      ),
    );
  });

  it("shows a failed room plan export as an error", async () => {
    mockSession(["rooms:read", "schedules:read", "users:read"]);
    exportPlanMock.mockRejectedValueOnce(new Error("offline"));
    searchParams.set("tab", "plan");
    render(<RoomDetailPage />);

    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Raumplan als PDF" }));
    await waitFor(() =>
      expect(showErrorMock).toHaveBeenCalledWith(
        expect.any(Error),
        expect.objectContaining({ object: "das Erstellen des Raumplans" }),
      ),
    );
  });

  it("offers no room plan PDF without users:read", () => {
    mockSession(["rooms:read", "schedules:read"]);
    searchParams.set("tab", "plan");
    render(<RoomDetailPage />);

    // Anker: der Plan ist da, das Menü fehlt mangels Einträgen.
    expect(screen.getByTestId("room-plan")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Weitere Aktionen" }),
    ).not.toBeInTheDocument();
  });

  it("shows the plan beside the room information in binary presence mode", async () => {
    presenceModeState.mode = "binary";
    mockSession(["rooms:read", "schedules:read"]);
    render(<RoomDetailPage />);

    expect(screen.getByTestId("room-overview")).toBeInTheDocument();
    expect(await screen.findByTestId("room-plan")).toBeInTheDocument();
    expect(screen.queryByRole("tab")).not.toBeInTheDocument();
  });
});

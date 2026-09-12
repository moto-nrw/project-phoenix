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

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("~/lib/tenant-context", () => ({
  usePresenceMode: () => presenceModeState.mode,
  useTenantSafe: () => null,
  useTenantSlugSafe: () => null,
  useTenantRoutingModeSafe: () => "subdomain",
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: toastSuccessMock, error: toastErrorMock }),
}));

vi.mock("~/lib/swr", () => ({
  useTenantMutate: () => vi.fn(() => Promise.resolve()),
  useTenantMutateMatching: () => vi.fn(() => Promise.resolve()),
}));

vi.mock("~/lib/database/service-factory", () => ({
  createCrudService: () => ({
    update: serviceUpdateMock,
    delete: serviceDeleteMock,
  }),
}));

vi.mock("~/components/rooms/room-detail-content", () => ({
  roomDetailKey: (id: string) => `room-detail-${id}`,
  useRoomDetail: (roomId: string) => useRoomDetailMock(roomId),
  RoomDetailContent: ({ room }: { room: { name: string } }) => (
    <div data-testid="room-overview">{room.name}</div>
  ),
  RoomDetailSkeleton: () => <div data-testid="room-detail-skeleton" />,
}));

vi.mock("~/components/rooms/room-stammdaten-tab", () => ({
  RoomStammdatenTab: ({
    onSave,
  }: {
    onSave: (data: { name: string }) => Promise<void>;
  }) => (
    <div data-testid="room-stammdaten">
      <button type="button" onClick={() => void onSave({ name: "Neu" })}>
        Speichern
      </button>
    </div>
  ),
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

function mockSession(permissions: string[]) {
  vi.mocked(useSession).mockReturnValue({
    data: {
      user: { id: "1", token: "t", permissions },
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
    presenceModeState.mode = "detailed";
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

    expect(
      screen.queryByRole("button", { name: "Weitere Aktionen" }),
    ).not.toBeInTheDocument();
  });

  it("shows only the Stammdaten in binary presence mode", () => {
    mockSession(["rooms:update"]);
    presenceModeState.mode = "binary";
    render(<RoomDetailPage />);

    expect(screen.getByTestId("room-stammdaten")).toBeInTheDocument();
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

    fireEvent.click(
      screen.getByRole("button", { name: "Zurück zu den Räumen" }),
    );

    expect(pushMock).toHaveBeenCalledWith("/rooms");
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
});

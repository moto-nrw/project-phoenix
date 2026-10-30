import type { ReactNode } from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import type { WeekProtocol } from "~/lib/week-protocols-api";

import WeekProtocolPage from "./page";

const state = vi.hoisted(() => ({
  protocol: null as WeekProtocol | null,
  saveNotes: vi.fn(),
  close: vi.fn(),
  mutate: vi.fn(() => Promise.resolve(undefined)),
}));

vi.mock("next/navigation", () => ({
  useParams: () => ({ protocolId: "41" }),
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
  usePathname: () => "/wochenprotokolle/41",
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("~/components/auth/role-guard", () => ({
  RoleGuard: ({ children }: { children: ReactNode }) => children,
}));

vi.mock("~/lib/swr", () => ({
  useTenantMutate: () => vi.fn(),
  useSWRAuth: () => ({
    data: state.protocol ?? undefined,
    error: undefined,
    isLoading: false,
    mutate: state.mutate,
  }),
}));

vi.mock("~/lib/week-protocols-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/week-protocols-api")>();
  return {
    ...actual,
    weekProtocolsService: {
      ...actual.weekProtocolsService,
      saveNotes: state.saveNotes,
      close: state.close,
      pdfUrl: (id: string) => `/api/week-protocols/${id}/pdf`,
    },
  };
});

function day(
  date: string,
  overrides: Partial<WeekProtocol["days"][number]> = {},
) {
  return {
    date,
    captured: true,
    kind: "care" as const,
    present: 9,
    sick: 1,
    excused: 0,
    classTrip: 0,
    absent: 1,
    missing: [
      { studentId: "1", name: "Mia Kurz", reason: "sick", label: "Krank" },
      { studentId: "2", name: "Ole Lang", reason: "absent", label: "Abwesend" },
    ],
    blocks: [
      {
        title: "Fußball",
        startTime: "14:00",
        endTime: "15:00",
        status: "cancelled",
        cancelReason: "Regen",
      },
    ],
    notices: [],
    note: "",
    ...overrides,
  };
}

function protocol(overrides: Partial<WeekProtocol> = {}): WeekProtocol {
  return {
    id: "41",
    scope: "group",
    groupId: "7",
    groupName: "Bären",
    weekStart: "2026-09-07",
    weekEnd: "2026-09-13",
    status: "draft",
    draftFrom: "2026-09-11",
    autoCloseOn: "2026-09-21",
    weekNote: "",
    closedAt: null,
    closedByName: "",
    closedAutomatically: false,
    days: [
      day("2026-09-07"),
      day("2026-09-08", {
        kind: "holiday",
        present: 0,
        missing: [],
        blocks: [],
      }),
      day("2026-09-09", {
        captured: false,
        kind: null,
        missing: [],
        blocks: [],
      }),
      day("2026-09-10"),
      day("2026-09-11"),
    ],
    ...overrides,
  };
}

function renderPage() {
  return render(
    <ToastProvider>
      <WeekProtocolPage />
    </ToastProvider>,
  );
}

describe("WeekProtocolPage", () => {
  beforeEach(() => {
    state.protocol = protocol();
    state.saveNotes.mockReset();
    state.close.mockReset();
    state.mutate.mockClear();
  });

  it("shows what each evening recorded and who acts next", () => {
    renderPage();
    expect(screen.getByText("Bären")).toBeInTheDocument();
    expect(screen.getByText("07.09. bis 13.09.2026")).toBeInTheDocument();
    expect(screen.getAllByText("Mia Kurz: Krank")).toHaveLength(3);
    expect(
      screen.getAllByText("14:00 bis 15:00 Fußball: Ausgefallen: Regen"),
    ).toHaveLength(3);
    expect(screen.getByText("Feiertag")).toBeInTheDocument();
    expect(
      screen.getByText("Dieser Tag wurde nicht festgehalten."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Sonst schließt moto es am 21\.09\.2026/),
    ).toBeInTheDocument();
    expect(screen.queryByText("PDF herunterladen")).not.toBeInTheDocument();
  });

  it("saves the notes of the days and the week", async () => {
    state.saveNotes.mockResolvedValue(protocol({ weekNote: "Ruhig" }));
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Ergänzen" }));
    fireEvent.change(screen.getByLabelText("Ergänzung für Montag, 07.09."), {
      target: { value: "Elternabend" },
    });
    fireEvent.change(screen.getByLabelText("Ergänzung für die Woche"), {
      target: { value: "Ruhig" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(state.saveNotes).toHaveBeenCalledWith("41", {
        weekNote: "Ruhig",
        days: {
          "2026-09-07": "Elternabend",
          "2026-09-08": "",
          "2026-09-09": "",
          "2026-09-10": "",
          "2026-09-11": "",
        },
      }),
    );
    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: "Speichern" }),
      ).not.toBeInTheDocument(),
    );
  });

  it("closes only after the confirmation", async () => {
    state.close.mockResolvedValue(protocol({ status: "closed" }));
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: "Abschließen" }));
    expect(state.close).not.toHaveBeenCalled();
    expect(
      screen.getByText("Wochenprotokoll abschließen?"),
    ).toBeInTheDocument();
    const buttons = screen.getAllByRole("button", { name: "Abschließen" });
    fireEvent.click(buttons[buttons.length - 1]!);
    await waitFor(() => expect(state.close).toHaveBeenCalledWith("41"));
  });

  it("offers the PDF of a closed protocol and nothing to change", () => {
    state.protocol = protocol({
      status: "closed",
      closedAt: "2026-09-12T08:30:00Z",
      closedByName: "Lea Leitung",
    });
    renderPage();
    expect(screen.getByText("PDF herunterladen").closest("a")).toHaveAttribute(
      "href",
      "/api/week-protocols/41/pdf",
    );
    expect(
      screen.queryByRole("button", { name: "Ergänzen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(/Lea Leitung hat das Protokoll/),
    ).toBeInTheDocument();
  });

  it("explains a running week without actions", () => {
    state.protocol = protocol({ status: "collecting" });
    renderPage();
    expect(
      screen.getByText(/Ab Freitagabend, 11\.09\.2026/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Abschließen" }),
    ).not.toBeInTheDocument();
  });
});

import {
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

// Exercise the real translation context supplied by the school auth guard.
vi.unmock("next-intl");
vi.mock("next/navigation", () => ({
  usePathname: () => "/school/kalender",
  redirect: vi.fn(),
}));
vi.mock("~/lib/shell-auth-context", () => ({
  SchoolShellProvider: ({ children }: { children: ReactNode }) => children,
}));
vi.mock("~/components/school/shell/school-shell", () => ({
  SchoolShell: ({ children }: { children: ReactNode }) => children,
}));
vi.mock("~/components/school/school-realtime-bridge", () => ({
  SchoolRealtimeBridge: () => null,
}));
vi.mock("~/components/tenant/pwa-install-hint", () => ({
  PwaInstallHint: () => null,
}));
vi.mock("~/components/notifications/portal-notification-setup", () => ({
  PortalNotificationSetup: () => null,
}));

vi.mock("~/components/ui/date-picker", async () =>
  (await import("~/test/mocks/date-picker")).datePickerModuleMock(),
);
vi.mock("vaul", () => import("~/test/mocks/vaul"));

const {
  mockUseSWRAuth,
  mockCreateSchoolAppointment,
  mockUpdateSchoolAppointment,
  mockRespondSchoolCalendar,
  mockToastSuccess,
  mockMutate,
} = vi.hoisted(() => ({
  mockUseSWRAuth: vi.fn(),
  mockCreateSchoolAppointment: vi.fn(),
  mockUpdateSchoolAppointment: vi.fn(),
  mockRespondSchoolCalendar: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockMutate: vi.fn(),
}));

vi.mock("~/lib/swr", () => ({ useSWRAuth: mockUseSWRAuth }));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({
    success: mockToastSuccess,
    error: vi.fn(),
    warning: vi.fn(),
  }),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { scope: "school" } },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/personal-calendar-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/personal-calendar-api")
  >("~/lib/personal-calendar-api");
  return {
    ...actual,
    getSchoolCalendar: vi.fn(),
    createSchoolAppointment: mockCreateSchoolAppointment,
    updateSchoolAppointment: mockUpdateSchoolAppointment,
    deleteSchoolAppointment: vi.fn(),
    respondSchoolCalendar: mockRespondSchoolCalendar,
  };
});

import { ToastProvider } from "~/contexts/ToastContext";
import type { SchoolCalendarResponse } from "~/lib/personal-calendar-api";

import { SchoolAuthGuard } from "../auth-guard";
import SchoolCalendarPage from "./page";

function render(ui: ReactElement) {
  return rtlRender(<SchoolAuthGuard>{ui}</SchoolAuthGuard>, {
    wrapper: ToastProvider,
  });
}

// Die Testuhr steht auf Mittwoch, 09.09.2026; alles liegt in dieser Woche.
function calendar(
  overrides: Partial<SchoolCalendarResponse> = {},
): SchoolCalendarResponse {
  return {
    from: "2026-09-07",
    to: "2026-09-13",
    events: [
      {
        id: "appointment:3:2026-09-09",
        source: "appointment",
        appointment_id: "3",
        title: "Zoo-Ausflug",
        start_date: "2026-09-09",
        end_date: "2026-09-09",
        start_time: "08:30",
        end_time: "12:00",
        all_day: false,
        school_event_type: "ausflug",
        can_respond: false,
        can_edit: true,
      },
      {
        id: "appointment:8:2026-09-10",
        source: "appointment",
        appointment_id: "8",
        title: "Sommerfest der OGS",
        start_date: "2026-09-10",
        end_date: "2026-09-10",
        start_time: "15:00",
        end_time: "17:00",
        all_day: false,
        response_status: "pending",
        recipient_id: "42",
        can_respond: true,
        can_edit: false,
      },
    ],
    closing_days: [
      {
        id: "4",
        name: "Brückentag",
        start_date: "2026-09-11",
        end_date: "2026-09-11",
      },
    ],
    holidays: [],
    classes: ["3b"],
    ...overrides,
  };
}

function mockCalendar(data: SchoolCalendarResponse) {
  mockUseSWRAuth.mockImplementation(() => ({
    data,
    error: null,
    isLoading: false,
    mutate: mockMutate,
  }));
}

function openCreate() {
  fireEvent.click(screen.getByRole("button", { name: "Neuer Termin" }));
  fireEvent.change(screen.getByLabelText("Titel"), {
    target: { value: "Wandertag" },
  });
}

describe("SchoolCalendarPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCalendar(calendar());
    mockCreateSchoolAppointment.mockResolvedValue({});
    mockUpdateSchoolAppointment.mockResolvedValue({});
    mockRespondSchoolCalendar.mockResolvedValue(undefined);
    mockMutate.mockResolvedValue(undefined);
  });

  it("shows appointments, closing days and the subscription", () => {
    render(<SchoolCalendarPage />);

    expect(screen.getAllByText("Zoo-Ausflug").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Brückentag").length).toBeGreaterThan(0);
    const legend = screen.getAllByLabelText("Legende Terminarten")[0]!;
    expect(within(legend).getByText("Schultermin")).toBeInTheDocument();
    expect(within(legend).getByText("Schließtag")).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Kalender abonnieren" }),
    ).toBeInTheDocument();
  });

  it("creates a Schultermin for the parents of one class", async () => {
    render(<SchoolCalendarPage />);
    openCreate();

    fireEvent.click(screen.getByLabelText("Art"));
    fireEvent.click(screen.getByRole("option", { name: "Unterrichtsende" }));
    fireEvent.click(screen.getByLabelText("Eltern meiner Klassen"));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    // Ohne Klasse geht nichts raus; der Hinweis steht an der Auswahl.
    expect(
      await screen.findByText("Bitte wählen Sie mindestens eine Klasse."),
    ).toBeInTheDocument();
    expect(mockCreateSchoolAppointment).not.toHaveBeenCalled();

    fireEvent.click(screen.getByLabelText("Klasse 3b"));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockCreateSchoolAppointment).toHaveBeenCalledWith(
        expect.objectContaining({
          title: "Wandertag",
          school_event_type: "unterrichtsende",
          start_date: "2026-09-09",
          parents: "classes",
          classes: ["3b"],
        }),
      ),
    );
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Der Termin „Wandertag“ ist gespeichert.",
    );
    await waitFor(() => expect(mockMutate).toHaveBeenCalled());
  });

  it("explains why the own classes cannot be chosen", () => {
    mockCalendar(calendar({ classes: [] }));
    render(<SchoolCalendarPage />);
    openCreate();

    expect(screen.getByLabelText(/Eltern meiner Klassen/)).toBeDisabled();
    expect(
      screen.getByText("Ihnen ist keine Klasse zugeordnet."),
    ).toBeInTheDocument();
  });

  it("edits an own Schultermin without asking for parents again", async () => {
    render(<SchoolCalendarPage />);

    fireEvent.click(screen.getAllByRole("button", { name: /Zoo-Ausflug/ })[0]!);
    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));

    expect(
      screen.getByText(/Wer ihn sieht, bleibt beim Ändern gleich/),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Keine Eltern")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockUpdateSchoolAppointment).toHaveBeenCalledWith(
        "3",
        expect.objectContaining({
          title: "Zoo-Ausflug",
          school_event_type: "ausflug",
        }),
      ),
    );
  });

  it("answers an OGS invitation", async () => {
    render(<SchoolCalendarPage />);

    fireEvent.click(
      screen.getAllByRole("button", { name: /Sommerfest der OGS/ })[0]!,
    );
    fireEvent.click(screen.getByRole("button", { name: "Zusagen" }));

    await waitFor(() =>
      expect(mockRespondSchoolCalendar).toHaveBeenCalledWith("42", "accepted"),
    );
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Ihre Zusage ist gespeichert.",
    );
  });
});

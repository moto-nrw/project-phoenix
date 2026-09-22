/**
 * Tests for the operator billing page (#2791): key day, monthly key-date
 * counts with totals, CSV downloads and the empty state.
 */
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
// eslint-disable-next-line no-restricted-imports -- operator pages are not tenant-scoped
import { SWRConfig } from "swr";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  BillingKeyDateCount,
  BillingKeyDay,
} from "~/lib/operator/billing-api";

const {
  mockGetKeyDay,
  mockUpdateKeyDay,
  mockListKeyDateCounts,
  mockDownload,
  mockToastSuccess,
  mockToastError,
} = vi.hoisted(() => ({
  mockGetKeyDay: vi.fn(),
  mockUpdateKeyDay: vi.fn(),
  mockListKeyDateCounts: vi.fn(),
  mockDownload: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
}));

// The page's loading, empty and saved states come from SWR itself, so this
// file runs the real library over a fresh cache per test.
vi.unmock("swr");

vi.mock("next-auth/react", () => ({
  useSession: () => ({ status: "authenticated", data: { user: {} } }),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mockToastSuccess, error: mockToastError }),
}));

vi.mock("~/lib/operator/billing-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/operator/billing-api")
  >("~/lib/operator/billing-api");
  return {
    ...actual,
    operatorBillingService: {
      getKeyDay: mockGetKeyDay,
      updateKeyDay: mockUpdateKeyDay,
      listKeyDateCounts: mockListKeyDateCounts,
      downloadKeyDateCounts: mockDownload,
    },
  };
});

import OperatorBillingPage from "./page";

const keyDay: BillingKeyDay = {
  keyDay: 15,
  nextKeyDate: "2026-09-15",
  updatedAt: "2026-09-01T08:00:00Z",
};

function count(
  overrides: Partial<BillingKeyDateCount> = {},
): BillingKeyDateCount {
  return {
    schoolId: "11",
    schoolName: "OGS Am Berg",
    organizationName: "Träger Nord",
    period: "2026-08-01",
    keyDate: "2026-08-15",
    activeStudents: 120,
    activeTerminals: 3,
    recordedAt: "2026-08-15T04:00:00Z",
    ...overrides,
  };
}

function renderPage() {
  return render(
    <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
      <OperatorBillingPage />
    </SWRConfig>,
  );
}

describe("OperatorBillingPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetKeyDay.mockResolvedValue(keyDay);
    mockDownload.mockResolvedValue(undefined);
  });

  it("shows the key day and the next key date", async () => {
    mockListKeyDateCounts.mockResolvedValue([]);
    renderPage();

    expect(await screen.findByText("Jeden Monat am 15.")).toBeInTheDocument();
    expect(screen.getByText("15.09.2026")).toBeInTheDocument();
  });

  it("explains when the first counts arrive while none are captured", async () => {
    mockListKeyDateCounts.mockResolvedValue([]);
    renderPage();

    expect(
      await screen.findByText("Noch keine Stichtagszahlen"),
    ).toBeInTheDocument();
    expect(
      await screen.findByText(
        "moto erfasst die Zahlen am Stichtag ab 6 Uhr. Fällt die Erfassung am Stichtag aus, holt moto sie im selben Monat nach.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Monat als CSV" }),
    ).not.toBeInTheDocument();
  });

  it("shows the newest month with its totals and only that month's rows", async () => {
    mockListKeyDateCounts.mockResolvedValue([
      count({ period: "2026-08-01", schoolId: "11", activeStudents: 120 }),
      count({
        period: "2026-08-01",
        schoolId: "12",
        schoolName: "OGS Am See",
        activeStudents: 80,
        activeTerminals: 2,
      }),
      count({
        period: "2026-07-01",
        keyDate: "2026-07-15",
        schoolId: "11",
        schoolName: "OGS Juli",
        activeStudents: 999,
        recordedAt: "2026-07-15T04:00:00Z",
      }),
    ]);
    renderPage();

    expect(await screen.findAllByText("OGS Am See")).not.toHaveLength(0);
    expect(screen.getByText("Stichtag 15.08.2026")).toBeInTheDocument();
    expect(screen.queryAllByText("OGS Juli")).toHaveLength(0);
    expect(screen.getByText("200")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
  });

  it("marks a capture that ran after the key date as late", async () => {
    mockListKeyDateCounts.mockResolvedValue([
      count({ recordedAt: "2026-08-17T07:30:00Z" }),
    ]);
    renderPage();

    // The table renders its desktop and its stacked phone layout.
    expect(
      await screen.findAllByText("17.08.2026, 09:30 Uhr · nachträglich"),
    ).not.toHaveLength(0);
  });

  it("downloads the chosen month and every month as CSV", async () => {
    mockListKeyDateCounts.mockResolvedValue([count()]);
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Monat als CSV" }),
    );
    await waitFor(() => expect(mockDownload).toHaveBeenCalledWith("2026-08"));

    fireEvent.click(
      screen.getByRole("button", { name: "Alle Monate als CSV" }),
    );
    await waitFor(() =>
      expect(mockDownload).toHaveBeenLastCalledWith(undefined),
    );
  });

  it("reports a failed download without leaving the page", async () => {
    mockListKeyDateCounts.mockResolvedValue([count()]);
    mockDownload.mockRejectedValue(new Error("boom"));
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Monat als CSV" }),
    );
    await waitFor(() => expect(mockToastError).toHaveBeenCalled());
  });

  it("changes the key day through the edit state", async () => {
    mockListKeyDateCounts.mockResolvedValue([]);
    mockUpdateKeyDay.mockResolvedValue({
      ...keyDay,
      keyDay: 10,
      nextKeyDate: "2026-10-10",
    });
    renderPage();

    fireEvent.click(await screen.findByRole("button", { name: "Ändern" }));
    const save = screen.getByRole("button", { name: "Speichern" });
    expect(save).toBeDisabled();

    fireEvent.click(screen.getByRole("combobox"));
    fireEvent.click(within(screen.getByRole("listbox")).getByText("10."));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(mockUpdateKeyDay).toHaveBeenCalledWith(10));
    expect(await screen.findByText("Jeden Monat am 10.")).toBeInTheDocument();
    expect(mockToastSuccess).toHaveBeenCalled();
  });

  it("keeps the edit state open and says so when saving fails", async () => {
    mockListKeyDateCounts.mockResolvedValue([]);
    mockUpdateKeyDay.mockRejectedValue(new Error("boom"));
    renderPage();

    fireEvent.click(await screen.findByRole("button", { name: "Ändern" }));
    fireEvent.click(screen.getByRole("combobox"));
    fireEvent.click(within(screen.getByRole("listbox")).getByText("20."));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Der Stichtag konnte nicht gespeichert werden. Bitte versuchen Sie es noch einmal.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).toBeInTheDocument();
  });
});

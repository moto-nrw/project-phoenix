import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { OfferingGap } from "~/lib/offering-gap";

const {
  mockGaps,
  mockLoadError,
  mockSetAcknowledged,
  mockMutate,
  mockToastSuccess,
  mockShowError,
} = vi.hoisted(() => ({
  mockGaps: { current: [] as OfferingGap[] },
  mockLoadError: { current: undefined as unknown },
  mockSetAcknowledged: vi.fn(),
  mockMutate: vi.fn(() => Promise.resolve()),
  mockToastSuccess: vi.fn(),
  mockShowError: vi.fn(),
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: { setOfferingGapAcknowledged: mockSetAcknowledged },
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => ({
    data: key === null || mockLoadError.current ? undefined : mockGaps.current,
    error: key === null ? undefined : mockLoadError.current,
  }),
  useTenantMutate: () => mockMutate,
}));

// Only the toasts are replaced; the load-error path stays real.
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: mockToastSuccess }),
  useApiErrorDisplay: () => ({ show: mockShowError }),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: null, status: "unauthenticated" }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() }),
}));

import { ApiError } from "~/lib/api-error";
import { OfferingGapHints } from "./offering-gap-hints";

const randstunde: OfferingGap = {
  offering: { id: "7", name: "Randstunde" },
  studentCount: 46,
  schoolClasses: ["3a", "3b", "4a"],
  weekdays: [1, 2, 3, 4, 5],
  acknowledged: false,
  acknowledgedClasses: [],
};

const mittag: OfferingGap = {
  offering: { id: "8", name: "Mittagessen" },
  studentCount: 1,
  schoolClasses: [""],
  weekdays: [3],
  acknowledged: true,
  acknowledgedClasses: [""],
};

function renderHints(canManage = true) {
  const onCreateSeries = vi.fn();
  render(
    <OfferingGapHints
      periodId="5"
      canManage={canManage}
      onCreateSeries={onCreateSeries}
      templatesVersion={undefined}
    />,
  );
  return { onCreateSeries };
}

describe("OfferingGapHints", () => {
  beforeEach(() => {
    mockGaps.current = [randstunde, mittag];
    mockLoadError.current = undefined;
    mockSetAcknowledged.mockReset();
    mockToastSuccess.mockClear();
    mockShowError.mockClear();
    mockMutate.mockClear();
    mockSetAcknowledged.mockResolvedValue(undefined);
  });

  it("names the offering, the children, their classes and days", () => {
    renderHints();

    expect(
      screen.getByText(
        "46 Kinder haben „Randstunde“ gebucht, aber keinen Regeltermin. Klassen 3a, 3b, 4a · Mo, Di, Mi, Do, Fr",
      ),
    ).toBeInTheDocument();
    // The decided offering waits in the folded list, not as a warning.
    expect(
      screen.getByRole("heading", { name: "Ohne eigenen Regeltermin (1)" }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/„Mittagessen“ gebucht/)).not.toBeInTheDocument();
  });

  it("creates a Regeltermin or records that none is needed", async () => {
    const { onCreateSeries } = renderHints();

    fireEvent.click(
      screen.getByRole("button", { name: "Regeltermin anlegen" }),
    );
    expect(onCreateSeries).toHaveBeenCalledTimes(1);

    fireEvent.click(
      screen.getByRole("button", { name: "Kein Regeltermin nötig" }),
    );
    await waitFor(() =>
      expect(mockSetAcknowledged).toHaveBeenCalledWith("7", "5", true),
    );
    await waitFor(() =>
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Der Hinweis zu „Randstunde“ ist ausgeblendet.",
      ),
    );
    expect(mockMutate).toHaveBeenCalledWith("offering-gaps-5");
  });

  it("brings a decided hint back", async () => {
    renderHints();

    fireEvent.click(
      screen.getByRole("button", {
        name: /Ohne eigenen Regeltermin.*ausklappen/,
      }),
    );
    expect(
      screen.getByText("„Mittagessen“ · Kinder ohne Klasse"),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Hinweis wieder zeigen" }),
    );

    await waitFor(() =>
      expect(mockSetAcknowledged).toHaveBeenCalledWith("8", "5", false),
    );
  });

  it("shows a failed decision through the error path", async () => {
    const failure = new Error("offline");
    mockSetAcknowledged.mockRejectedValue(failure);
    renderHints();

    fireEvent.click(
      screen.getByRole("button", { name: "Kein Regeltermin nötig" }),
    );

    await waitFor(() =>
      expect(mockShowError).toHaveBeenCalledWith(failure, {
        object: "das Ausblenden des Hinweises",
      }),
    );
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("only shows open hints to readers without planning rights", () => {
    renderHints(false);

    expect(screen.getByText(/„Randstunde“ gebucht/)).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(
      screen.queryByText(/Ohne eigenen Regeltermin/),
    ).not.toBeInTheDocument();
  });
  it("shows a failed check with a retry instead of an empty list", async () => {
    mockLoadError.current = new ApiError("boom", 500);
    renderHints();

    expect(
      await screen.findByText(/Prüfung der Angebote ohne Regeltermin/),
    ).toBeInTheDocument();
    expect(screen.queryByText(/gebucht, aber/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() =>
      expect(mockMutate).toHaveBeenCalledWith("offering-gaps-5"),
    );
  });
});

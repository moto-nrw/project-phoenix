import { act, fireEvent, render, screen } from "@testing-library/react";
import { SWRConfig } from "swr";
import { expect, it, vi } from "vitest";

import type { OfferingGap, OfferingGapReport } from "~/lib/offering-gap";

const { getGaps, setAcknowledged } = vi.hoisted(() => ({
  getGaps: vi.fn(),
  setAcknowledged: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    getOfferingGaps: getGaps,
    setOfferingGapAcknowledged: setAcknowledged,
  },
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "planner" } },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school",
  useTenantRoutingModeSafe: () => "path",
}));
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: vi.fn() }),
  useApiErrorDisplay: () => ({ show: vi.fn() }),
}));

vi.unmock("swr");

import { OfferingGapHints } from "~/components/timetable/offering-gap-hints";

it("removes old hints until the selected period has loaded", async () => {
  const oldGap: OfferingGap = {
    offering: { id: "7", name: "Randstunde" },
    studentCount: 1,
    schoolClasses: ["3a"],
    weekdays: [1],
    acknowledged: false,
    acknowledgedClasses: [],
    newSeriesClasses: [],
    newSeriesPlansCovered: false,
  };
  let resolveNext!: (report: OfferingGapReport) => void;
  const next = new Promise<OfferingGapReport>((resolve) => {
    resolveNext = resolve;
  });
  getGaps.mockImplementation((periodId: string) =>
    periodId === "5"
      ? Promise.resolve({ gaps: [oldGap], phasesOutsidePeriod: [] })
      : next,
  );
  const cache = new Map();
  const view = (periodId: string) => (
    <SWRConfig value={{ provider: () => cache }}>
      <OfferingGapHints
        periodId={periodId}
        canManage
        canManagePhases={false}
        onCreateSeries={vi.fn()}
        templatesVersion={undefined}
      />
    </SWRConfig>
  );
  const { rerender } = render(view("5"));
  expect(await screen.findByText("„Randstunde“")).toBeInTheDocument();

  rerender(view("6"));
  expect(screen.queryAllByRole("button")).toHaveLength(0);

  await act(async () => {
    resolveNext({
      gaps: [{ ...oldGap, offering: { id: "8", name: "Mittagessen" } }],
      phasesOutsidePeriod: [],
    });
  });
  // Anker: die Hinweise des neu gewählten Zeitraums sind geladen.
  expect(await screen.findByText("„Mittagessen“")).toBeInTheDocument();
  expect(screen.queryByText("„Randstunde“")).not.toBeInTheDocument();
  fireEvent.click(
    screen.getByRole("button", {
      name: "Kein Regeltermin nötig: „Mittagessen“",
    }),
  );
  expect(setAcknowledged).toHaveBeenCalledWith("8", "6", true);
});

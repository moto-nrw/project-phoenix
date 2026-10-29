import "@testing-library/jest-dom/vitest";
import {
  render as rtlRender,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { CareTimeOverrunsSection } from "./care-time-overruns-section";
import { ToastProvider } from "~/contexts/ToastContext";
import type { CareTimeOverruns } from "~/lib/care-times";

function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: vi.fn() })),
  usePathname: vi.fn(() => "/statistics"),
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => path,
}));

const mockFetch = vi.fn();
vi.mock("~/lib/care-times", async () => {
  const actual =
    await vi.importActual<typeof import("~/lib/care-times")>(
      "~/lib/care-times",
    );
  return {
    ...actual,
    fetchCareTimeOverruns: (...args: unknown[]) =>
      mockFetch(...args) as Promise<CareTimeOverruns>,
  };
});

function summary(overrunCount: number, unbooked: number, extra: number) {
  return {
    overrun_count: overrunCount,
    overrun_days: overrunCount - unbooked,
    unbooked_days: unbooked,
    absent_days: 0,
    extra_minutes: extra,
    planned_minutes: 0,
    actual_minutes: 0,
    deviation_minutes: 0,
  };
}

describe("Soll und Ist — Liste für die Leitung (#4774)", () => {
  beforeEach(() => {
    mockFetch.mockReset();
  });

  it("sortiert nach der Zahl der Überschreitungen und verlinkt das Kind", async () => {
    mockFetch.mockResolvedValue({
      from: "2026-09-01",
      to: "2026-09-09",
      tolerance_minutes: 15,
      students: [
        {
          student_id: "8",
          first_name: "Ben",
          last_name: "Arnold",
          school_class: "1a",
          group_name: "Füchse",
          summary: summary(1, 0, 20),
        },
        {
          student_id: "7",
          first_name: "Mia",
          last_name: "Berg",
          school_class: "2a",
          group_name: "Eulen",
          summary: summary(4, 1, 150),
        },
      ],
    });
    render(
      <CareTimeOverrunsSection
        fromISO="2026-09-01"
        toISO="2026-09-09"
        groupIds={["3"]}
      />,
    );

    // Der Telefonstapel wiederholt die Zeilen; geprüft wird die Tabelle.
    await waitFor(() => expect(mockFetch).toHaveBeenCalled());
    const table = within(await screen.findByRole("table"));
    const mia = await table.findByRole("link", { name: "Berg, Mia" });
    expect(mockFetch).toHaveBeenCalledWith("2026-09-01", "2026-09-09", ["3"]);
    expect(mia).toHaveAttribute("href", "/students/7?tab=betreuungszeiten");
    const ben = table.getByRole("link", { name: "Arnold, Ben" });
    // Mia hat mehr Überschreitungen und steht deshalb oben.
    expect(
      mia.compareDocumentPosition(ben) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(table.getByText("150")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Liste exportieren" }),
    ).toBeInTheDocument();
  });

  it("nennt im Leerzustand, dass alle Kinder in ihren Zeiten waren", async () => {
    mockFetch.mockResolvedValue({
      from: "2026-09-01",
      to: "2026-09-09",
      tolerance_minutes: 15,
      students: [],
    });
    render(
      <CareTimeOverrunsSection
        fromISO="2026-09-01"
        toISO="2026-09-09"
        groupIds={[]}
      />,
    );

    await waitFor(() =>
      expect(
        screen.getAllByText("Keine Überschreitungen").length,
      ).toBeGreaterThan(0),
    );
  });
});

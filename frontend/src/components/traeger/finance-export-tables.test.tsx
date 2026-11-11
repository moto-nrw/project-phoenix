import { fireEvent, render, screen, within } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import type {
  CostCenterFallback,
  ExportProtocolEntry,
} from "~/lib/traeger-finance-exports-api";
import { ExportProtocolTable } from "./finance-export-tables";

vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

function holidayFile(fallbacks: CostCenterFallback[]): ExportProtocolEntry {
  return {
    id: "21",
    fileNumber: 21,
    format: "datev",
    runKind: "holiday",
    runId: "4",
    runLabel: "Herbstferien 2026",
    fileName: "EXTF_Buchungsstapel_21.csv",
    invoiceCount: 3,
    totalCents: 24000,
    reference: "",
    repeatOf: null,
    datevAll: false,
    resolvedPositions: 0,
    costCenterFallbacks: fallbacks,
    exportedByYou: true,
    exportedByAccount: true,
    exportedAt: "2026-11-09T10:00:00Z",
    submittedAt: null,
  };
}

function fallback(number: string, week: string): CostCenterFallback {
  return {
    number,
    weekStartsOn: week,
    weekEndsOn: week,
    reason: "no_school_cost_center",
    costCenter: "FER",
  };
}

// #6624: the protocol names each holiday position on the fallback cost
// center with its document, week, reason and the cost center used.
it("names the holiday positions booked on the fallback cost center", () => {
  render(
    <ExportProtocolTable
      entries={[
        holidayFile([
          fallback("FE-2026-0001", "2026-10-12"),
          {
            number: "FE-2026-0002",
            weekStartsOn: "2026-10-19",
            weekEndsOn: "2026-10-23",
            reason: "no_host_school",
            costCenter: "",
          },
        ]),
      ]}
    />,
  );

  expect(
    screen.getAllByText("2 Einträge mit Ersatz-Kostenstelle").length,
  ).toBeGreaterThan(0);
  const first = screen.getAllByText("FE-2026-0001")[0]!.closest("li")!;
  expect(first).toHaveTextContent(
    "FE-2026-0001, Woche ab 12.10.2026: Die Schule der Woche hat an dem Tag keine Kostenstelle. Gebucht auf FER.",
  );
  const second = screen.getAllByText("FE-2026-0002")[0]!.closest("li")!;
  expect(second).toHaveTextContent(
    "Zur Woche gibt es keine Schule. Gebucht ohne Kostenstelle.",
  );
});

it("folds more than five fallback positions", () => {
  const weeks = ["01", "02", "03", "04", "05", "06", "07"];
  render(
    <ExportProtocolTable
      entries={[
        holidayFile(
          weeks.map((day) => fallback(`FE-2026-00${day}`, `2026-10-${day}`)),
        ),
      ]}
    />,
  );
  const table = screen.getAllByRole("table")[0]!;
  expect(within(table).getAllByRole("listitem")).toHaveLength(5);
  const more = within(table).getByRole("button", {
    name: "2 weitere anzeigen",
  });
  expect(more).toHaveAttribute("aria-expanded", "false");
  fireEvent.click(more);
  expect(within(table).getAllByRole("listitem")).toHaveLength(7);
  expect(
    within(table).getByRole("button", { name: "Weniger anzeigen" }),
  ).toHaveAttribute("aria-expanded", "true");
});

it("shows no fallback note for a file without one", () => {
  render(<ExportProtocolTable entries={[holidayFile([])]} />);
  expect(
    screen.getAllByText("EXTF_Buchungsstapel_21.csv").length,
  ).toBeGreaterThan(0);
  expect(screen.queryByText(/Ersatz-Kostenstelle/)).toBeNull();
});

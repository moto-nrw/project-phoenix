import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { CapacityStrip } from "./capacity-strip";
import { CoverageIndicator } from "./coverage-indicator";
import { PlanBlock } from "./plan-block";
import { ResourceGrid, type ResourceGridColumn } from "./resource-grid";

interface DemoRow {
  readonly id: string;
  readonly name: string;
  readonly weeklySum: string;
}

const DEMO_ROWS: readonly DemoRow[] = [
  { id: "r1", name: "A. Krause", weeklySum: "19,5/20,25 h" },
  { id: "r2", name: "B. Yilmaz", weeklySum: "11/12 h" },
];

const DAY_COLUMNS: readonly ResourceGridColumn[] = [
  { key: "mo", label: "Mo 13.07." },
  { key: "di", label: "Di 14.07.", isCurrent: true },
  { key: "mi", label: "Mi 15.07." },
  { key: "do", label: "Do 16.07." },
  { key: "fr", label: "Fr 17.07." },
];

const WEEK_COLUMNS: readonly ResourceGridColumn[] = Array.from(
  { length: 12 },
  (_, index) => ({
    key: `kw-${29 + index}`,
    label: `KW ${29 + index}`,
    isCurrent: index === 0,
  }),
);

const meta = {
  title: "components/ui/ResourceGrid",
} satisfies Meta;

export default meta;

type Story = StoryObj;

export const WeekDays: Story = {
  render: () => (
    <ResourceGrid<DemoRow>
      ariaLabel="Wochenraster"
      columns={DAY_COLUMNS}
      rows={DEMO_ROWS}
      getRowKey={(row) => row.id}
      cornerHeader="Person"
      renderRowHeader={(row) => (
        <span className="block">
          <span className="block text-sm font-medium text-gray-900">
            {row.name}
          </span>
          <CoverageIndicator state="covered" label={row.weeklySum} size="sm" />
        </span>
      )}
      renderCell={(row, column) =>
        row.id === "r1" && column.key === "mo" ? (
          <PlanBlock
            size="compact"
            timeRange="08:00–10:00"
            label="Vertretung"
            color="#0F766E"
          />
        ) : null
      }
      emptyCellLabel={(row, column) =>
        `Schicht anlegen, ${row.name}, ${column.label}`
      }
      onEmptyCellClick={() => undefined}
      footer={
        <CapacityStrip
          rowLabel="Kapazität 12-16"
          cells={DAY_COLUMNS.map((column, index) => ({
            key: column.key,
            content: 6 - (index % 3),
          }))}
        />
      }
    />
  ),
};

export const HalfYearWeeks: Story = {
  render: () => (
    <ResourceGrid<DemoRow>
      ariaLabel="Halbjahresraster"
      columnMode="weeks"
      columns={WEEK_COLUMNS}
      rows={DEMO_ROWS}
      getRowKey={(row) => row.id}
      cornerHeader="Person"
      renderRowHeader={(row) => (
        <span className="block text-sm font-medium text-gray-900">
          {row.name}
        </span>
      )}
      renderCell={(row, column) =>
        column.key === "kw-29" ? (
          <span className="block px-1 text-center text-xs text-gray-600 tabular-nums">
            {row.weeklySum.split("/")[0]}
          </span>
        ) : null
      }
    />
  ),
};

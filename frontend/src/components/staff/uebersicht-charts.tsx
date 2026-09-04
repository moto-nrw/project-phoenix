"use client";

import {
  Area,
  AreaChart,
  CartesianGrid,
  Cell,
  Label,
  Line,
  LineChart,
  Pie,
  PieChart,
  ReferenceLine,
  XAxis,
  YAxis,
} from "recharts";
import {
  type ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "~/components/ui/chart";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import { formatSignedDuration } from "./staff-time-views";

function formatHoursCompact(minutes: number): string {
  if (minutes === 0) return "0h";
  const sign = minutes > 0 ? "+" : "−";
  return `${sign}${Math.round(Math.abs(minutes) / 60)}h`;
}

interface DailyTrendPoint {
  readonly dayLabel: string;
  readonly fullLabel: string;
  readonly ist: number;
  readonly soll: number;
}

function DailyTrendAxes() {
  return (
    <>
      <CartesianGrid vertical={false} strokeDasharray="3 3" />
      <XAxis
        dataKey="dayLabel"
        tickLine={false}
        axisLine={false}
        tickMargin={8}
        fontSize={11}
        interval="preserveStartEnd"
      />
      <YAxis
        tickFormatter={(value) => formatHoursCompact(Number(value))}
        tickLine={false}
        axisLine={false}
        tickMargin={4}
        fontSize={11}
        width={56}
      />
    </>
  );
}

function DailyTrendTooltip() {
  return (
    <ChartTooltip
      cursor={false}
      content={
        <ChartTooltipContent
          indicator="line"
          labelFormatter={(_label, payload) => {
            const point = payload?.[0]?.payload as
              { fullLabel?: string } | undefined;
            return point?.fullLabel ?? "";
          }}
          formatter={(value) => formatSignedDuration(Number(value))}
        />
      }
    />
  );
}

function DailyTrendLines() {
  return (
    <>
      <Line
        dataKey="soll"
        name="Soll"
        type="monotone"
        stroke="var(--color-soll)"
        strokeWidth={1.5}
        strokeDasharray="4 4"
        dot={false}
        isAnimationActive={false}
      />
      <Line
        dataKey="ist"
        name="Ist"
        type="monotone"
        stroke="var(--color-ist)"
        strokeWidth={2.5}
        dot={{ fill: "var(--color-ist)", r: 3 }}
        activeDot={{ r: 5 }}
      />
    </>
  );
}

export function DailyTrendChart({
  config,
  data,
}: {
  readonly config: ChartConfig;
  readonly data: readonly DailyTrendPoint[];
}) {
  return (
    <ChartContainer config={config} className="!aspect-auto h-64 w-full">
      <LineChart
        accessibilityLayer
        data={data}
        margin={{ top: 8, right: 12, left: 0, bottom: 0 }}
      >
        <DailyTrendAxes />
        <DailyTrendTooltip />
        <DailyTrendLines />
      </LineChart>
    </ChartContainer>
  );
}

function BalanceTrendAxes() {
  return (
    <>
      <CartesianGrid vertical={false} strokeDasharray="3 3" />
      <XAxis
        dataKey="weekLabel"
        tickLine={false}
        axisLine={false}
        tickMargin={8}
        fontSize={11}
      />
      <YAxis
        tickFormatter={(value) => formatHoursCompact(Number(value))}
        tickLine={false}
        axisLine={false}
        tickMargin={4}
        fontSize={11}
        width={56}
      />
    </>
  );
}

function BalanceGradient() {
  return (
    <defs>
      <linearGradient id="fillBalance" x1="0" y1="0" x2="0" y2="1">
        <stop offset="5%" stopColor="var(--color-balance)" stopOpacity={0.18} />
        <stop
          offset="95%"
          stopColor="var(--color-balance)"
          stopOpacity={0.02}
        />
      </linearGradient>
    </defs>
  );
}

function BalanceSeries() {
  return (
    <>
      <ReferenceLine
        y={0}
        stroke={MOTO_COLOR_PALETTE.neutral.light}
        strokeDasharray="3 3"
        strokeWidth={1}
      />
      <ChartTooltip
        cursor={false}
        content={
          <ChartTooltipContent
            indicator="line"
            formatter={(value) => formatSignedDuration(Number(value))}
          />
        }
      />
      <Area
        dataKey="balance"
        type="natural"
        stroke="var(--color-balance)"
        strokeWidth={2.5}
        fill="url(#fillBalance)"
      />
    </>
  );
}

export function BalanceTrendChart({
  config,
  data,
}: {
  readonly config: ChartConfig;
  readonly data: readonly { weekLabel: string; balance: number }[];
}) {
  return (
    <ChartContainer config={config} className="!aspect-auto h-64 w-full">
      <AreaChart
        accessibilityLayer
        data={data}
        margin={{ top: 8, right: 12, left: 0, bottom: 0 }}
      >
        <BalanceGradient />
        <BalanceTrendAxes />
        <BalanceSeries />
      </AreaChart>
    </ChartContainer>
  );
}

function getChartCenter(viewBox: unknown): { cx: number; cy: number } | null {
  const candidate =
    viewBox && typeof viewBox === "object"
      ? (viewBox as { cx?: unknown; cy?: unknown })
      : null;
  if (
    !candidate ||
    typeof candidate.cx !== "number" ||
    typeof candidate.cy !== "number"
  ) {
    return null;
  }
  return { cx: candidate.cx, cy: candidate.cy };
}

function DistributionCenterLabel({
  total,
  viewBox,
}: {
  readonly total: number;
  readonly viewBox?: unknown;
}) {
  const center = getChartCenter(viewBox);
  if (!center) return null;

  return (
    <text
      x={center.cx}
      y={center.cy}
      textAnchor="middle"
      dominantBaseline="middle"
    >
      <tspan
        x={center.cx}
        y={center.cy}
        className="fill-gray-900 text-3xl font-bold"
      >
        {total}
      </tspan>
      <tspan x={center.cx} y={center.cy + 22} className="fill-gray-500 text-sm">
        Tage
      </tspan>
    </text>
  );
}

interface DistributionPoint {
  readonly key: string;
  readonly label: string;
  readonly value: number;
  readonly color: string;
}

function DistributionTooltip() {
  return (
    <ChartTooltip
      cursor={false}
      content={
        <ChartTooltipContent
          hideLabel
          formatter={(value, _name, item) => {
            const days = Number(value);
            const label =
              typeof item?.payload?.label === "string"
                ? item.payload.label
                : "";
            return `${label}: ${days} ${days === 1 ? "Tag" : "Tage"}`;
          }}
        />
      }
    />
  );
}

function DistributionPie({
  data,
  total,
}: {
  readonly data: readonly DistributionPoint[];
  readonly total: number;
}) {
  const visibleData = data.filter((point) => point.value > 0);
  return (
    <PieChart>
      <DistributionTooltip />
      <Pie
        data={visibleData}
        dataKey="value"
        nameKey="key"
        innerRadius={60}
        outerRadius={95}
        strokeWidth={3}
      >
        {visibleData.map((point) => (
          <Cell key={point.key} fill={point.color} />
        ))}
        <Label content={<DistributionCenterLabel total={total} />} />
      </Pie>
    </PieChart>
  );
}

export function DistributionChart({
  config,
  data,
  total,
}: {
  readonly config: ChartConfig;
  readonly data: readonly DistributionPoint[];
  readonly total: number;
}) {
  return (
    <ChartContainer
      config={config}
      className="mx-auto aspect-square h-64 w-full max-w-[18rem]"
    >
      <DistributionPie data={data} total={total} />
    </ChartContainer>
  );
}

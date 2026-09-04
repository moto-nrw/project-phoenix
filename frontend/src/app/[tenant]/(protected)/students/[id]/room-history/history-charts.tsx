"use client";

import { useCallback, useMemo, type ReactNode } from "react";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import {
  type ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "~/components/ui/chart";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";

export interface HistoryChartPoint {
  readonly date: string;
  readonly isToday: boolean;
  readonly roomDetailAvailable: boolean;
  readonly duration: number;
  readonly visits: number;
}

const durationChartConfig: ChartConfig = {
  duration: {
    label: "Stunden",
    color: MOTO_COLOR_PALETTE.green.base,
  },
};

const activityChartConfig: ChartConfig = {
  visits: {
    label: "Raumwechsel",
    color: MOTO_COLOR_PALETTE.blue.base,
  },
};

function TodayMarker({ x, y }: { readonly x: number; readonly y: number }) {
  return (
    <text
      x={x}
      y={y + 24}
      textAnchor="middle"
      fontSize={9}
      fontWeight={500}
      fill={MOTO_COLOR_PALETTE.green.base}
    >
      heute
    </text>
  );
}

function TodayTick({
  data,
  props,
}: {
  readonly data: readonly HistoryChartPoint[];
  readonly props: Record<string, unknown>;
}) {
  const x = Number(props.x);
  const y = Number(props.y);
  const idx = (props.payload as { index?: number })?.index ?? 0;
  const item = data[idx];
  const isToday = item?.isToday;
  return (
    <g>
      <text
        x={x}
        y={y + 12}
        textAnchor="middle"
        fontSize={11}
        fontWeight={isToday ? 700 : 400}
        fill={
          isToday
            ? MOTO_COLOR_PALETTE.neutral.strong
            : MOTO_COLOR_PALETTE.neutral.light
        }
      >
        {item?.date}
      </text>
      {isToday && <TodayMarker x={x} y={y} />}
    </g>
  );
}

function useTodayTickRenderer(data: readonly HistoryChartPoint[]) {
  return useCallback(
    (props: Record<string, unknown>) => <TodayTick data={data} props={props} />,
    [data],
  );
}

function durationTooltipValue(value: string | number) {
  return <span className="font-medium">{value} Std</span>;
}

function activityTooltipValue(value: string | number) {
  return <span className="font-medium">{value} Wechsel</span>;
}

function HistoryAxes({
  tick,
  allowDecimals,
  tickFormatter,
}: {
  readonly tick: (props: Record<string, unknown>) => ReactNode;
  readonly allowDecimals: boolean;
  readonly tickFormatter?: (value: number) => string;
}) {
  return (
    <>
      <CartesianGrid vertical={false} />
      <XAxis
        dataKey="date"
        tickLine={false}
        axisLine={false}
        tickMargin={8}
        fontSize={11}
        interval={0}
        tick={tick}
      />
      <YAxis
        tickLine={false}
        axisLine={false}
        tickMargin={4}
        fontSize={12}
        allowDecimals={allowDecimals}
        tickFormatter={tickFormatter}
      />
    </>
  );
}

function HistoryTooltip({
  formatter,
}: {
  readonly formatter: (value: string | number) => ReactNode;
}) {
  return (
    <ChartTooltip
      content={
        <ChartTooltipContent
          labelFormatter={(label) => `Tag: ${label}`}
          formatter={formatter}
        />
      }
    />
  );
}

function HistoryBar({ dataKey }: { readonly dataKey: "duration" | "visits" }) {
  return (
    <Bar
      dataKey={dataKey}
      fill={`var(--color-${dataKey})`}
      radius={[6, 6, 6, 6]}
    />
  );
}

function HistoryPlot({
  config,
  data,
  dataKey,
  tick,
  tooltipFormatter,
  allowDecimals = true,
  tickFormatter,
}: {
  readonly config: ChartConfig;
  readonly data: readonly HistoryChartPoint[];
  readonly dataKey: "duration" | "visits";
  readonly tick: (props: Record<string, unknown>) => ReactNode;
  readonly tooltipFormatter: (value: string | number) => ReactNode;
  readonly allowDecimals?: boolean;
  readonly tickFormatter?: (value: number) => string;
}) {
  return (
    <ChartContainer config={config} className="h-[180px] w-full sm:h-[200px]">
      <BarChart
        data={data}
        margin={{ top: 4, right: 4, bottom: 0, left: -20 }}
        barCategoryGap="20%"
      >
        <HistoryAxes
          tick={tick}
          allowDecimals={allowDecimals}
          tickFormatter={tickFormatter}
        />
        <HistoryTooltip formatter={tooltipFormatter} />
        <HistoryBar dataKey={dataKey} />
      </BarChart>
    </ChartContainer>
  );
}

function HistoryCard({
  title,
  concept,
  subtitle,
  children,
}: {
  readonly title: string;
  readonly concept: "present" | "rooms";
  readonly subtitle: string;
  readonly children: ReactNode;
}) {
  return (
    <div className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm">
      <div className="p-4 sm:p-6">
        <ConceptSectionHeader
          className="mb-3"
          title={title}
          concept={concept}
          subtitle={subtitle}
        />
        {children}
      </div>
    </div>
  );
}

function ActivityChart({
  data,
  tick,
}: {
  readonly data: readonly HistoryChartPoint[];
  readonly tick: (props: Record<string, unknown>) => ReactNode;
}) {
  if (data.length === 0) {
    return (
      <div className="flex h-[180px] items-center justify-center sm:h-[200px]">
        <p className="text-sm text-gray-400">
          Keine Raumdetails verfügbar (Aufbewahrungsfrist überschritten).
        </p>
      </div>
    );
  }
  return (
    <HistoryPlot
      config={activityChartConfig}
      data={data}
      dataKey="visits"
      tick={tick}
      tooltipFormatter={activityTooltipValue}
      allowDecimals={false}
    />
  );
}

export default function HistoryCharts({
  data,
}: {
  readonly data: readonly HistoryChartPoint[];
}) {
  const activityData = useMemo(
    () => data.filter((point) => point.roomDetailAvailable),
    [data],
  );
  const durationTick = useTodayTickRenderer(data);
  const activityTick = useTodayTickRenderer(activityData);

  return (
    <div className="grid grid-cols-1 gap-4 md:grid-cols-2 md:gap-6">
      <HistoryCard
        title="Anwesenheit"
        concept="present"
        subtitle="Tägliche Aufenthaltsdauer in Stunden"
      >
        <HistoryPlot
          config={durationChartConfig}
          data={data}
          dataKey="duration"
          tick={durationTick}
          tooltipFormatter={durationTooltipValue}
          tickFormatter={(value) => `${value}h`}
        />
      </HistoryCard>
      <HistoryCard
        title="Aktivität"
        concept="rooms"
        subtitle="Raumwechsel pro Tag"
      >
        <ActivityChart data={activityData} tick={activityTick} />
      </HistoryCard>
    </div>
  );
}

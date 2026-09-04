"use client";

import { memo, useEffect, useMemo, useState } from "react";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import {
  type ChartConfig,
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
} from "~/components/ui/chart";
import { parseISODate, toISODate } from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import {
  indexWorkSessionMinutesByBerlinDate,
  type WorkSessionHistory,
} from "~/lib/time-tracking-helpers";

const DAY_NAMES = ["Mo", "Di", "Mi", "Do", "Fr", "Sa", "So"];

const weekChartConfig = {
  netMinutes: {
    label: "Arbeitszeit",
    color: MOTO_COLOR_PALETTE.timeTracking.base,
  },
  breakMinutes: {
    label: "Pause",
    color: MOTO_COLOR_PALETTE.neutral.light,
  },
} satisfies ChartConfig;

function formatDateShort(date: Date): string {
  const day = date.getDate().toString().padStart(2, "0");
  const month = (date.getMonth() + 1).toString().padStart(2, "0");
  return `${day}.${month}`;
}

interface WeekChartPoint {
  readonly dayKey: string;
  readonly dayShort: string;
  readonly label: string;
  readonly netMinutes: number;
  readonly breakMinutes: number;
}

function useMobileViewport(): boolean {
  const [isMobile, setIsMobile] = useState(false);
  useEffect(() => {
    const check = () => setIsMobile(window.innerWidth < 768);
    check();
    window.addEventListener("resize", check);
    return () => window.removeEventListener("resize", check);
  }, []);
  return isMobile;
}

function useWeekChartData({
  history,
  weekOffset,
}: {
  readonly history: WorkSessionHistory[];
  readonly weekOffset: number;
}): WeekChartPoint[] {
  // The chart's date axis is keyed by Berlin calendar days. Keeping that day as
  // an explicit dependency makes the memo accurate and rolls an open tab over
  // at Berlin midnight without rebuilding it for every render.
  const todayISO = useBerlinToday();

  return useMemo(() => {
    const referenceDate = parseISODate(todayISO);
    referenceDate.setDate(referenceDate.getDate() + weekOffset * 7);
    const allDays: Date[] = [];
    const day = new Date(referenceDate);
    while (allDays.length < 10) {
      if (day.getDay() !== 0 && day.getDay() !== 6) {
        allDays.unshift(new Date(day));
      }
      day.setDate(day.getDate() - 1);
    }
    const minutesByDate = indexWorkSessionMinutesByBerlinDate(history);
    return allDays.map((date) => {
      const dayKey = toISODate(date);
      const dayMinutes = minutesByDate.get(dayKey);
      const dayShort = DAY_NAMES[(date.getDay() + 6) % 7] ?? "";
      return {
        dayKey,
        dayShort,
        label: `${dayShort} ${formatDateShort(date)}`,
        netMinutes: dayMinutes?.netMinutes ?? 0,
        breakMinutes: dayMinutes?.breakMinutes ?? 0,
      };
    });
  }, [history, todayISO, weekOffset]);
}

function tooltipLabelFormatter(
  _value: unknown,
  payload: ReadonlyArray<{ payload?: { label?: string } }>,
): string {
  return payload[0]?.payload?.label ?? "";
}

function tooltipValueFormatter(
  value: number | string | ReadonlyArray<number | string> | undefined,
  name: string | number | undefined,
): string {
  const totalMinutes = (value ?? 0) as number;
  const label = name === "netMinutes" ? "Arbeitszeit" : "Pause";
  return `${label}: ${Math.floor(totalMinutes / 60)}h ${totalMinutes % 60}min`;
}

function WeekChartAxes({
  chartData,
  isMobile,
}: {
  readonly chartData: readonly WeekChartPoint[];
  readonly isMobile: boolean;
}) {
  return (
    <>
      <CartesianGrid vertical={false} />
      <XAxis
        dataKey="dayKey"
        tickLine={false}
        axisLine={false}
        tickMargin={8}
        fontSize={isMobile ? 10 : 11}
        interval={0}
        tickFormatter={(value: string) =>
          chartData.find((entry) => entry.dayKey === value)?.dayShort ?? ""
        }
      />
      <YAxis
        tickLine={false}
        axisLine={false}
        tickMargin={4}
        fontSize={isMobile ? 10 : 12}
        tickFormatter={(value: number) => `${Math.round(value / 60)}h`}
        domain={[0, "auto"]}
      />
    </>
  );
}

function WeekChartSeries() {
  return (
    <>
      <Bar
        dataKey="breakMinutes"
        stackId="a"
        fill="var(--color-breakMinutes)"
        radius={[0, 0, 4, 4]}
        isAnimationActive={false}
      />
      <Bar
        dataKey="netMinutes"
        stackId="a"
        fill="var(--color-netMinutes)"
        radius={[4, 4, 0, 0]}
        isAnimationActive={false}
      />
    </>
  );
}

function WeekChartPlot({
  chartData,
  isMobile,
}: {
  readonly chartData: readonly WeekChartPoint[];
  readonly isMobile: boolean;
}) {
  return (
    <BarChart
      accessibilityLayer
      data={chartData}
      margin={{ top: 4, right: 4, bottom: 0, left: isMobile ? -24 : -20 }}
      barCategoryGap={isMobile ? 2 : 4}
    >
      <WeekChartAxes chartData={chartData} isMobile={isMobile} />
      <ChartTooltip
        content={
          <ChartTooltipContent
            labelFormatter={tooltipLabelFormatter}
            formatter={tooltipValueFormatter}
          />
        }
      />
      <ChartLegend content={<ChartLegendContent />} />
      <WeekChartSeries />
    </BarChart>
  );
}

function WeekChartHeader({
  chartData,
}: {
  readonly chartData: readonly WeekChartPoint[];
}) {
  return (
    <div className="mb-3 flex items-baseline justify-between sm:mb-4">
      <h2 className="text-base font-bold text-gray-900 sm:text-lg">
        Wochenübersicht
      </h2>
      <span className="text-[10px] text-gray-400 sm:text-xs">
        {chartData[0]?.label.split(" ")[1] ?? ""} –{" "}
        {chartData.at(-1)?.label.split(" ")[1] ?? ""}
      </span>
    </div>
  );
}

const WeekChart = memo(function WeekChart({
  history,
  weekOffset,
}: {
  readonly history: WorkSessionHistory[];
  readonly weekOffset: number;
}) {
  const isMobile = useMobileViewport();
  const chartData = useWeekChartData({ history, weekOffset });

  return (
    <div className="moto-content-surface relative flex min-h-[280px] flex-col overflow-hidden rounded-2xl border shadow-sm md:h-full md:min-h-0">
      <div className="flex min-h-0 flex-1 flex-col p-4 sm:p-6">
        <WeekChartHeader chartData={chartData} />
        <ChartContainer
          config={weekChartConfig}
          className="!aspect-auto min-h-0 flex-1"
        >
          <WeekChartPlot chartData={chartData} isMobile={isMobile} />
        </ChartContainer>
      </div>
    </div>
  );
});

export default WeekChart;

"use client";

import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";
import {
  type ChartConfig,
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
} from "~/components/ui/chart";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";

const feedbackChartConfig = {
  positive: { label: "Positiv", color: MOTO_COLOR_PALETTE.green.base },
  neutral: { label: "Neutral", color: MOTO_COLOR_PALETTE.amber.base },
  negative: { label: "Negativ", color: MOTO_COLOR_PALETTE.red.base },
} satisfies ChartConfig;

interface FeedbackChartPoint {
  readonly day: string;
  readonly label: string;
  readonly positive: number;
  readonly neutral: number;
  readonly negative: number;
}

function FeedbackAxes({ interval }: { readonly interval: number }) {
  return (
    <>
      <CartesianGrid vertical={false} />
      <XAxis
        dataKey="day"
        tickLine={false}
        axisLine={false}
        tickMargin={8}
        fontSize={11}
        interval={interval}
      />
      <YAxis
        tickLine={false}
        axisLine={false}
        tickMargin={4}
        fontSize={11}
        allowDecimals={false}
      />
    </>
  );
}

function FeedbackSeries() {
  return (
    <>
      <Bar
        dataKey="negative"
        stackId="fb"
        fill="var(--color-negative)"
        radius={[0, 0, 4, 4]}
      />
      <Bar
        dataKey="neutral"
        stackId="fb"
        fill="var(--color-neutral)"
        radius={[0, 0, 0, 0]}
      />
      <Bar
        dataKey="positive"
        stackId="fb"
        fill="var(--color-positive)"
        radius={[4, 4, 0, 0]}
      />
    </>
  );
}

function FeedbackPlot({
  data,
}: {
  readonly data: readonly FeedbackChartPoint[];
}) {
  const dense = data.length > 14;
  return (
    <BarChart
      accessibilityLayer
      data={data}
      margin={{ top: 4, right: 4, bottom: 0, left: -24 }}
      barCategoryGap={dense ? 1 : 4}
    >
      <FeedbackAxes interval={dense ? Math.floor(data.length / 7) : 0} />
      <ChartTooltip
        content={
          <ChartTooltipContent
            labelFormatter={(_value, payload) =>
              payload[0]?.payload?.label ?? ""
            }
          />
        }
      />
      <ChartLegend content={<ChartLegendContent />} />
      <FeedbackSeries />
    </BarChart>
  );
}

export default function FeedbackHistoryChart({
  data,
}: {
  readonly data: readonly FeedbackChartPoint[];
}) {
  return (
    <ChartContainer
      config={feedbackChartConfig}
      className="!aspect-auto h-[180px] w-full sm:h-[220px]"
    >
      <FeedbackPlot data={data} />
    </ChartContainer>
  );
}

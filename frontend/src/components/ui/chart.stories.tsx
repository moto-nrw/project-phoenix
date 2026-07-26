import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from "recharts";

import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "~/components/ui/chart";

const meta: Meta<typeof ChartContainer> = {
  title: "components/ui/Chart",
  component: ChartContainer,
};

export default meta;
type Story = StoryObj<typeof ChartContainer>;

const chartConfig = {
  netMinutes: {
    label: "Arbeitszeit",
    color: "#83CD2D",
  },
  breakMinutes: {
    label: "Pause",
    color: "#6B7280",
  },
} satisfies ChartConfig;

const chartData = [
  { dayKey: "mon", dayShort: "Mo", netMinutes: 420, breakMinutes: 30 },
  { dayKey: "tue", dayShort: "Di", netMinutes: 400, breakMinutes: 45 },
  { dayKey: "wed", dayShort: "Mi", netMinutes: 380, breakMinutes: 30 },
  { dayKey: "thu", dayShort: "Do", netMinutes: 410, breakMinutes: 30 },
  { dayKey: "fri", dayShort: "Fr", netMinutes: 300, breakMinutes: 15 },
];

export const BarChartComposition: Story = {
  render: () => (
    <div style={{ width: 480, height: 320 }}>
      <ChartContainer config={chartConfig} className="!aspect-auto h-full">
        <BarChart accessibilityLayer data={chartData}>
          <CartesianGrid vertical={false} />
          <XAxis
            dataKey="dayKey"
            tickLine={false}
            axisLine={false}
            tickFormatter={(value: string) =>
              chartData.find((entry) => entry.dayKey === value)?.dayShort ?? ""
            }
          />
          <YAxis tickLine={false} axisLine={false} />
          <ChartTooltip content={<ChartTooltipContent />} />
          <ChartLegend content={<ChartLegendContent />} />
          <Bar
            dataKey="breakMinutes"
            stackId="a"
            fill="var(--color-breakMinutes)"
            radius={[0, 0, 4, 4]}
          />
          <Bar
            dataKey="netMinutes"
            stackId="a"
            fill="var(--color-netMinutes)"
            radius={[4, 4, 0, 0]}
          />
        </BarChart>
      </ChartContainer>
    </div>
  ),
};

export const EmptyData: Story = {
  render: () => (
    <div style={{ width: 480, height: 320 }}>
      <ChartContainer config={chartConfig} className="!aspect-auto h-full">
        <BarChart accessibilityLayer data={[]}>
          <CartesianGrid vertical={false} />
          <XAxis dataKey="dayKey" tickLine={false} axisLine={false} />
          <YAxis tickLine={false} axisLine={false} />
          <ChartTooltip content={<ChartTooltipContent />} />
          <Bar dataKey="netMinutes" fill="var(--color-netMinutes)" />
        </BarChart>
      </ChartContainer>
    </div>
  ),
};

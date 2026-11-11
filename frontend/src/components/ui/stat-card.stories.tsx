import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { StatCard } from "./stat-card";

const meta = {
  title: "components/ui/StatCard",
} satisfies Meta;

export default meta;
type Story = StoryObj;

const TONES = ["blue", "green", "orange", "red", "gray", "neutral"] as const;

// Figure in the accessible shade of its tone, bar in the raw tone.
export const AllTones: Story = {
  render: () => (
    <div className="grid w-[640px] grid-cols-3 gap-3">
      {TONES.map((tone) => (
        <StatCard
          key={tone}
          label={tone}
          value={42}
          hint="von 60"
          progressPct={70}
          tone={tone}
        />
      ))}
    </div>
  ),
};

export const Tiles: Story = {
  render: () => (
    <div className="grid w-[640px] grid-cols-3 gap-3">
      {TONES.map((tone) => (
        <StatCard
          key={tone}
          variant="tile"
          label={tone}
          value={12}
          tone={tone}
        />
      ))}
    </div>
  ),
};

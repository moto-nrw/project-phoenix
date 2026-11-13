import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { StatusColorBadge } from "./status-color-badge";

const meta = {
  title: "components/ui/StatusColorBadge",
} satisfies Meta;

export default meta;
type Story = StoryObj;

// Every hex with its own pill tone, written out so the story does not depend
// on where the palette lives.
const PALETTE_COLORS = [
  "#83CD2D",
  "#5080D8",
  "#F78C10",
  "#DC2626",
  "#365D83",
  "#0891B2",
  "#217A78",
  "#78716C",
  "#EAB308",
  "#7C3AED",
  "#D946EF",
  "#4F46E5",
  "#6B7280",
  "#0EA5E9",
  "#8F2535",
  "#B7791F",
  "#159E90",
] as const;

export const PaletteColors: Story = {
  render: () => (
    <div className="flex w-[560px] flex-wrap gap-2">
      {PALETTE_COLORS.map((color) => (
        <StatusColorBadge key={color} label={color} color={color} />
      ))}
    </div>
  ),
};

// Room colours outside the palette keep their hue in a darkened label.
export const RoomAndMissingColors: Story = {
  render: () => (
    <div className="flex flex-wrap gap-2">
      <StatusColorBadge label="Raum hellgrün" color="#A3D977" />
      <StatusColorBadge label="Raum rosa" color="#F9A8D4" />
      <StatusColorBadge label="Ohne Farbe" color={null} />
      <StatusColorBadge label="Kleinbuchstaben" color="#83cd2d" />
    </div>
  ),
};

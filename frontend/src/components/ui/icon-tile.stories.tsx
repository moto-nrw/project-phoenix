import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { HouseIcon } from "@phosphor-icons/react/ssr";

import { IconTile } from "./icon-tile";

const meta = {
  title: "components/ui/IconTile",
} satisfies Meta;

export default meta;
type Story = StoryObj;

const TONES = [
  "neutral",
  "inverse",
  "blue",
  "green",
  "greenFill",
  "orange",
  "red",
] as const;

export const AllTones: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-3">
      {TONES.map((tone) => (
        <IconTile key={tone} tone={tone} size="section" icon={<HouseIcon />} />
      ))}
    </div>
  ),
};

export const Round: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <IconTile tone="green" shape="round" size="sm" icon={<HouseIcon />} />
      <IconTile tone="red" shape="round" size="page" icon={<HouseIcon />} />
    </div>
  ),
};

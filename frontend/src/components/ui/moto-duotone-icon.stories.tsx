import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { HouseIcon } from "@phosphor-icons/react/ssr";

import { MOTO_DUOTONE_TONES, MotoDuotoneIcon } from "./moto-duotone-icon";

const meta = {
  title: "components/ui/MotoDuotoneIcon",
} satisfies Meta;

export default meta;
type Story = StoryObj;

const TONES = Object.keys(MOTO_DUOTONE_TONES) as Array<
  keyof typeof MOTO_DUOTONE_TONES
>;

// Every tone of the duotone table, so a palette change shows up here.
export const AllTones: Story = {
  render: () => (
    <div className="grid grid-cols-5 gap-4">
      {TONES.map((tone) => (
        <div key={tone} className="flex flex-col items-center gap-1">
          <MotoDuotoneIcon icon={HouseIcon} tone={tone} />
          <span className="text-xs text-gray-600">{tone}</span>
        </div>
      ))}
    </div>
  ),
};

export const ColorModes: Story = {
  render: () => (
    <div className="flex items-center gap-4 text-gray-700">
      <MotoDuotoneIcon icon={HouseIcon} tone="blue" colorMode="tone" />
      <MotoDuotoneIcon icon={HouseIcon} tone="blue" colorMode="inherit" />
      <MotoDuotoneIcon
        icon={HouseIcon}
        tone="blue"
        colorMode="class"
        className="text-moto-orange"
      />
    </div>
  ),
};

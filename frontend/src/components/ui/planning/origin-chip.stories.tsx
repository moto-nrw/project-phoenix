import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { OriginChip } from "./origin-chip";

const meta = {
  title: "components/ui/planning/OriginChip",
  component: OriginChip,
  args: {
    label: "Soll aus Arbeitszeitmodell",
  },
} satisfies Meta<typeof OriginChip>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithTooltip: Story = {
  args: {
    label: "Soll aus Arbeitszeitmodell",
    title:
      "Wochensaldo und Stundenkonto rechnen gegen das im Arbeitszeitmodell hinterlegte Soll.",
  },
};

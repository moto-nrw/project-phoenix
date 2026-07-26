import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { OriginChip } from "./origin-chip";

const meta = {
  title: "components/ui/OriginChip",
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

export const WithIcon: Story = {
  args: {
    label: "Bedarf: Anmeldung HJ1",
    icon: (
      <svg width="12" height="12" viewBox="0 0 24 24" aria-hidden="true">
        <circle
          cx="12"
          cy="12"
          r="9"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
        />
        <path
          d="M12 7v5l3 3"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
        />
      </svg>
    ),
  },
};

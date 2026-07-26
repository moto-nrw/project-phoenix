import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Tooltip } from "./tooltip";

const meta = {
  title: "components/ui/Tooltip",
  component: Tooltip,
  decorators: [
    (Story) => (
      <div className="p-12">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof Tooltip>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    content: "Geplant 7,5 h · Soll 6 h",
    children: <span className="text-xs font-medium text-gray-600">7,5 h</span>,
  },
};

export const LongContent: Story = {
  args: {
    content:
      "Geplant 4 h · kein Arbeitszeitmodell hinterlegt, daher ist kein Soll-Vergleich möglich",
    children: <span className="text-xs font-medium text-gray-600">4 h</span>,
  },
};

import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { CapacityStrip } from "./capacity-strip";

const meta = {
  title: "components/ui/CapacityStrip",
  component: CapacityStrip,
  decorators: [
    (Story) => (
      <table className="w-full border-collapse">
        <tfoot>
          <Story />
        </tfoot>
      </table>
    ),
  ],
  args: {
    rowLabel: "Kapazität 12-16",
    cells: [
      { key: "mo", content: 6 },
      { key: "di", content: 5 },
      { key: "mi", content: 6 },
      { key: "do", content: 5 },
      { key: "fr", content: 4 },
    ],
  },
} satisfies Meta<typeof CapacityStrip>;

export default meta;

type Story = StoryObj<typeof meta>;

export const PersonCounts: Story = {};

export const WithUnderstaffing: Story = {
  args: {
    cells: [
      { key: "mo", content: 6 },
      { key: "di", content: 5 },
      { key: "mi", content: 2, understaffed: true },
      { key: "do", content: 5 },
      { key: "fr", content: 4 },
    ],
  },
};

export const ChildrenAndPeople: Story = {
  args: {
    rowLabel: "Kernzeit",
    cells: [
      { key: "mo", content: "~42 · 6 P." },
      { key: "di", content: "~40 · 5 P." },
      { key: "mi", content: "~44 · 6 P." },
    ],
  },
};

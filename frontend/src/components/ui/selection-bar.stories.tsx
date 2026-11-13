import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Button } from "./button";
import { SelectionBar } from "./selection-bar";

const meta = {
  title: "components/ui/SelectionBar",
  component: SelectionBar,
  decorators: [
    (Story) => (
      <div className="p-6">
        <Story />
      </div>
    ),
  ],
  args: {
    count: 3,
    onClear: () => {},
    actions: (
      <Button type="button" variant="outline" size="compact">
        Abmelden
      </Button>
    ),
  },
} satisfies Meta<typeof SelectionBar>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Top: Story = { args: { placement: "top" } };

export const Bottom: Story = { args: { placement: "bottom" } };

export const Inline: Story = { args: { placement: "inline" } };

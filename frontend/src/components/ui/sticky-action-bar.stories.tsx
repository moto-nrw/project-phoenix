import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Button } from "./button";
import { StickyActionBar } from "./sticky-action-bar";

const meta = {
  title: "components/ui/StickyActionBar",
  component: StickyActionBar,
  decorators: [
    (Story) => (
      <div className="p-6">
        <Story />
      </div>
    ),
  ],
  args: {
    children: (
      <>
        <Button type="button" variant="outline" size="md">
          Abbrechen
        </Button>
        <Button type="button" variant="primary" size="md">
          Speichern
        </Button>
      </>
    ),
  },
} satisfies Meta<typeof StickyActionBar>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithSummary: Story = {
  args: { summary: "2 Änderungen nicht gespeichert" },
};

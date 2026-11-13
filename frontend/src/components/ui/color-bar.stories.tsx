import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ColorBar } from "./color-bar";

const meta = {
  title: "components/ui/ColorBar",
} satisfies Meta;

export default meta;
type Story = StoryObj;

export const WithAndWithoutColor: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <ColorBar color="#5080D8" size="lg" />
      <ColorBar color={null} size="lg" />
      <ColorBar color="#83CD2D" size="md" />
      <ColorBar size="md" />
    </div>
  ),
};

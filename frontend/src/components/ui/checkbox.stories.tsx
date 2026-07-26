import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Checkbox } from "./checkbox";

const meta = {
  title: "components/ui/Checkbox",
  component: Checkbox,
} satisfies Meta<typeof Checkbox>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Unchecked: Story = {
  render: () => (
    <label htmlFor="checkbox-unchecked" className="flex items-center gap-2">
      <Checkbox id="checkbox-unchecked" />
      <span>Option</span>
    </label>
  ),
};

export const Checked: Story = {
  render: () => (
    <label htmlFor="checkbox-checked" className="flex items-center gap-2">
      <Checkbox id="checkbox-checked" defaultChecked />
      <span>Option</span>
    </label>
  ),
};

export const Disabled: Story = {
  render: () => (
    <label htmlFor="checkbox-disabled" className="flex items-center gap-2">
      <Checkbox id="checkbox-disabled" disabled />
      <span>Option</span>
    </label>
  ),
};

export const DisabledChecked: Story = {
  render: () => (
    <label
      htmlFor="checkbox-disabled-checked"
      className="flex items-center gap-2"
    >
      <Checkbox id="checkbox-disabled-checked" disabled defaultChecked />
      <span>Option</span>
    </label>
  ),
};

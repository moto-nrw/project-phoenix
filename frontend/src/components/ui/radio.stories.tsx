import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Radio } from "./radio";

const meta = {
  title: "components/ui/Radio",
  component: Radio,
} satisfies Meta<typeof Radio>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Unchecked: Story = {
  render: () => (
    <label htmlFor="radio-unchecked" className="flex items-center gap-2">
      <Radio id="radio-unchecked" name="radio-single" />
      <span>Option</span>
    </label>
  ),
};

export const Checked: Story = {
  render: () => (
    <label htmlFor="radio-checked" className="flex items-center gap-2">
      <Radio id="radio-checked" name="radio-single-checked" defaultChecked />
      <span>Option</span>
    </label>
  ),
};

export const Disabled: Story = {
  render: () => (
    <label htmlFor="radio-disabled" className="flex items-center gap-2">
      <Radio id="radio-disabled" name="radio-single-disabled" disabled />
      <span>Option</span>
    </label>
  ),
};

export const Group: Story = {
  render: () => (
    <fieldset className="space-y-2">
      <legend className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
        Aktion
      </legend>
      <label htmlFor="radio-group-a" className="flex items-center gap-2">
        <Radio id="radio-group-a" name="radio-group" defaultChecked />
        <span>Erste Option</span>
      </label>
      <label htmlFor="radio-group-b" className="flex items-center gap-2">
        <Radio id="radio-group-b" name="radio-group" />
        <span>Zweite Option</span>
      </label>
    </fieldset>
  ),
};

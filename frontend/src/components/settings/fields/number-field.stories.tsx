import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { NumberField } from "./number-field";

const meta = {
  title: "components/settings/fields/NumberField",
  component: NumberField,
  args: {
    value: 30,
    onChange: () => undefined,
  },
} satisfies Meta<typeof NumberField>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithMinMax: Story = {
  args: {
    value: 15,
    min: 0,
    max: 60,
  },
};

export const Disabled: Story = {
  args: {
    value: 30,
    disabled: true,
  },
};

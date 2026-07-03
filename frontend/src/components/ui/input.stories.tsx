import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Input } from "./input";

const meta = {
  title: "components/ui/Input",
  component: Input,
  args: {
    name: "example",
    label: "Name",
    placeholder: "Bitte eingeben",
  },
} satisfies Meta<typeof Input>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
};

export const Compact: Story = {
  args: {
    controlSize: "compact",
  },
};

export const WithError: Story = {
  args: {
    error: "Dieses Feld ist erforderlich.",
  },
};

export const Disabled: Story = {
  args: {
    disabled: true,
    value: "Nicht bearbeitbar",
  },
};

export const NoLabel: Story = {
  args: {
    label: undefined,
  },
};

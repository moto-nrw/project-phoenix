import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ComponentProps } from "react";
import { useState } from "react";
import { CustomSelect } from "./custom-select";

const OPTIONS = [
  { value: "group-a", label: "Gruppe A" },
  { value: "group-b", label: "Gruppe B" },
  { value: "group-c", label: "Gruppe C", disabled: true },
] as const;

function ControlledCustomSelect(props: ComponentProps<typeof CustomSelect>) {
  const [value, setValue] = useState(props.value);
  return <CustomSelect {...props} value={value} onChange={setValue} />;
}

const meta = {
  title: "components/ui/CustomSelect",
  component: CustomSelect,
  parameters: {
    layout: "padded",
  },
  args: {
    value: "",
    onChange: () => undefined,
    options: OPTIONS,
    ariaLabel: "Gruppe auswählen",
  },
} satisfies Meta<typeof CustomSelect>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: (args) => <ControlledCustomSelect {...args} />,
  args: {
    options: OPTIONS,
    ariaLabel: "Gruppe auswählen",
  },
};

export const WithPlaceholder: Story = {
  render: (args) => <ControlledCustomSelect {...args} />,
  args: {
    options: OPTIONS,
    ariaLabel: "Gruppe auswählen",
    placeholder: "Bitte Gruppe wählen",
  },
};

export const Disabled: Story = {
  render: (args) => <ControlledCustomSelect {...args} />,
  args: {
    options: OPTIONS,
    ariaLabel: "Gruppe auswählen",
    disabled: true,
  },
};

export const Invalid: Story = {
  render: (args) => <ControlledCustomSelect {...args} />,
  args: {
    options: OPTIONS,
    ariaLabel: "Gruppe auswählen",
    invalid: true,
    required: true,
  },
};

export const Empty: Story = {
  render: (args) => <ControlledCustomSelect {...args} />,
  args: {
    options: [],
    ariaLabel: "Gruppe auswählen",
  },
};

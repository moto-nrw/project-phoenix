import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ComponentProps } from "react";
import { useState } from "react";
import { MultiCheckboxSelect } from "./multi-checkbox-select";

const OPTIONS = [
  { value: "group-a", label: "Gruppe A" },
  { value: "group-b", label: "Gruppe B", badge: "Neu" },
  { value: "group-c", label: "Gruppe C", disabled: true },
] as const;

function ControlledMultiCheckboxSelect(
  props: ComponentProps<typeof MultiCheckboxSelect>,
) {
  const [value, setValue] = useState<string[]>([...props.value]);
  return <MultiCheckboxSelect {...props} value={value} onChange={setValue} />;
}

const meta = {
  title: "components/ui/MultiCheckboxSelect",
  component: MultiCheckboxSelect,
  parameters: {
    layout: "padded",
  },
  args: {
    value: [],
    onChange: () => undefined,
    options: OPTIONS,
    ariaLabel: "Gruppen auswählen",
  },
} satisfies Meta<typeof MultiCheckboxSelect>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: (args) => <ControlledMultiCheckboxSelect {...args} />,
};

export const WithSelection: Story = {
  render: (args) => <ControlledMultiCheckboxSelect {...args} />,
  args: {
    value: ["group-a"],
  },
};

export const MultipleSelected: Story = {
  render: (args) => <ControlledMultiCheckboxSelect {...args} />,
  args: {
    value: ["group-a", "group-b"],
  },
};

export const Disabled: Story = {
  render: (args) => <ControlledMultiCheckboxSelect {...args} />,
  args: {
    disabled: true,
  },
};

export const Empty: Story = {
  render: (args) => <ControlledMultiCheckboxSelect {...args} />,
  args: {
    options: [],
  },
};

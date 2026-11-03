import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ComponentProps } from "react";
import { useState } from "react";
import { MultiSelect } from "./multi-select";

const OPTIONS = [
  { value: "group-a", label: "Gruppe A" },
  { value: "group-b", label: "Gruppe B", badge: "Neu" },
  { value: "group-c", label: "Gruppe C", disabled: true },
] as const;

function ControlledMultiSelect(props: ComponentProps<typeof MultiSelect>) {
  const [value, setValue] = useState<string[]>([...props.value]);
  return <MultiSelect {...props} value={value} onChange={setValue} />;
}

const meta = {
  title: "components/ui/MultiSelect",
  component: MultiSelect,
  parameters: {
    layout: "padded",
  },
  args: {
    value: [],
    onChange: () => undefined,
    options: OPTIONS,
    ariaLabel: "Gruppen auswählen",
    placeholder: "Alle Gruppen",
    className: "w-64",
  },
} satisfies Meta<typeof MultiSelect>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: (args) => <ControlledMultiSelect {...args} />,
};

export const WithSelection: Story = {
  render: (args) => <ControlledMultiSelect {...args} />,
  args: {
    value: ["group-a"],
  },
};

export const MultipleSelected: Story = {
  render: (args) => <ControlledMultiSelect {...args} />,
  args: {
    value: ["group-a", "group-b"],
  },
};

export const Disabled: Story = {
  render: (args) => <ControlledMultiSelect {...args} />,
  args: {
    disabled: true,
  },
};

export const Empty: Story = {
  render: (args) => <ControlledMultiSelect {...args} />,
  args: {
    options: [],
    unavailableLabel: "Keine Gruppen vorhanden",
  },
};

export const SearchableWithSelectAll: Story = {
  render: (args) => <ControlledMultiSelect {...args} />,
  args: {
    searchable: true,
    selectAll: true,
    searchPlaceholder: "Gruppe suchen",
  },
};

export const Invalid: Story = {
  render: (args) => <ControlledMultiSelect {...args} />,
  args: {
    invalid: true,
  },
};

import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import {
  ListboxDropdown,
  type ListboxDropdownOption,
} from "~/components/ui/listbox-dropdown";

const fruitOptions: readonly ListboxDropdownOption<string>[] = [
  { value: "apple", label: "Apfel" },
  { value: "banana", label: "Banane" },
  { value: "cherry", label: "Kirsche", disabled: true },
];

function ControlledListboxDropdown(
  props: Omit<
    Parameters<typeof ListboxDropdown<string>>[0],
    "value" | "onChange"
  > & { initialValue: string },
) {
  const { initialValue, ...rest } = props;
  const [value, setValue] = useState(initialValue);
  return <ListboxDropdown {...rest} value={value} onChange={setValue} />;
}

const meta: Meta<typeof ControlledListboxDropdown> = {
  title: "components/ui/ListboxDropdown",
  component: ControlledListboxDropdown,
  args: {
    initialValue: "apple",
    options: fruitOptions,
    ariaLabel: "Frucht auswählen",
    className: "w-64",
  },
};

export default meta;

type Story = StoryObj<typeof ControlledListboxDropdown>;

export const Default: Story = {};

export const Compact: Story = {
  args: {
    size: "compact",
  },
};

export const Disabled: Story = {
  args: {
    disabled: true,
  },
};

export const Invalid: Story = {
  args: {
    invalid: true,
    required: true,
  },
};

export const EmptyValuePlaceholder: Story = {
  args: {
    initialValue: "",
    placeholder: "Bitte wählen",
  },
};

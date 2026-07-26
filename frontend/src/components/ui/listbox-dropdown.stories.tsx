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
    className:
      "flex items-center justify-between gap-2 rounded-md border border-gray-300 bg-white px-4 py-2 text-sm",
    menuClassName:
      "overflow-y-auto rounded-md border border-gray-200 bg-white p-1 shadow-lg",
    optionClassName: "block w-full rounded-md px-3 py-2 text-left text-sm",
    activeOptionClassName:
      "block w-full rounded-md bg-gray-100 px-3 py-2 text-left text-sm",
    disabledOptionClassName:
      "block w-full rounded-md px-3 py-2 text-left text-sm text-gray-400",
  },
};

export default meta;

type Story = StoryObj<typeof ControlledListboxDropdown>;

export const Default: Story = {};

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

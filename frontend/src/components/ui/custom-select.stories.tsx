import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ComponentProps } from "react";
import { useState } from "react";
import { expect, screen, userEvent, within } from "storybook/test";
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

/** Beschriftung, Hinweis und Fehler aus der Feld-Hülle (#6579). */
export const WithLabelHintAndError: Story = {
  render: (args) => <ControlledCustomSelect {...args} />,
  args: {
    options: OPTIONS,
    ariaLabel: undefined,
    label: "Gruppe",
    required: true,
    hint: "Die Gruppe, in der das Kind betreut wird.",
    error: "Bitte wählen Sie eine Gruppe.",
  },
};

/** Dichte Zeilen: die kleinere Feldhöhe. */
export const Compact: Story = {
  render: (args) => <ControlledCustomSelect {...args} />,
  args: {
    options: OPTIONS,
    value: "group-a",
    ariaLabel: "Gruppe auswählen",
    controlSize: "compact",
  },
};

/** Offene Liste mit Suchfeld (#6576, Baseline des Pixel-Vergleichs). */
export const SearchableOpened: Story = {
  render: (args) => <ControlledCustomSelect {...args} />,
  args: {
    options: OPTIONS,
    ariaLabel: "Gruppe auswählen",
    searchable: true,
  },
  play: async ({ canvasElement }) => {
    await userEvent.click(
      within(canvasElement).getByRole("combobox", { name: "Gruppe auswählen" }),
    );
    await expect(
      await screen.findByRole("textbox", { name: "Suchen" }),
    ).toBeVisible();
  },
};

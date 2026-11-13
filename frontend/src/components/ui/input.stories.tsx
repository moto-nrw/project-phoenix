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

/** Pflichtfeld mit Hinweis und Fehler unter dem Feld. */
export const RequiredWithHintAndError: Story = {
  args: {
    required: true,
    hint: "So, wie er im Ausweis steht.",
    error: "Bitte geben Sie einen Namen ein.",
  },
};

/** Einzelnes Feld im Telefon-Layout: 44 px unter `lg`. */
export const Touch: Story = {
  args: {
    controlSize: "touch",
  },
};

/** Passwortfeld mit Augen-Knopf. */
export const Revealable: Story = {
  args: {
    label: "Passwort",
    type: "password",
    revealable: true,
    value: "geheim",
    readOnly: true,
  },
};

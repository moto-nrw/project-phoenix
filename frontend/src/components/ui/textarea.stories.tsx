import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Textarea } from "./textarea";

const meta = {
  title: "components/ui/Textarea",
  component: Textarea,
  args: {
    name: "note",
    label: "Notiz",
    placeholder: "Bitte eingeben",
  },
} satisfies Meta<typeof Textarea>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = { args: {} };

/** Pflichtfeld mit Hinweis und Fehler unter dem Feld. */
export const RequiredWithHintAndError: Story = {
  args: {
    required: true,
    hint: "Höchstens 500 Zeichen.",
    error: "Bitte geben Sie eine Notiz ein.",
  },
};

export const NoLabel: Story = {
  args: { label: undefined, "aria-label": "Notiz" },
};

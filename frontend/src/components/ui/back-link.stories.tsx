import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { BackAction, BackLink } from "./back-link";

const meta = {
  title: "components/ui/BackLink",
  component: BackLink,
  parameters: {
    layout: "padded",
  },
} satisfies Meta<typeof BackLink>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    href: "/rooms",
    label: "Zurück zu den Räumen",
  },
};

/** Ansicht ohne eigene Adresse: derselbe Rückweg als Knopf. */
export const Action: Story = {
  args: {
    href: "",
    label: "Zurück zur Übersicht",
  },
  render: ({ label }) => <BackAction onClick={() => {}} label={label} />,
};

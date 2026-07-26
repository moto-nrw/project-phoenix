import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { DatabaseCreateAction } from "./database-create-action";

const meta: Meta<typeof DatabaseCreateAction> = {
  title: "database/DatabaseCreateAction",
  component: DatabaseCreateAction,
  args: {
    label: "Erstellen",
    ariaLabel: "Neuen Eintrag erstellen",
    onClick: () => undefined,
  },
};

export default meta;

type Story = StoryObj<typeof DatabaseCreateAction>;

export const Default: Story = {};

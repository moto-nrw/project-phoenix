import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Database } from "lucide-react";

import { DatabaseEmptyState } from "./database-empty-state";

const meta: Meta<typeof DatabaseEmptyState> = {
  title: "components/database/DatabaseEmptyState",
  component: DatabaseEmptyState,
};

export default meta;

type Story = StoryObj<typeof DatabaseEmptyState>;

export const Default: Story = {
  args: {
    icon: <Database className="mx-auto h-12 w-12 text-gray-400" />,
    title: "Keine Einträge gefunden",
    description:
      "Es wurden keine Daten gefunden, die den Kriterien entsprechen.",
  },
};

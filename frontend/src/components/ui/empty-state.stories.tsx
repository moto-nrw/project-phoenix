import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Lightbulb, Search } from "lucide-react";

import { Button } from "./button";
import { EmptyState } from "./empty-state";

const meta = {
  title: "components/ui/EmptyState",
  component: EmptyState,
} satisfies Meta<typeof EmptyState>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Basic: Story = {
  args: {
    title: "Keine Ergebnisse gefunden",
    description: "Versuche einen anderen Suchbegriff.",
  },
};

export const WithIcon: Story = {
  args: {
    icon: <Search className="h-12 w-12" strokeWidth={1.5} />,
    title: "Keine Ergebnisse gefunden",
    description: "Versuche einen anderen Suchbegriff.",
  },
};

export const WithAction: Story = {
  args: {
    icon: <Lightbulb className="h-12 w-12" strokeWidth={1.5} />,
    title: "Noch keine Beiträge",
    description: "Teile Ideen, melde Probleme oder schlage Verbesserungen vor.",
    action: (
      <Button type="button" size="md">
        Neuer Beitrag
      </Button>
    ),
  },
};

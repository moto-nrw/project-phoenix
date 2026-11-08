import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Button } from "./button";
import { SectionCard } from "./section-card";

const meta = {
  title: "components/ui/SectionCard",
  component: SectionCard,
} satisfies Meta<typeof SectionCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const WithTitle: Story = {
  args: {
    title: "Formularvorlagen",
    description: "Jede Anmeldephase nutzt eine Vorlage.",
    children: <p className="text-sm text-gray-700">Inhalt</p>,
  },
};

/**
 * Eine Karte unter vielen gleichartigen (#5594): `as="article"` und
 * `hoverElevated`. Die Karte selbst ist kein Ziel, ihre Bedienelemente
 * liegen im Inhalt.
 */
export const ArticleWithHover: Story = {
  args: {
    as: "article",
    hoverElevated: true,
    title: "Lernzeit Jahrgang 1",
    titleSize: "sm",
    description: "Betreuung · Gruppenraum",
    children: (
      <Button type="button" variant="primary" size="compact" className="w-full">
        Termine erzeugen
      </Button>
    ),
  },
  render: (args) => (
    <div className="w-72">
      <SectionCard {...args} />
    </div>
  ),
};

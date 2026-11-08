import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ExternalLink } from "lucide-react";

import { TileCard } from "./tile-card";

const meta = {
  title: "components/ui/TileCard",
  component: TileCard,
} satisfies Meta<typeof TileCard>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Eine Kachel, die ihr Ziel in einem neuen Tab öffnet (#5594). */
export const NewTab: Story = {
  args: {
    href: "/anmeldung/preview?schemaId=1",
    newTab: true,
    className: "flex items-start gap-3",
    children: (
      <>
        <ExternalLink className="h-4 w-4 text-gray-600" aria-hidden="true" />
        <span className="min-w-0">
          <span className="block text-sm font-semibold text-gray-900">
            Vorschau öffnen
          </span>
          <span className="mt-0.5 block text-xs leading-5 text-gray-500">
            Öffnet die Formularvorschau in einem neuen Tab.
          </span>
        </span>
      </>
    ),
  },
  render: (args) => (
    <div className="w-80">
      <TileCard {...args} />
    </div>
  ),
};

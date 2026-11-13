import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ExternalLink } from "lucide-react";
import { expect, fn, userEvent, within } from "storybook/test";

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

const TILE_CONTENT = (
  <span className="min-w-0">
    <span className="block text-sm font-semibold text-gray-900">
      OGS-Raum 1
    </span>
    <span className="mt-0.5 block text-xs leading-5 text-gray-500">
      Erdgeschoss · 24 Plätze
    </span>
  </span>
);

function inTileColumn(args: Story["args"]) {
  return (
    <div className="w-80">
      <TileCard {...args}>{args.children}</TileCard>
    </div>
  );
}

/** Kachel, die navigiert. */
export const AsLink: Story = {
  args: { href: "/rooms/1", children: TILE_CONTENT },
  render: inTileColumn,
};

/** Kachel ohne Ziel: eine Schaltfläche, etwa für ein Detail-Panel. */
export const AsButton: Story = {
  args: { onClick: fn(), ariaHasPopup: "dialog", children: TILE_CONTENT },
  render: inTileColumn,
};

/** Ohne Innenabstand, wenn der Inhalt seine Ränder selbst setzt. */
export const PaddingNone: Story = {
  args: {
    href: "/activities/1",
    padding: "none",
    children: (
      <span className="block">
        <span className="block bg-gray-50 px-5 py-3 text-sm font-semibold text-gray-900">
          Fußball-AG
        </span>
        <span className="block px-5 py-3 text-xs text-gray-500">
          Montag · 14:00 bis 15:30
        </span>
      </span>
    ),
  },
  render: inTileColumn,
};

export const Disabled: Story = {
  args: { href: "/rooms/1", disabled: true, children: TILE_CONTENT },
  render: inTileColumn,
};

/** Tastaturfokus: `play` springt per Tab auf die Kachel (#6565). */
export const LinkFocused: Story = {
  ...AsLink,
  play: async ({ canvasElement }) => {
    await userEvent.tab();
    await expect(within(canvasElement).getByRole("link")).toHaveFocus();
  },
};

export const ButtonFocused: Story = {
  ...AsButton,
  play: async ({ canvasElement }) => {
    await userEvent.tab();
    await expect(within(canvasElement).getByRole("button")).toHaveFocus();
  },
};

import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, screen, userEvent, within } from "storybook/test";

import { backConcept } from "~/lib/concepts/back";
import { helpConcept } from "~/lib/concepts/help";
import { languageConcept } from "~/lib/concepts/language";
import { moreConcept } from "~/lib/concepts/more";
import { notificationsConcept } from "~/lib/concepts/notifications";
import { passkeysConcept } from "~/lib/concepts/passkeys";
import { privacyConcept } from "~/lib/concepts/privacy";
import { settingsConcept } from "~/lib/concepts/settings";
import {
  PortalOverflowBottomNav,
  type PortalOverflowNavItem,
} from "./portal-overflow-bottom-nav";

const ITEMS: readonly PortalOverflowNavItem[] = [
  {
    key: "back",
    href: "/",
    label: "Zurück",
    concept: backConcept,
    active: false,
  },
  {
    key: "general",
    href: "/settings",
    label: "Allgemein",
    concept: settingsConcept,
    active: true,
  },
  {
    key: "notifications",
    href: "/settings/notifications",
    label: "Benachrichtigungen",
    concept: notificationsConcept,
    active: false,
  },
  {
    key: "language",
    href: "/settings/language",
    label: "Sprache",
    concept: languageConcept,
    active: false,
  },
];

const CROWDED: readonly PortalOverflowNavItem[] = [
  ...ITEMS,
  {
    key: "privacy",
    href: "/settings/privacy",
    label: "Datenschutz",
    concept: privacyConcept,
    active: false,
  },
  {
    key: "passkeys",
    href: "/settings/passkeys",
    label: "Passkeys",
    concept: passkeysConcept,
    active: false,
  },
  {
    key: "help",
    href: "/help",
    label: "Hilfe",
    concept: helpConcept,
    active: false,
    newTab: true,
    separated: true,
  },
];

const meta = {
  title: "components/ui/PortalOverflowBottomNav",
  component: PortalOverflowBottomNav,
  parameters: { layout: "fullscreen" },
  args: {
    items: ITEMS,
    navItemAttribute: "data-story-nav-item",
    moreConcept,
  },
  decorators: [
    (StoryFn) => (
      <div className="min-h-[200px] bg-gray-50">
        <StoryFn />
      </div>
    ),
  ],
} satisfies Meta<typeof PortalOverflowBottomNav>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Bis fünf Ziele stehen alle in der Leiste. */
export const FitsBar: Story = {};

/** Ab sechs Zielen stehen die ersten vier in der Leiste, der Rest hinter „Mehr“. */
export const Crowded: Story = {
  args: { items: CROWDED },
};

export const MoreOpened: Story = {
  args: { items: CROWDED },
  play: async ({ canvasElement }) => {
    const more = within(canvasElement).queryByRole("button", { name: "Mehr" });
    // Ab 1024 px blendet CSS die Leiste aus; dort bleibt das Bild leer.
    if (more === null) return;
    await userEvent.click(more);
    await expect(await screen.findByRole("dialog")).toBeVisible();
  },
};

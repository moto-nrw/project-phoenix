import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, screen, userEvent, within } from "storybook/test";

import { backConcept } from "~/lib/concepts/back";
import { calendarConcept } from "~/lib/concepts/calendar";
import { documentsConcept } from "~/lib/concepts/documents";
import { helpConcept } from "~/lib/concepts/help";
import { homeConcept } from "~/lib/concepts/home";
import { messagesConcept } from "~/lib/concepts/messages";
import { settingsConcept } from "~/lib/concepts/settings";
import {
  PortalOverflowBottomNav,
  type PortalOverflowNavItem,
} from "./portal-overflow-bottom-nav";

const ITEMS: readonly PortalOverflowNavItem[] = [
  {
    key: "back",
    href: "#back",
    label: "Zurück zur App",
    concept: backConcept,
    active: false,
  },
  {
    key: "home",
    href: "#home",
    label: "Start",
    concept: homeConcept,
    active: true,
  },
  {
    key: "messages",
    href: "#messages",
    label: "Nachrichten",
    concept: messagesConcept,
    active: false,
  },
  {
    key: "calendar",
    href: "#calendar",
    label: "Kalender",
    concept: calendarConcept,
    active: false,
  },
  {
    key: "documents",
    href: "#documents",
    label: "Dokumente",
    concept: documentsConcept,
    active: false,
  },
  {
    key: "settings",
    href: "#settings",
    label: "Einstellungen",
    concept: settingsConcept,
    active: false,
  },
  {
    key: "help",
    href: "#help",
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
  parameters: {
    layout: "fullscreen",
  },
  decorators: [
    (Story) => (
      <div className="min-h-[640px] bg-gray-50">
        <Story />
      </div>
    ),
  ],
  args: {
    items: ITEMS,
    navItemAttribute: "data-story-nav-item",
  },
} satisfies Meta<typeof PortalOverflowBottomNav>;

export default meta;

type Story = StoryObj<typeof meta>;

/** Bis fünf Ziele: alle in der Leiste, kein „Mehr“. */
export const Few: Story = {
  args: { items: ITEMS.slice(0, 4) },
};

/** Ab sechs Zielen: vier in der Leiste, der Rest hinter „Mehr“. */
export const Crowded: Story = {};

/** Das aktive Ziel steht hinter „Mehr“: „Mehr“ leuchtet. */
export const ActiveBehindMore: Story = {
  args: {
    items: ITEMS.map((item) => ({ ...item, active: item.key === "settings" })),
  },
};

/** Das geöffnete „Mehr“-Blatt. */
export const MoreOpened: Story = {
  play: async ({ canvasElement }) => {
    // Ab 1024 px blendet CSS die Leiste aus; dort gibt es nichts zu öffnen.
    const more = within(canvasElement).queryByRole("button", { name: "Mehr" });
    if (!more) return;
    await userEvent.click(more);
    await expect(await screen.findByRole("dialog")).toBeVisible();
  },
};

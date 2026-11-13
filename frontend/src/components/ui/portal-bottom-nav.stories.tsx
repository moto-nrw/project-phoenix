import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { homeConcept } from "~/lib/concepts/home";
import { logoutConcept } from "~/lib/concepts/logout";
import { messagesConcept } from "~/lib/concepts/messages";
import { moreConcept } from "~/lib/concepts/more";
import { settingsConcept } from "~/lib/concepts/settings";
import {
  PortalBottomNav,
  PortalBottomNavButton,
  PortalBottomNavLink,
  PortalNavSheetButton,
  PortalNavSheetLink,
} from "./portal-bottom-nav";

const ITEM = {
  paddingClassName: "px-3 py-2",
  labelClassName: "ml-2 text-sm font-semibold",
  quiet: true,
  shrinkActive: true,
};

const meta = {
  title: "components/ui/PortalBottomNav",
  component: PortalBottomNav,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    listClassName: "gap-1",
    children: null,
  },
} satisfies Meta<typeof PortalBottomNav>;

export default meta;

type Story = StoryObj<typeof meta>;

/** Die mobile Leiste; ab 1024 px blendet CSS sie aus. */
export const Bar: Story = {
  render: (args) => (
    <div className="min-h-[200px] bg-gray-50">
      <PortalBottomNav {...args}>
        <PortalBottomNavLink
          {...ITEM}
          href="/dashboard"
          label="Start"
          concept={homeConcept}
          active
        />
        <PortalBottomNavLink
          {...ITEM}
          href="/messages"
          label="Nachrichten"
          concept={messagesConcept}
          active={false}
        />
        <PortalBottomNavLink
          {...ITEM}
          href="/settings"
          label="Einstellungen"
          concept={settingsConcept}
          active={false}
        />
        <PortalBottomNavButton
          {...ITEM}
          label="Mehr"
          concept={moreConcept}
          active={false}
        />
      </PortalBottomNav>
    </div>
  ),
};

/** Die Zeilen im „Mehr“-Blatt. */
export const SheetRows: Story = {
  parameters: {
    layout: "padded",
  },
  render: () => (
    <ul className="flex max-w-sm flex-col gap-2">
      <li>
        <PortalNavSheetLink
          href="/messages"
          label="Nachrichten"
          concept={messagesConcept}
          active
        />
      </li>
      <li>
        <PortalNavSheetLink
          href="/settings"
          label="Einstellungen"
          concept={settingsConcept}
          active={false}
        />
      </li>
      <li>
        <PortalNavSheetLink
          href="/dashboard"
          label="Start"
          concept={homeConcept}
          active={false}
          plain
        />
      </li>
      <li>
        <PortalNavSheetButton label="Abmelden" concept={logoutConcept} />
      </li>
    </ul>
  ),
};

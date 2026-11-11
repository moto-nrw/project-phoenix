import type { Decorator, Meta, StoryObj } from "@storybook/nextjs-vite";
import { UsersIcon } from "@phosphor-icons/react";

import { ModuleLockMark } from "./module-lock-mark";
import { NavSceneIcon } from "./nav-scene-icon/nav-scene-icon";
import { SIDEBAR_ICON_CLASSES } from "./sidebar-geometry";
import {
  SidebarRowButton,
  SidebarRowLink,
  SidebarRowPlaceholder,
  SidebarSectionHeading,
} from "./sidebar-row";

// Zeile mit Zähler, ausgeklappt und im 64-px-Streifen: das Vorbild für
// Gruppe und Akkordeon-Bereich (#6566).
const meta = {
  title: "components/ui/SidebarRowLink",
  component: SidebarRowLink,
  parameters: {
    layout: "padded",
  },
  decorators: [
    (Story) => (
      <div className="w-64">
        <Story />
      </div>
    ),
  ],
  args: {
    href: "#",
    icon: <NavSceneIcon icon={UsersIcon} className={SIDEBAR_ICON_CLASSES} />,
    label: "Nachrichten",
    active: false,
  },
} satisfies Meta<typeof SidebarRowLink>;

export default meta;

type Story = StoryObj<typeof meta>;

export const WithCount: Story = {
  args: {
    badge: { count: 3, tone: "parents", ariaLabel: "3 ungelesene Nachrichten" },
  },
};

export const RailWithCount: Story = {
  decorators: [
    (Story) => (
      <div className="w-11">
        <Story />
      </div>
    ),
  ],
  args: {
    badge: { count: 3, tone: "parents", ariaLabel: "3 ungelesene Nachrichten" },
    collapsed: true,
    labelsMounted: false,
    labelsVisible: false,
  },
};

const RAIL: Decorator = (Story) => (
  <div className="w-11">
    <Story />
  </div>
);

const COUNT = {
  count: 3,
  tone: "parents",
  ariaLabel: "3 ungelesene Nachrichten",
} as const;

export const Active: Story = {
  args: { active: true, badge: COUNT },
};

export const Locked: Story = {
  args: {
    trailing: <ModuleLockMark />,
    collapsedLabel: "Nachrichten, nicht gebucht",
  },
};

export const RailActive: Story = {
  decorators: [RAIL],
  args: {
    active: true,
    collapsed: true,
    labelsMounted: false,
    labelsVisible: false,
  },
};

/** Mitten in der Blende zum Streifen: Zähler rechts und Punkt stehen beide im Baum. */
export const FadingToRail: Story = {
  args: {
    badge: COUNT,
    trailing: <ModuleLockMark />,
    collapsed: true,
    labelsMounted: true,
    labelsVisible: false,
  },
};

export const Button: Story = {
  render: (args) => (
    <SidebarRowButton icon={args.icon} label="Abmelden" onClick={() => {}} />
  ),
};

export const Placeholder: Story = {
  render: (args) => (
    <SidebarRowPlaceholder
      icon={args.icon}
      label="Speiseplan"
      hint="Bald"
      title="Kommt bald"
    />
  ),
};

export const SectionHeading: Story = {
  render: (args) => (
    <div>
      <SidebarSectionHeading>Verwaltung</SidebarSectionHeading>
      <SidebarRowLink {...args} />
    </div>
  ),
};

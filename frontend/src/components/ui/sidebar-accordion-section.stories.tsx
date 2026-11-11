import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { groupsConcept } from "~/lib/concepts/groups";
import { SidebarAccordionSection } from "./sidebar-accordion-section";

const meta = {
  title: "components/ui/SidebarAccordionSection",
  component: SidebarAccordionSection,
  args: {
    concept: groupsConcept,
    label: "Gruppen",
    isExpanded: false,
    onToggle: () => {},
    isActive: false,
    hasChildren: false,
  },
} satisfies Meta<typeof SidebarAccordionSection>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Collapsed: Story = {
  args: {
    isExpanded: false,
  },
};

export const ExpandedWithChildren: Story = {
  args: {
    isExpanded: true,
    isActive: true,
    isIconActive: true,
    hasChildren: true,
    children: (
      <div className="space-y-1 pr-3 pl-10 text-sm text-gray-700">
        <div>Gruppe A</div>
        <div>Gruppe B</div>
      </div>
    ),
  },
};

export const ExpandedEmpty: Story = {
  args: {
    isExpanded: true,
    isActive: true,
    hasChildren: false,
    emptyText: "Keine Gruppen vorhanden",
  },
};

export const Loading: Story = {
  args: {
    isExpanded: true,
    isLoading: true,
  },
};

export const CollapsedWithCount: Story = {
  args: {
    label: "Anmeldungen",
    badgeCount: 3,
    badgeTone: "staff",
    badgeNoun: "ungelesene Anmeldungen",
  },
};

export const CollapsedWithZeroCount: Story = {
  args: {
    label: "Anmeldungen mit einer langen Bezeichnung",
    badgeCount: 0,
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
    label: "Anmeldungen",
    badgeCount: 3,
    badgeTone: "staff",
    badgeNoun: "ungelesene Anmeldungen",
    collapsed: true,
    labelsMounted: false,
    labelsVisible: false,
  },
};

export const CollapsedActive: Story = {
  args: {
    isActive: true,
  },
};

export const RailActive: Story = {
  decorators: [
    (Story) => (
      <div className="w-11">
        <Story />
      </div>
    ),
  ],
  args: {
    isActive: false,
    isIconActive: true,
    collapsed: true,
    labelsMounted: false,
    labelsVisible: false,
  },
};

/** Mitten in der Blende zum Streifen: Zähler rechts und Punkt stehen beide im Baum. */
export const FadingWithCount: Story = {
  args: {
    label: "Anmeldungen",
    badgeCount: 3,
    badgeTone: "staff",
    badgeNoun: "ungelesene Anmeldungen",
    collapsed: true,
    labelsMounted: true,
    labelsVisible: false,
  },
};

import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { SidebarAccordionSection } from "./sidebar-accordion-section";

const meta = {
  title: "components/dashboard/SidebarAccordionSection",
  component: SidebarAccordionSection,
  args: {
    icon: "M12 6v6m0 0v6m0-6h6m-6 0H6",
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
    activeColor: "text-moto-green",
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

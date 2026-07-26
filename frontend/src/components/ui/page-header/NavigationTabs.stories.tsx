import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { NavigationTabs } from "./NavigationTabs";

const meta = {
  title: "ui/page-header/NavigationTabs",
  component: NavigationTabs,
  args: {
    items: [
      { id: "overview", label: "Übersicht" },
      { id: "students", label: "Schüler" },
      { id: "settings", label: "Einstellungen" },
    ],
    activeTab: "overview",
    onTabChange: () => {},
  },
} satisfies Meta<typeof NavigationTabs>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const ManyTabsMobileDropdown: Story = {
  args: {
    items: [
      { id: "overview", label: "Übersicht" },
      { id: "students", label: "Schüler" },
      { id: "settings", label: "Einstellungen" },
      { id: "activities", label: "Aktivitäten" },
      { id: "feedback", label: "Feedback" },
    ],
    activeTab: "activities",
  },
};

export const SingleTab: Story = {
  args: {
    items: [{ id: "overview", label: "Übersicht" }],
    activeTab: "overview",
  },
};

export const Interactive: Story = {
  render: (args) => {
    function InteractiveNavigationTabs() {
      const [activeTab, setActiveTab] = useState(args.items[0]?.id ?? "");
      return (
        <NavigationTabs
          {...args}
          activeTab={activeTab}
          onTabChange={setActiveTab}
        />
      );
    }
    return <InteractiveNavigationTabs />;
  },
};

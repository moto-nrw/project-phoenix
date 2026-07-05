import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { SettingsLayout } from "~/components/shared/settings-layout";

const tabs = [
  {
    id: "settings-general",
    label: "Allgemein",
    icon: "M12 6v6l4 2",
  },
  {
    id: "settings-security",
    label: "Sicherheit",
    icon: "M12 2l7 4v6c0 5-3.5 8-7 10-3.5-2-7-5-7-10V6l7-4z",
  },
  {
    id: "settings-admin",
    label: "Verwaltung",
    icon: "M4 6h16M4 12h16M4 18h16",
    adminOnly: true,
  },
];

const meta: Meta<typeof SettingsLayout> = {
  title: "components/shared/SettingsLayout",
  component: SettingsLayout,
  parameters: {
    nextjs: {
      appDirectory: true,
      navigation: {
        pathname: "/settings",
        query: {},
      },
    },
  },
  args: {
    tabs,
    renderTab: (tabId: string) => (
      <div className="p-4 text-sm text-gray-700">Inhalt für {tabId}</div>
    ),
  },
};

export default meta;
type Story = StoryObj<typeof SettingsLayout>;

export const Default: Story = {};

export const WithRequestedTab: Story = {
  parameters: {
    nextjs: {
      appDirectory: true,
      navigation: {
        pathname: "/settings",
        query: { tab: "security" },
      },
    },
  },
};

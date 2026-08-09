import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { SettingsLayout } from "~/components/shared/settings-layout";

const tabs = [
  {
    id: "settings-general",
    label: "Allgemein",
    icon: "settings" as const,
  },
  {
    id: "settings-security",
    label: "Sicherheit",
    icon: "permissions" as const,
  },
  {
    id: "settings-admin",
    label: "Verwaltung",
    icon: "devices" as const,
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

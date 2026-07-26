import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { SchemaTab } from "~/lib/settings-api";
import { EnrollmentLinkPanel } from "./enrollment-link-panel";

function buildTab(enabled: boolean): SchemaTab {
  return {
    key: "enrollment",
    label: "Anmeldung",
    categories: [
      {
        key: "general",
        label: "Allgemein",
        items: [
          {
            key: "enrollment.enabled",
            label: "Anmeldung aktiviert",
            description: "Öffentliche Anmeldung für Eltern aktivieren.",
            type: "boolean",
            default: false,
            value: enabled,
            is_default: false,
            writable: true,
            visible: true,
            sort_order: 0,
            access_policy: "shared",
          },
        ],
      },
    ],
  };
}

const meta = {
  title: "settings/EnrollmentLinkPanel",
  component: EnrollmentLinkPanel,
} satisfies Meta<typeof EnrollmentLinkPanel>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Enabled: Story = {
  args: {
    tab: buildTab(true),
  },
};

export const Disabled: Story = {
  args: {
    tab: buildTab(false),
  },
};

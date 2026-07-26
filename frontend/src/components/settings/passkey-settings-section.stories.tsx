import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { PasskeySettingsSection } from "~/components/settings/passkey-settings-section";

const meta = {
  title: "settings/PasskeySettingsSection",
  component: PasskeySettingsSection,
  parameters: {
    // The component calls listPasskeys() on mount, which hits the real
    // /api/auth/passkeys route via fetch. In Storybook this request fails
    // (no backend), which the component surfaces as an error alert — that
    // is expected and harmless for demoing the static layout.
    docs: { description: { component: "" } },
  },
} satisfies Meta<typeof PasskeySettingsSection>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default tenant-scoped passkey section, as embedded on the tenant
 * settings page.
 */
export const Default: Story = {
  args: {
    scope: "tenant",
  },
};

/**
 * Operator-scoped passkey section, as embedded on the operator settings
 * page.
 */
export const OperatorScope: Story = {
  args: {
    scope: "operator",
  },
};

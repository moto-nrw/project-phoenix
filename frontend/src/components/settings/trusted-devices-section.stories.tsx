import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { TrustedDevicesSection } from "~/components/settings/trusted-devices-section";

const meta = {
  title: "settings/TrustedDevicesSection",
  component: TrustedDevicesSection,
  parameters: {
    // The component calls listTrustedDevices() on mount, which hits the
    // real /api/mfa/trusted-devices route via fetch. In Storybook this
    // request fails (no backend), which the component surfaces as an
    // error alert — that is expected and harmless for demoing the static
    // layout.
    docs: { description: { component: "" } },
  },
} satisfies Meta<typeof TrustedDevicesSection>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default tenant-scoped trusted devices section, as embedded on the
 * tenant settings page.
 */
export const Default: Story = {
  args: {
    scope: "tenant",
  },
};

/**
 * Operator-scoped trusted devices section, as embedded on the operator
 * settings page.
 */
export const OperatorScope: Story = {
  args: {
    scope: "operator",
  },
};

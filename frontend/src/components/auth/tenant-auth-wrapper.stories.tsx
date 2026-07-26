import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { TenantAuthWrapper } from "~/components/auth/tenant-auth-wrapper";

const meta: Meta<typeof TenantAuthWrapper> = {
  title: "components/auth/TenantAuthWrapper",
  component: TenantAuthWrapper,
};

export default meta;
type Story = StoryObj<typeof TenantAuthWrapper>;

export const Default: Story = {
  args: {
    children: <div>Tenant content</div>,
  },
};

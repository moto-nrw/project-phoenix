import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import { MFAAdminOverrideModal } from "./mfa-admin-override-modal";

const meta = {
  title: "components/auth/MFAAdminOverrideModal",
  component: MFAAdminOverrideModal,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
  args: {
    isOpen: true,
    onClose: () => {
      // no-op for story
    },
    bearerToken: "storybook-bearer-token",
    accountId: "1",
    accountLabel: "Max Mustermann",
  },
} satisfies Meta<typeof MFAAdminOverrideModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const TenantScope: Story = {
  args: {
    scope: "tenant",
  },
};

export const OperatorScope: Story = {
  args: {
    scope: "operator",
    schoolId: "1",
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};

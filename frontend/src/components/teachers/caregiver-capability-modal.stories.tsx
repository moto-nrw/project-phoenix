import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import { CaregiverCapabilityModal } from "./caregiver-capability-modal";

const meta = {
  title: "components/teachers/CaregiverCapabilityModal",
  component: CaregiverCapabilityModal,
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
    accountId: "1",
    accountLabel: "Max Mustermann",
  },
} satisfies Meta<typeof CaregiverCapabilityModal>;

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
    schoolName: "Grundschule am Berg",
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
    scope: "tenant",
  },
};

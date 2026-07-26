import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { PasswordChangeModal } from "./password-change-modal";

const meta = {
  title: "components/ui/PasswordChangeModal",
  component: PasswordChangeModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    onClose: () => undefined,
  },
} satisfies Meta<typeof PasswordChangeModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Open: Story = {};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};

import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { PasswordResetModal } from "./password-reset-modal";

const meta: Meta<typeof PasswordResetModal> = {
  title: "components/ui/PasswordResetModal",
  component: PasswordResetModal,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof PasswordResetModal>;

export const Default: Story = {
  args: {
    isOpen: true,
    onClose: () => {
      // no-op for story
    },
    onRequestReset: async () => ({ message: "ok" }),
  },
};

export const Loading: Story = {
  args: {
    isOpen: true,
    onClose: () => {
      // no-op for story
    },
    onRequestReset: () =>
      new Promise(() => {
        // never resolves, keeps the loading state visible
      }),
  },
};

export const Success: Story = {
  args: {
    isOpen: true,
    onClose: () => {
      // no-op for story
    },
    onRequestReset: async () => ({ message: "ok" }),
  },
  play: async ({ canvasElement }) => {
    const emailInput =
      canvasElement.querySelector<HTMLInputElement>("#reset-email");
    if (emailInput) {
      emailInput.value = "test@example.com";
    }
  },
};

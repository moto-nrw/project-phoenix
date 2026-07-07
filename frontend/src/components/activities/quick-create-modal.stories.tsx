import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { QuickCreateActivityModal } from "./quick-create-modal";
import { ToastProvider } from "~/contexts/ToastContext";

const meta = {
  title: "components/activities/QuickCreateActivityModal",
  component: QuickCreateActivityModal,
  parameters: {
    layout: "fullscreen",
  },
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
} satisfies Meta<typeof QuickCreateActivityModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
    onClose: () => {},
  },
};

import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ModalWrapper } from "./modal-utils";

const meta = {
  title: "components/ui/ModalWrapper",
  component: ModalWrapper,
} satisfies Meta<typeof ModalWrapper>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    onClose: () => undefined,
    isAnimating: true,
    isExiting: false,
    children: (
      <div className="p-6">
        <h2 className="text-lg font-semibold text-gray-900">Modal Titel</h2>
        <p className="mt-2 text-sm text-gray-600">
          Beispielinhalt innerhalb des Modal-Wrappers.
        </p>
      </div>
    ),
  },
};

export const Exiting: Story = {
  args: {
    onClose: () => undefined,
    isAnimating: true,
    isExiting: true,
    children: (
      <div className="p-6">
        <p className="text-sm text-gray-600">Wird geschlossen...</p>
      </div>
    ),
  },
};

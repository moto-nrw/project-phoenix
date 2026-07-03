import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { DeviceCreateModal } from "./device-create-modal";

const meta = {
  title: "components/devices/DeviceCreateModal",
  component: DeviceCreateModal,
  args: {
    isOpen: true,
    onClose: () => undefined,
    onCreate: async () => undefined,
  },
} satisfies Meta<typeof DeviceCreateModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

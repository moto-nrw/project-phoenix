import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { Device } from "@/lib/iot-helpers";
import { DeviceEditModal } from "./device-edit-modal";

const device: Device = {
  id: "1",
  device_id: "DEV-001",
  device_type: "tablet",
  name: "Eingangs-Tablet",
  status: "active",
  last_seen: "2026-06-30T10:00:00Z",
  room_id: "1",
  room_name: "Eingang",
  is_online: true,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-06-30T10:00:00Z",
};

const meta = {
  title: "components/devices/DeviceEditModal",
  component: DeviceEditModal,
  parameters: {
    layout: "fullscreen",
  },
} satisfies Meta<typeof DeviceEditModal>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
    device,
    onSave: async () => {},
    loading: false,
  },
};

export const Loading: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
    device,
    onSave: async () => {},
    loading: true,
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
    onClose: () => {},
    device,
    onSave: async () => {},
    loading: false,
  },
};

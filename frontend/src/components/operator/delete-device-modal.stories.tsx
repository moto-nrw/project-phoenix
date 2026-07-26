import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { OperatorDevice } from "~/lib/operator/provisioning-helpers";
import { DeleteDeviceModal } from "./delete-device-modal";

const meta = {
  title: "components/operator/DeleteDeviceModal",
  component: DeleteDeviceModal,
  parameters: {
    layout: "fullscreen",
  },
} satisfies Meta<typeof DeleteDeviceModal>;

export default meta;

type Story = StoryObj<typeof meta>;

const device: OperatorDevice = {
  id: "1",
  deviceId: "DEV-001",
  deviceType: "tablet",
  name: "Eingang Haupthaus",
  status: "active",
  apiKey: "",
  maskedApiKey: "****1234",
  lastSeen: null,
  isOnline: true,
  schoolId: "1",
  schoolName: "Grundschule am Berg",
  organizationId: "1",
  organizationName: "Träger e.V.",
  createdAt: "2026-01-01T00:00:00Z",
  updatedAt: "2026-01-01T00:00:00Z",
};

export const Open: Story = {
  args: {
    device,
    onClose: () => {},
    onDeleted: () => {},
  },
};

export const WithoutName: Story = {
  args: {
    device: { ...device, name: "" },
    onClose: () => {},
    onDeleted: () => {},
  },
};

export const Closed: Story = {
  args: {
    device: null,
    onClose: () => {},
    onDeleted: () => {},
  },
};

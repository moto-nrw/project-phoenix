import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { GroupDefinition } from "~/components/database/grouped-list";
import type { Device } from "@/lib/iot-helpers";
import { DevicesMasterDetail } from "./devices-master-detail";

const devices: Device[] = [
  {
    id: "1",
    device_id: "IOT-001",
    device_type: "checkin_terminal",
    name: "Empfangs-Tablet",
    status: "active",
    room_id: "1",
    room_name: "Empfang",
    is_online: true,
    created_at: "2026-01-10T08:00:00Z",
    updated_at: "2026-01-10T08:00:00Z",
    api_key: "example-device-api-key-000000",
  },
  {
    id: "2",
    device_id: "IOT-002",
    device_type: "checkin_terminal",
    name: "Turnhallen-Tablet",
    status: "inactive",
    last_seen: "2026-01-09T16:30:00Z",
    room_id: "2",
    room_name: "Turnhalle",
    is_online: false,
    created_at: "2026-01-05T08:00:00Z",
    updated_at: "2026-01-09T16:30:00Z",
  },
];

const groupDefinitions: GroupDefinition<Device>[] = [
  {
    id: "all",
    title: "Alle Geräte",
    items: devices,
  },
];

const meta: Meta<typeof DevicesMasterDetail> = {
  title: "components/devices/DevicesMasterDetail",
  component: DevicesMasterDetail,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;

type Story = StoryObj<typeof DevicesMasterDetail>;

export const NoSelection: Story = {
  args: {
    groupDefinitions,
    selectedId: null,
    selectedDevice: null,
    onSelect: () => {},
    onSaveDevice: async () => undefined,
    onDeleteClick: () => {},
  },
};

export const OnlineDeviceSelected: Story = {
  args: {
    groupDefinitions,
    selectedId: "1",
    selectedDevice: devices[0] ?? null,
    onSelect: () => {},
    onSaveDevice: async () => undefined,
    onDeleteClick: () => {},
  },
};

export const OfflineDeviceSelected: Story = {
  args: {
    groupDefinitions,
    selectedId: "2",
    selectedDevice: devices[1] ?? null,
    onSelect: () => {},
    onSaveDevice: async () => undefined,
    onDeleteClick: () => {},
  },
};

export const EmptyList: Story = {
  args: {
    groupDefinitions: [],
    selectedId: null,
    selectedDevice: null,
    onSelect: () => {},
    onSaveDevice: async () => undefined,
    onDeleteClick: () => {},
  },
};

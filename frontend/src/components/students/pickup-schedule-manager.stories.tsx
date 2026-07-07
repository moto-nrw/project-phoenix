import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { installStorybookFetch, jsonResponse } from "~storybook/mocks/fetch";
import PickupScheduleManager from "~/components/students/pickup-schedule-manager";
import type { BackendPickupData } from "~/lib/pickup-schedule-helpers";

interface MockResponseConfig {
  status: number;
  body: unknown;
}

function mockPickupSchedule(response: MockResponseConfig) {
  return installStorybookFetch(({ url }) => {
    if (!url.includes("/pickup-schedules")) return undefined;
    return jsonResponse(response.body, { status: response.status });
  });
}

const filledSchedules: BackendPickupData = {
  schedules: [
    {
      id: 1,
      student_id: 1,
      weekday: 1,
      weekday_name: "Montag",
      pickup_time: "15:00",
      notes: "Oma holt ab",
      created_by: 1,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    {
      id: 2,
      student_id: 1,
      weekday: 2,
      weekday_name: "Dienstag",
      pickup_time: "16:00",
      created_by: 1,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    {
      id: 3,
      student_id: 1,
      weekday: 3,
      weekday_name: "Mittwoch",
      pickup_time: "13:30",
      created_by: 1,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    {
      id: 4,
      student_id: 1,
      weekday: 4,
      weekday_name: "Donnerstag",
      pickup_time: "16:00",
      created_by: 1,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    {
      id: 5,
      student_id: 1,
      weekday: 5,
      weekday_name: "Freitag",
      pickup_time: "14:00",
      created_by: 1,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
  ],
  exceptions: [],
  notes: [],
};

const meta = {
  title: "students/PickupScheduleManager",
  component: PickupScheduleManager,
  parameters: {
    docs: {
      description: {
        component:
          "Loads via `/api/students/{studentId}/pickup-schedules` on mount — each story stubs `fetch` to simulate the backend response.",
      },
    },
  },
} satisfies Meta<typeof PickupScheduleManager>;

export default meta;
type Story = StoryObj<typeof meta>;

export const WithSchedule: Story = {
  args: { studentId: "1" },
  beforeEach: () =>
    mockPickupSchedule({
      status: 200,
      body: { status: "ok", data: filledSchedules },
    }),
};

export const Empty: Story = {
  args: { studentId: "1" },
  beforeEach: () =>
    mockPickupSchedule({
      status: 200,
      body: {
        status: "ok",
        data: { schedules: [], exceptions: [], notes: [] },
      },
    }),
};

export const ReadOnly: Story = {
  args: { studentId: "1", readOnly: true },
  beforeEach: () =>
    mockPickupSchedule({
      status: 200,
      body: { status: "ok", data: filledSchedules },
    }),
};

export const LoadError: Story = {
  args: { studentId: "1" },
  beforeEach: () =>
    mockPickupSchedule({
      status: 500,
      body: { status: "error", error: "Interner Serverfehler" },
    }),
};

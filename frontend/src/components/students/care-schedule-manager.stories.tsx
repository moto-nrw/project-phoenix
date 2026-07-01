import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { installStorybookFetch, jsonResponse } from "~storybook/mocks/fetch";
import { CareScheduleManager } from "~/components/students/care-schedule-manager";
import type { ArrivalData } from "~/lib/student-arrival-api";
import type { BackendPickupData } from "~/lib/pickup-schedule-helpers";

interface MockResponses {
  arrival: { status: number; body: unknown };
  pickup: { status: number; body: unknown };
}

function mockCareSchedule(responses: MockResponses) {
  return installStorybookFetch(({ url }) => {
    if (url.includes("/arrival-schedules")) {
      return jsonResponse(responses.arrival.body, {
        status: responses.arrival.status,
      });
    }

    if (url.includes("/pickup-schedules")) {
      return jsonResponse(responses.pickup.body, {
        status: responses.pickup.status,
      });
    }

    return undefined;
  });
}

const filledArrival: ArrivalData = {
  schedules: [
    {
      id: 1,
      student_id: 1,
      weekday: 1,
      weekday_name: "Montag",
      expected_arrival: "08:00",
      created_by: 1,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
    {
      id: 2,
      student_id: 1,
      weekday: 2,
      weekday_name: "Dienstag",
      expected_arrival: "08:15",
      created_by: 1,
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    },
  ],
  exceptions: [],
  notes: [],
};

const filledPickup: BackendPickupData = {
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
  ],
  exceptions: [],
  notes: [],
};

const emptyPickup: BackendPickupData = {
  schedules: [],
  exceptions: [],
  notes: [],
};

const meta = {
  title: "students/CareScheduleManager",
  component: CareScheduleManager,
  parameters: {
    docs: {
      description: {
        component:
          "Loads via `/api/students/{studentId}/arrival-schedules` and `/api/students/{studentId}/pickup-schedules` on mount — each story stubs `fetch` to simulate the backend responses.",
      },
    },
  },
} satisfies Meta<typeof CareScheduleManager>;

export default meta;
type Story = StoryObj<typeof meta>;

export const WithSchedule: Story = {
  args: { studentId: "1" },
  beforeEach: () =>
    mockCareSchedule({
      arrival: { status: 200, body: { data: filledArrival } },
      pickup: { status: 200, body: { status: "ok", data: filledPickup } },
    }),
};

export const Empty: Story = {
  args: { studentId: "1" },
  beforeEach: () =>
    mockCareSchedule({
      arrival: {
        status: 200,
        body: { data: { schedules: [], exceptions: [], notes: [] } },
      },
      pickup: { status: 200, body: { status: "ok", data: emptyPickup } },
    }),
};

export const ReadOnly: Story = {
  args: { studentId: "1", readOnly: true },
  beforeEach: () =>
    mockCareSchedule({
      arrival: { status: 200, body: { data: filledArrival } },
      pickup: { status: 200, body: { status: "ok", data: filledPickup } },
    }),
};

export const LoadError: Story = {
  args: { studentId: "1" },
  beforeEach: () =>
    mockCareSchedule({
      arrival: {
        status: 500,
        body: { error: "Interner Serverfehler" },
      },
      pickup: {
        status: 500,
        body: { status: "error", error: "Interner Serverfehler" },
      },
    }),
};

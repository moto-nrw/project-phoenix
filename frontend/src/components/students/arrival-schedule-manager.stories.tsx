import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { installStorybookFetch, jsonResponse } from "~storybook/mocks/fetch";
import { ArrivalScheduleManager } from "~/components/students/arrival-schedule-manager";

interface MockResponseConfig {
  status: number;
  body: unknown;
}

function mockArrivalSchedule(response: MockResponseConfig) {
  return installStorybookFetch(({ url }) => {
    if (!url.includes("/arrival-schedules")) return undefined;
    return jsonResponse(response.body, { status: response.status });
  });
}

const meta = {
  title: "components/students/ArrivalScheduleManager",
  component: ArrivalScheduleManager,
  parameters: {
    docs: {
      description: {
        component:
          "Loads via `/api/students/{studentId}/arrival-schedules` on mount — each story stubs `fetch` to simulate the backend response.",
      },
    },
  },
  args: {
    studentId: "1",
  },
} satisfies Meta<typeof ArrivalScheduleManager>;

export default meta;
type Story = StoryObj<typeof meta>;

export const WithSchedule: Story = {
  beforeEach: () =>
    mockArrivalSchedule({
      status: 200,
      body: {
        data: {
          schedules: [
            {
              id: 1,
              student_id: 1,
              weekday: 1,
              weekday_name: "Montag",
              expected_arrival: "08:00",
              notes: "Bringt der Vater",
              created_by: 1,
              created_at: "2026-01-05T08:00:00Z",
              updated_at: "2026-01-05T08:00:00Z",
            },
            {
              id: 2,
              student_id: 1,
              weekday: 3,
              weekday_name: "Mittwoch",
              expected_arrival: "12:30",
              notes: null,
              created_by: 1,
              created_at: "2026-01-05T08:00:00Z",
              updated_at: "2026-01-05T08:00:00Z",
            },
          ],
          exceptions: [],
          notes: [],
        },
      },
    }),
};

export const Empty: Story = {
  beforeEach: () =>
    mockArrivalSchedule({
      status: 200,
      body: { data: { schedules: [], exceptions: [], notes: [] } },
    }),
};

export const ReadOnly: Story = {
  args: { readOnly: true },
  beforeEach: () =>
    mockArrivalSchedule({
      status: 200,
      body: {
        data: {
          schedules: [
            {
              id: 1,
              student_id: 1,
              weekday: 2,
              weekday_name: "Dienstag",
              expected_arrival: "09:15",
              notes: null,
              created_by: 1,
              created_at: "2026-01-05T08:00:00Z",
              updated_at: "2026-01-05T08:00:00Z",
            },
          ],
          exceptions: [],
          notes: [],
        },
      },
    }),
};

export const LoadError: Story = {
  beforeEach: () =>
    mockArrivalSchedule({
      status: 500,
      body: { error: "Interner Serverfehler" },
    }),
};

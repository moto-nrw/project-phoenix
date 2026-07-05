import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { installStorybookFetch, jsonResponse } from "~storybook/mocks/fetch";
import { StudentHistorieTab } from "~/components/students/student-historie-tab";

interface MockResponseConfig {
  status: number;
  body: unknown;
}

function mockAttendanceHistory(response: MockResponseConfig) {
  return installStorybookFetch(({ url }) => {
    if (!url.includes("/attendance-history")) return undefined;
    return jsonResponse(response.body, { status: response.status });
  });
}

const meta = {
  title: "components/students/StudentHistorieTab",
  component: StudentHistorieTab,
  parameters: {
    docs: {
      description: {
        component:
          "Loads via `/api/students/{studentId}/attendance-history` on mount — each story stubs `fetch` to simulate the backend response.",
      },
    },
  },
} satisfies Meta<typeof StudentHistorieTab>;

export default meta;
type Story = StoryObj<typeof meta>;

export const WithHistory: Story = {
  args: { studentId: "1" },
  beforeEach: () =>
    mockAttendanceHistory({
      status: 200,
      body: {
        data: {
          student_id: "1",
          days: [
            {
              date: "2026-06-10",
              attendance: {
                check_in_time: "2026-06-10T13:00:00Z",
                check_out_time: "2026-06-10T16:30:00Z",
                duration_minutes: 210,
              },
              room_detail_available: true,
              visits: [
                {
                  room_id: 1,
                  room_name: "Gruppenraum A",
                  entry_time: "2026-06-10T13:00:00Z",
                  exit_time: "2026-06-10T14:15:00Z",
                  duration_minutes: 75,
                },
                {
                  room_id: 2,
                  room_name: "Schulhof",
                  entry_time: "2026-06-10T14:15:00Z",
                  exit_time: null,
                  duration_minutes: null,
                },
              ],
            },
            {
              date: "2026-06-09",
              attendance: {
                check_in_time: "2026-06-09T13:15:00Z",
                check_out_time: null,
                duration_minutes: null,
              },
              room_detail_available: false,
              visits: [],
            },
          ],
          range: { start: "2026-06-01", end: "2026-06-10" },
          clamped: false,
          caps: { attendance_days: 30, room_detail_days: 7 },
        },
      },
    }),
};

export const Empty: Story = {
  args: { studentId: "1" },
  beforeEach: () =>
    mockAttendanceHistory({
      status: 200,
      body: {
        data: {
          student_id: "1",
          days: [],
          range: { start: "2026-06-01", end: "2026-06-10" },
          clamped: false,
          caps: { attendance_days: 30, room_detail_days: 7 },
        },
      },
    }),
};

export const Disabled: Story = {
  args: { studentId: "1" },
  beforeEach: () =>
    mockAttendanceHistory({
      status: 403,
      body: { error: "feature_disabled" },
    }),
};

export const Forbidden: Story = {
  args: { studentId: "1" },
  beforeEach: () =>
    mockAttendanceHistory({
      status: 403,
      body: { error: "forbidden" },
    }),
};

export const LoadError: Story = {
  args: { studentId: "1" },
  beforeEach: () =>
    mockAttendanceHistory({
      status: 500,
      body: { error: "Interner Serverfehler" },
    }),
};

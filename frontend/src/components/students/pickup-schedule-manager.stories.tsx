import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ReactNode } from "react";
import { useEffect } from "react";
import PickupScheduleManager from "~/components/students/pickup-schedule-manager";
import type { BackendPickupData } from "~/lib/pickup-schedule-helpers";

interface MockResponseConfig {
  status: number;
  body: unknown;
}

/**
 * Installs a `globalThis.fetch` stub for the story's lifetime, answering the
 * component's `/api/students/{id}/pickup-schedules` GET call (all mutation
 * endpoints are left untouched since these stories are read-only demos).
 * Restored on unmount so it never leaks into other stories.
 */
function FetchMockProvider({
  response,
  children,
}: {
  response: MockResponseConfig;
  children: ReactNode;
}) {
  useEffect(() => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();

      if (url.includes("/pickup-schedules")) {
        return Promise.resolve(
          new Response(JSON.stringify(response.body), {
            status: response.status,
            headers: { "Content-Type": "application/json" },
          }),
        );
      }

      return originalFetch(input, init);
    }) as typeof fetch;

    return () => {
      globalThis.fetch = originalFetch;
    };
  }, [response]);

  return <>{children}</>;
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
  decorators: [
    (Story) => (
      <FetchMockProvider
        response={{
          status: 200,
          body: { status: "ok", data: filledSchedules },
        }}
      >
        <Story />
      </FetchMockProvider>
    ),
  ],
};

export const Empty: Story = {
  args: { studentId: "1" },
  decorators: [
    (Story) => (
      <FetchMockProvider
        response={{
          status: 200,
          body: {
            status: "ok",
            data: { schedules: [], exceptions: [], notes: [] },
          },
        }}
      >
        <Story />
      </FetchMockProvider>
    ),
  ],
};

export const ReadOnly: Story = {
  args: { studentId: "1", readOnly: true },
  decorators: [
    (Story) => (
      <FetchMockProvider
        response={{
          status: 200,
          body: { status: "ok", data: filledSchedules },
        }}
      >
        <Story />
      </FetchMockProvider>
    ),
  ],
};

export const LoadError: Story = {
  args: { studentId: "1" },
  decorators: [
    (Story) => (
      <FetchMockProvider
        response={{
          status: 500,
          body: { status: "error", error: "Interner Serverfehler" },
        }}
      >
        <Story />
      </FetchMockProvider>
    ),
  ],
};

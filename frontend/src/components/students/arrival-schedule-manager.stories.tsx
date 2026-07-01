import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ReactNode } from "react";
import { useEffect } from "react";
import { ArrivalScheduleManager } from "~/components/students/arrival-schedule-manager";

interface MockResponseConfig {
  status: number;
  body: unknown;
}

/**
 * Installs a `globalThis.fetch` stub for the story's lifetime, answering the
 * component's `/api/students/{id}/arrival-schedules` call (and letting
 * `next-auth`'s own requests fall through to the real fetch). Restored on
 * unmount so it never leaks into other stories.
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

      if (url.includes("/arrival-schedules")) {
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
  decorators: [
    (Story) => (
      <FetchMockProvider
        response={{
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
        }}
      >
        <Story />
      </FetchMockProvider>
    ),
  ],
};

export const Empty: Story = {
  decorators: [
    (Story) => (
      <FetchMockProvider
        response={{
          status: 200,
          body: { data: { schedules: [], exceptions: [], notes: [] } },
        }}
      >
        <Story />
      </FetchMockProvider>
    ),
  ],
};

export const ReadOnly: Story = {
  args: { readOnly: true },
  decorators: [
    (Story) => (
      <FetchMockProvider
        response={{
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
        }}
      >
        <Story />
      </FetchMockProvider>
    ),
  ],
};

export const LoadError: Story = {
  decorators: [
    (Story) => (
      <FetchMockProvider
        response={{ status: 500, body: { error: "Interner Serverfehler" } }}
      >
        <Story />
      </FetchMockProvider>
    ),
  ],
};

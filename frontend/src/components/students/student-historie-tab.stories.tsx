import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ReactNode } from "react";
import { useEffect } from "react";
import { StudentHistorieTab } from "~/components/students/student-historie-tab";

interface MockResponseConfig {
  status: number;
  body: unknown;
}

/**
 * Installs a `globalThis.fetch` stub for the story's lifetime, answering the
 * component's `/api/students/{id}/attendance-history` call (and letting
 * `next-auth`'s own `/api/auth/session` request fall through to the real
 * fetch). Restored on unmount so it never leaks into other stories.
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

      if (url.includes("/attendance-history")) {
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
  decorators: [
    (Story) => (
      <FetchMockProvider
        response={{
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
            data: {
              student_id: "1",
              days: [],
              range: { start: "2026-06-01", end: "2026-06-10" },
              clamped: false,
              caps: { attendance_days: 30, room_detail_days: 7 },
            },
          },
        }}
      >
        <Story />
      </FetchMockProvider>
    ),
  ],
};

export const Disabled: Story = {
  args: { studentId: "1" },
  decorators: [
    (Story) => (
      <FetchMockProvider
        response={{ status: 403, body: { error: "feature_disabled" } }}
      >
        <Story />
      </FetchMockProvider>
    ),
  ],
};

export const Forbidden: Story = {
  args: { studentId: "1" },
  decorators: [
    (Story) => (
      <FetchMockProvider
        response={{ status: 403, body: { error: "forbidden" } }}
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
        response={{ status: 500, body: { error: "Interner Serverfehler" } }}
      >
        <Story />
      </FetchMockProvider>
    ),
  ],
};

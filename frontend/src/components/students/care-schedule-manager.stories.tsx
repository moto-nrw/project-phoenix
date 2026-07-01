import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ReactNode } from "react";
import { useEffect } from "react";
import { CareScheduleManager } from "~/components/students/care-schedule-manager";
import type { ArrivalData } from "~/lib/student-arrival-api";
import type { BackendPickupData } from "~/lib/pickup-schedule-helpers";

interface MockResponses {
  arrival: { status: number; body: unknown };
  pickup: { status: number; body: unknown };
}

/**
 * Installs a `globalThis.fetch` stub for the story's lifetime, answering the
 * component's `/api/students/{id}/arrival-schedules` and
 * `/api/students/{id}/pickup-schedules` GET calls (mutation endpoints are
 * left untouched since these stories are read-only demos).
 */
function FetchMockProvider({
  responses,
  children,
}: {
  responses: MockResponses;
  children: ReactNode;
}) {
  useEffect(() => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();

      if (url.includes("/arrival-schedules")) {
        return Promise.resolve(
          new Response(JSON.stringify(responses.arrival.body), {
            status: responses.arrival.status,
            headers: { "Content-Type": "application/json" },
          }),
        );
      }

      if (url.includes("/pickup-schedules")) {
        return Promise.resolve(
          new Response(JSON.stringify(responses.pickup.body), {
            status: responses.pickup.status,
            headers: { "Content-Type": "application/json" },
          }),
        );
      }

      return originalFetch(input, init);
    }) as typeof fetch;

    return () => {
      globalThis.fetch = originalFetch;
    };
  }, [responses]);

  return <>{children}</>;
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
  decorators: [
    (Story) => (
      <FetchMockProvider
        responses={{
          arrival: { status: 200, body: { data: filledArrival } },
          pickup: { status: 200, body: { status: "ok", data: filledPickup } },
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
        responses={{
          arrival: {
            status: 200,
            body: { data: { schedules: [], exceptions: [], notes: [] } },
          },
          pickup: { status: 200, body: { status: "ok", data: emptyPickup } },
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
        responses={{
          arrival: { status: 200, body: { data: filledArrival } },
          pickup: { status: 200, body: { status: "ok", data: filledPickup } },
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
        responses={{
          arrival: {
            status: 500,
            body: { error: "Interner Serverfehler" },
          },
          pickup: {
            status: 500,
            body: { status: "error", error: "Interner Serverfehler" },
          },
        }}
      >
        <Story />
      </FetchMockProvider>
    ),
  ],
};

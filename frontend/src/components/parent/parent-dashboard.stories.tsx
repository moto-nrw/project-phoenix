import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ReactNode } from "react";
import { useEffect } from "react";
import { ParentDashboard } from "~/components/parent/parent-dashboard";
import type { Child, EnrollmentRequest } from "~/lib/parent-api";

const manyChildren: Child[] = [
  {
    student_id: "1",
    tenant_id: "1",
    first_name: "Mia",
    last_name: "Schmidt",
    school_class: "3b",
    status: "active",
    enrolled_from: "2024-08-01",
    school_name: "OGS Am Berg",
    school_slug: "am-berg",
  },
  {
    student_id: "2",
    tenant_id: "1",
    first_name: "Ben",
    last_name: "Schmidt",
    status: "pending",
    school_name: "OGS Am Berg",
    school_slug: "am-berg",
  },
];

const manyEnrollments: EnrollmentRequest[] = [
  {
    request_id: "10",
    tenant_id: "2",
    status_token: "tok-10",
    submitted_at: "2026-05-01T10:00:00Z",
    phase_id: "1",
    phase_name: "Anmeldephase 2026/27",
    service_start_date: "2026-08-01",
    service_end_date: "2027-07-31",
    school_name: "OGS Sonnenhof",
    school_slug: "sonnenhof",
    children: [
      {
        child_id: "20",
        first_name: "Lena",
        last_name: "Weber",
        status: "under_review",
      },
    ],
  },
];

/**
 * Installs a `globalThis.fetch` stub for the story's lifetime, answering the
 * component's own `/api/parent/me/children` and `/api/parent/me/enrollments`
 * GET calls. Any other request falls through to the original fetch. Restored
 * on unmount so it never leaks into other stories.
 */
function FetchMockProvider({
  childData,
  enrollments,
  shouldFail,
  pending,
  children,
}: {
  childData: Child[];
  enrollments: EnrollmentRequest[];
  shouldFail?: boolean;
  pending?: boolean;
  children: ReactNode;
}) {
  useEffect(() => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();

      if (pending) {
        return new Promise<Response>(() => {
          // Never resolves — keeps the component in its loading state.
        });
      }

      if (url.includes("/api/parent/me/children")) {
        if (shouldFail) {
          return Promise.resolve(
            new Response(JSON.stringify({ error: "Interner Serverfehler" }), {
              status: 500,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        return Promise.resolve(
          new Response(JSON.stringify({ data: childData }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        );
      }

      if (url.includes("/api/parent/me/enrollments")) {
        if (shouldFail) {
          return Promise.resolve(
            new Response(JSON.stringify({ error: "Interner Serverfehler" }), {
              status: 500,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }
        return Promise.resolve(
          new Response(JSON.stringify({ data: enrollments }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        );
      }

      return originalFetch(input, init);
    }) as typeof fetch;

    return () => {
      globalThis.fetch = originalFetch;
    };
  }, [childData, enrollments, shouldFail, pending]);

  return <>{children}</>;
}

const meta = {
  title: "components/parent/ParentDashboard",
  component: ParentDashboard,
  parameters: {
    docs: {
      description: {
        component:
          "Loads children + enrollments via `/api/parent/me/children` and `/api/parent/me/enrollments` on mount — each story stubs `fetch` to simulate the backend response.",
      },
    },
  },
} satisfies Meta<typeof ParentDashboard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const WithChildrenAndEnrollments: Story = {
  decorators: [
    (Story) => (
      <FetchMockProvider childData={manyChildren} enrollments={manyEnrollments}>
        <Story />
      </FetchMockProvider>
    ),
  ],
};

export const Empty: Story = {
  decorators: [
    (Story) => (
      <FetchMockProvider childData={[]} enrollments={[]}>
        <Story />
      </FetchMockProvider>
    ),
  ],
};

export const Loading: Story = {
  decorators: [
    (Story) => (
      <FetchMockProvider childData={[]} enrollments={[]} pending>
        <Story />
      </FetchMockProvider>
    ),
  ],
};

export const LoadError: Story = {
  decorators: [
    (Story) => (
      <FetchMockProvider childData={[]} enrollments={[]} shouldFail>
        <Story />
      </FetchMockProvider>
    ),
  ],
};

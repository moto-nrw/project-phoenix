import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { installStorybookFetch, jsonResponse } from "~storybook/mocks/fetch";
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

function mockParentDashboard({
  childData,
  enrollments,
  shouldFail,
  pending,
}: {
  childData: Child[];
  enrollments: EnrollmentRequest[];
  shouldFail?: boolean;
  pending?: boolean;
}) {
  return installStorybookFetch(({ url }) => {
    if (
      pending &&
      (url.includes("/api/parent/me/children") ||
        url.includes("/api/parent/me/enrollments"))
    ) {
      return new Promise<Response>(() => {
        // Keep the component in its loading state.
      });
    }

    if (url.includes("/api/parent/me/children")) {
      if (shouldFail) {
        return jsonResponse(
          { error: "Interner Serverfehler" },
          { status: 500 },
        );
      }
      return jsonResponse({ data: childData });
    }

    if (url.includes("/api/parent/me/enrollments")) {
      if (shouldFail) {
        return jsonResponse(
          { error: "Interner Serverfehler" },
          { status: 500 },
        );
      }
      return jsonResponse({ data: enrollments });
    }

    return undefined;
  });
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
  beforeEach: () =>
    mockParentDashboard({
      childData: manyChildren,
      enrollments: manyEnrollments,
    }),
};

export const Empty: Story = {
  beforeEach: () => mockParentDashboard({ childData: [], enrollments: [] }),
};

export const Loading: Story = {
  beforeEach: () =>
    mockParentDashboard({ childData: [], enrollments: [], pending: true }),
};

export const LoadError: Story = {
  beforeEach: () =>
    mockParentDashboard({ childData: [], enrollments: [], shouldFail: true }),
};

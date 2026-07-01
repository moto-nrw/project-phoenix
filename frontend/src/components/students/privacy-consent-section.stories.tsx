import type { Meta, StoryObj, Decorator } from "@storybook/nextjs-vite";
import { PrivacyConsentSection } from "./privacy-consent-section";

interface MockBackendPrivacyConsent {
  id: number;
  student_id: number;
  policy_version: string;
  accepted: boolean;
  accepted_at?: string;
  expires_at?: string;
  duration_days?: number;
  renewal_required: boolean;
  data_retention_days: number;
  created_at: string;
  updated_at: string;
}

// The component fetches its own data (via fetchStudentPrivacyConsent ->
// sessionFetch -> global fetch). Stub `fetch` locally per story so it has
// data to render, without touching the global preview decorators.
function withMockedPrivacyConsent(
  consent: MockBackendPrivacyConsent | null,
): Decorator {
  return (Story) => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();
      if (url.includes("/privacy-consent")) {
        if (consent === null) {
          return Promise.resolve(new Response(null, { status: 404 }));
        }
        return Promise.resolve(
          new Response(JSON.stringify({ data: consent }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        );
      }
      return originalFetch(input, init);
    }) as typeof fetch;

    return <Story />;
  };
}

const meta = {
  title: "students/PrivacyConsentSection",
  component: PrivacyConsentSection,
} satisfies Meta<typeof PrivacyConsentSection>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Accepted: Story = {
  args: {
    studentId: "1",
  },
  decorators: [
    withMockedPrivacyConsent({
      id: 1,
      student_id: 1,
      policy_version: "1.0",
      accepted: true,
      accepted_at: "2026-01-15T08:00:00Z",
      expires_at: "2027-01-15T08:00:00Z",
      duration_days: 365,
      renewal_required: false,
      data_retention_days: 30,
      created_at: "2026-01-15T08:00:00Z",
      updated_at: "2026-01-15T08:00:00Z",
    }),
  ],
};

export const NotAccepted: Story = {
  args: {
    studentId: "2",
  },
  decorators: [
    withMockedPrivacyConsent({
      id: 2,
      student_id: 2,
      policy_version: "1.0",
      accepted: false,
      renewal_required: false,
      data_retention_days: 30,
      created_at: "2026-01-15T08:00:00Z",
      updated_at: "2026-01-15T08:00:00Z",
    }),
  ],
};

export const RenewalRequired: Story = {
  args: {
    studentId: "3",
  },
  decorators: [
    withMockedPrivacyConsent({
      id: 3,
      student_id: 3,
      policy_version: "1.0",
      accepted: true,
      accepted_at: "2025-01-15T08:00:00Z",
      expires_at: "2026-01-15T08:00:00Z",
      duration_days: 365,
      renewal_required: true,
      data_retention_days: 30,
      created_at: "2025-01-15T08:00:00Z",
      updated_at: "2025-01-15T08:00:00Z",
    }),
  ],
};

export const NoConsent: Story = {
  args: {
    studentId: "4",
  },
  decorators: [withMockedPrivacyConsent(null)],
};

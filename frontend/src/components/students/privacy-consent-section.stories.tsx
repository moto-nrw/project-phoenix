import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { installStorybookFetch, jsonResponse } from "~storybook/mocks/fetch";
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

function withMockedPrivacyConsent(consent: MockBackendPrivacyConsent | null) {
  return installStorybookFetch(({ url }) => {
    if (!url.includes("/privacy-consent")) return undefined;
    if (consent === null) {
      return new Response(null, { status: 404 });
    }
    return jsonResponse({ data: consent });
  });
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
  beforeEach: () =>
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
};

export const NotAccepted: Story = {
  args: {
    studentId: "2",
  },
  beforeEach: () =>
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
};

export const RenewalRequired: Story = {
  args: {
    studentId: "3",
  },
  beforeEach: () =>
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
};

export const NoConsent: Story = {
  args: {
    studentId: "4",
  },
  beforeEach: () => withMockedPrivacyConsent(null),
};

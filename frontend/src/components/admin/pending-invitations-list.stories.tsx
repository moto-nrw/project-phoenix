import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { installStorybookFetch, jsonResponse } from "~storybook/mocks/fetch";
import { PendingInvitationsList } from "~/components/admin/pending-invitations-list";
import type { BackendInvitation } from "~/lib/invitation-helpers";

const manyInvitations: BackendInvitation[] = [
  {
    id: 1,
    email: "neue.lehrkraft@example.com",
    role_id: 2,
    role_name: "teacher",
    expires_at: "2099-01-01T12:00:00Z",
    created_by: 1,
    creator: "admin@example.com",
  },
  {
    id: 2,
    email: "abgelaufen@example.com",
    role_id: 3,
    role_name: "supervisor",
    expires_at: "2020-01-01T12:00:00Z",
    created_by: 1,
    first_name: "Erika",
    last_name: "Musterfrau",
  },
];

function mockInvitations(invitations: BackendInvitation[], shouldFail = false) {
  return installStorybookFetch(({ url, method }) => {
    if (!url.includes("/api/invitations")) return undefined;

    if (shouldFail) {
      return jsonResponse({ error: "Interner Serverfehler" }, { status: 500 });
    }

    if (method === "GET") {
      return jsonResponse({ data: invitations });
    }

    return new Response(null, { status: 204 });
  });
}

const meta = {
  title: "components/admin/PendingInvitationsList",
  component: PendingInvitationsList,
  parameters: {
    docs: {
      description: {
        component:
          "Loads via `/api/invitations` on mount — each story stubs `fetch` to simulate the backend response.",
      },
    },
  },
} satisfies Meta<typeof PendingInvitationsList>;

export default meta;
type Story = StoryObj<typeof meta>;

export const WithInvitations: Story = {
  args: { refreshKey: 0 },
  beforeEach: () => mockInvitations(manyInvitations),
};

export const Empty: Story = {
  args: { refreshKey: 0 },
  beforeEach: () => mockInvitations([]),
};

export const LoadError: Story = {
  args: { refreshKey: 0 },
  beforeEach: () => mockInvitations([], true),
};

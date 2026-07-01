import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ReactNode } from "react";
import { useEffect } from "react";
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

/**
 * Installs a `globalThis.fetch` stub for the story's lifetime, answering the
 * component's own `/api/invitations` GET/POST/DELETE calls. Any other request
 * falls through to the original fetch. Restored on unmount so it never leaks
 * into other stories.
 */
function FetchMockProvider({
  invitations,
  shouldFail,
  children,
}: {
  invitations: BackendInvitation[];
  shouldFail?: boolean;
  children: ReactNode;
}) {
  useEffect(() => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();
      const method = init?.method ?? "GET";

      if (url.includes("/api/invitations")) {
        if (shouldFail) {
          return Promise.resolve(
            new Response(JSON.stringify({ error: "Interner Serverfehler" }), {
              status: 500,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }

        if (method === "GET") {
          return Promise.resolve(
            new Response(JSON.stringify({ data: invitations }), {
              status: 200,
              headers: { "Content-Type": "application/json" },
            }),
          );
        }

        return Promise.resolve(new Response(null, { status: 204 }));
      }

      return originalFetch(input, init);
    }) as typeof fetch;

    return () => {
      globalThis.fetch = originalFetch;
    };
  }, [invitations, shouldFail]);

  return <>{children}</>;
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
  decorators: [
    (Story) => (
      <FetchMockProvider invitations={manyInvitations}>
        <Story />
      </FetchMockProvider>
    ),
  ],
};

export const Empty: Story = {
  args: { refreshKey: 0 },
  decorators: [
    (Story) => (
      <FetchMockProvider invitations={[]}>
        <Story />
      </FetchMockProvider>
    ),
  ],
};

export const LoadError: Story = {
  args: { refreshKey: 0 },
  decorators: [
    (Story) => (
      <FetchMockProvider invitations={[]} shouldFail>
        <Story />
      </FetchMockProvider>
    ),
  ],
};

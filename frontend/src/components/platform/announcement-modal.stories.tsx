import type { Meta, StoryObj, Decorator } from "@storybook/nextjs-vite";
import { AnnouncementModal } from "./announcement-modal";

interface MockAnnouncement {
  id: number;
  title: string;
  content: string;
  type: string;
  severity: string;
  version?: string;
  published_at: string;
}

// The component fetches unread announcements itself (via useAnnouncements ->
// SWR -> authFetch -> global fetch). Stub `fetch` locally per story so the
// modal has data to render, without touching the global preview decorators.
function withMockedUnreadAnnouncements(
  announcements: MockAnnouncement[],
): Decorator {
  return (Story) => {
    const originalFetch = globalThis.fetch;
    globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === "string" ? input : input.toString();
      if (url.includes("/api/platform/announcements/unread")) {
        return Promise.resolve(
          new Response(JSON.stringify({ data: announcements }), {
            status: 200,
            headers: { "Content-Type": "application/json" },
          }),
        );
      }
      if (url.includes("/dismiss")) {
        return Promise.resolve(new Response(null, { status: 204 }));
      }
      return originalFetch(input, init);
    }) as typeof fetch;

    return <Story />;
  };
}

const meta = {
  title: "platform/AnnouncementModal",
  component: AnnouncementModal,
} satisfies Meta<typeof AnnouncementModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Release: Story = {
  decorators: [
    withMockedUnreadAnnouncements([
      {
        id: 1,
        title: "Neue Anwesenheitsübersicht",
        content:
          "Ab sofort seht ihr die Anwesenheit eurer Gruppe in Echtzeit auf einen Blick.",
        type: "release",
        severity: "info",
        version: "1.16.0",
        published_at: "2026-06-01T08:00:00Z",
      },
    ]),
  ],
};

export const Maintenance: Story = {
  decorators: [
    withMockedUnreadAnnouncements([
      {
        id: 2,
        title: "Geplante Wartung",
        content:
          "Am Wochenende führen wir Wartungsarbeiten durch. Das System kann kurzzeitig nicht erreichbar sein.",
        type: "maintenance",
        severity: "warning",
        published_at: "2026-06-05T08:00:00Z",
      },
    ]),
  ],
};

export const MultipleQueued: Story = {
  decorators: [
    withMockedUnreadAnnouncements([
      {
        id: 3,
        title: "Erste Mitteilung",
        content: "Dies ist die erste von zwei Mitteilungen.",
        type: "announcement",
        severity: "info",
        published_at: "2026-06-01T08:00:00Z",
      },
      {
        id: 4,
        title: "Zweite Mitteilung",
        content: "Dies ist die zweite von zwei Mitteilungen.",
        type: "announcement",
        severity: "info",
        published_at: "2026-06-02T08:00:00Z",
      },
    ]),
  ],
};

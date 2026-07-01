import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { installStorybookFetch, jsonResponse } from "~storybook/mocks/fetch";
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

function withMockedUnreadAnnouncements(announcements: MockAnnouncement[]) {
  return installStorybookFetch(({ url }) => {
    if (url.includes("/api/platform/announcements/unread")) {
      return jsonResponse({ data: announcements });
    }
    if (url.includes("/dismiss")) {
      return new Response(null, { status: 204 });
    }
    return undefined;
  });
}

const meta = {
  title: "platform/AnnouncementModal",
  component: AnnouncementModal,
} satisfies Meta<typeof AnnouncementModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Release: Story = {
  beforeEach: () =>
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
};

export const Maintenance: Story = {
  beforeEach: () =>
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
};

export const MultipleQueued: Story = {
  beforeEach: () =>
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
};

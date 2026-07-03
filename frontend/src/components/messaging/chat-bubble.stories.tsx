import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ChatBubble, ChatEventCard } from "./chat-bubble";

const meta: Meta<typeof ChatBubble> = {
  title: "components/messaging/ChatBubble",
  component: ChatBubble,
};

export default meta;

type Story = StoryObj<typeof ChatBubble>;

export const OwnMessage: Story = {
  args: {
    body: "Klar, ich hole ihn um 15 Uhr ab.",
    own: true,
    senderName: "Frau Müller",
    createdAt: "2026-06-10T14:32:00Z",
  },
};

export const OtherMessage: Story = {
  args: {
    body: "Kann mein Sohn heute etwas später abgeholt werden?",
    own: false,
    senderName: "Herr Schmidt",
    createdAt: "2026-06-10T14:30:00Z",
  },
};

export const WithReadReceipt: Story = {
  args: {
    body: "Alles klar, kein Problem.",
    own: true,
    senderName: "Frau Müller",
    createdAt: "2026-06-10T14:33:00Z",
    readReceiptLabel: "Gelesen",
  },
};

export const LongWrappedMessage: Story = {
  args: {
    body: "Das ist eine sehr lange Nachricht, die über mehrere Zeilen umgebrochen werden sollte, um zu prüfen, ob der Zeilenumbruch und das Wort-Wrapping korrekt funktionieren.",
    own: false,
    senderName: "Herr Schmidt",
    createdAt: "2026-06-10T09:15:00Z",
  },
};

export const EventCard: StoryObj<typeof ChatEventCard> = {
  render: (args) => <ChatEventCard {...args} />,
  args: {
    body: "Der Betreuungszeitraum wurde geändert.",
    createdAt: "2026-06-10T08:00:00Z",
  },
};

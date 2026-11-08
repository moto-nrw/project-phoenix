import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { LOCATION_COLORS } from "~/lib/location-helper";
import { FramedList } from "./framed-list";
import { ListRowLink } from "./list-row-link";
import { NotificationBadge } from "./notification-badge";

const meta = {
  title: "components/ui/ListRowLink",
  component: ListRowLink,
  parameters: {
    layout: "padded",
  },
  args: {
    href: "/rooms/1",
    title: "OGS-Raum 1",
    subtitle: "Erdgeschoss · 24 Plätze",
  },
} satisfies Meta<typeof ListRowLink>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Einzeln als Karte. */
export const Framed: Story = {
  args: { framed: true },
};

/** Offene Auswahl einer Liste, deren Objekt daneben steht. */
export const Selected: Story = {
  render: () => (
    <FramedList>
      <li>
        <ListRowLink
          href="/database/groups/1"
          title="Bärengruppe"
          subtitle="OGS-Raum 2"
          selected
        />
      </li>
      <li>
        <ListRowLink
          href="/database/groups/2"
          title="Blumengruppe"
          subtitle="OGS-Raum 2"
        />
      </li>
    </FramedList>
  ),
};

/** Farbige Kante, etwa die Planungsspur eines Blocks; getönt als eigener Block. */
export const Accent: Story = {
  render: () => (
    <FramedList>
      <li>
        <ListRowLink
          onClick={() => {}}
          title="Frühbetreuung"
          subtitle="OGS-Raum 1 · 13 Kinder"
          accent={LOCATION_COLORS.OTHER_ROOM}
        />
      </li>
      <li>
        <ListRowLink
          onClick={() => {}}
          title="Mittagessen"
          subtitle="Mensa · 52 Kinder"
          accent={LOCATION_COLORS.GROUP_ROOM}
          accentTint
        />
      </li>
    </FramedList>
  ),
};

/** Dreizeilig wie eine Unterhaltung; Ungelesenes zeigt die Zahl. */
export const Unread: Story = {
  render: () => (
    <FramedList>
      <li>
        <ListRowLink
          href="/parents/messages/1"
          overline="Für Baran Amini"
          meta="08.11.2026, 09:35"
          title="OGS-Team der OGS Sonnenhof"
          subtitle="Ja, sie stand in der Mensa."
          badge={
            <NotificationBadge
              count={2}
              tone="parents"
              ariaLabel="2 ungelesene Nachrichten"
            />
          }
          unread
        />
      </li>
      <li>
        <ListRowLink
          href="/parents/messages/group/1"
          overline="Gruppe Sternengruppe"
          meta="07.11.2026, 14:10"
          title="Laternen basteln"
          subtitle="Bitte geben Sie Ihrem Kind alte Kleidung mit."
          badge={
            <NotificationBadge
              count={1}
              tone="parents"
              ariaLabel="1 ungelesene Nachricht"
            />
          }
          unread
        />
      </li>
      <li>
        <ListRowLink
          href="/parents/messages/2"
          overline="Für Aida Amini"
          meta="06.11.2026, 08:00"
          title="OGS-Team der OGS Sonnenhof"
          subtitle="Sie: Krankmeldung: 09.11."
        />
      </li>
    </FramedList>
  ),
};

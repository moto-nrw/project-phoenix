import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Button } from "~/components/ui/button";
import { DatabaseDetailHeader } from "./database-detail-header";

const meta: Meta<typeof DatabaseDetailHeader> = {
  title: "database/DatabaseDetailHeader",
  component: DatabaseDetailHeader,
};

export default meta;
type Story = StoryObj<typeof DatabaseDetailHeader>;

export const LegacyInitials: Story = {
  args: {
    avatar: "MM",
    title: "Max Mustermann",
    subtitle: "Lehrkraft",
  },
};

export const WithAvatarObject: Story = {
  args: {
    avatar: { name: "Anna Schmidt" },
    title: "Anna Schmidt",
    subtitle: "Klasse 3b",
  },
};

export const WithWarning: Story = {
  args: {
    avatar: "AB",
    title: "Gruppe A",
    subtitle: "12 Kinder",
    warning: "Kein aktiver Betreuer",
  },
};

export const WithActions: Story = {
  args: {
    avatar: { name: "Erika Musterfrau" },
    title: "Erika Musterfrau",
    subtitle: "Erziehungsberechtigte",
    actions: (
      <Button type="button" size="sm">
        Bearbeiten
      </Button>
    ),
  },
};

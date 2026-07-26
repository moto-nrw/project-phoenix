import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Button } from "~/components/ui/button";

import { EntityHeaderCard } from "./entity-header-card";

const meta = {
  title: "operator/EntityHeaderCard",
  component: EntityHeaderCard,
} satisfies Meta<typeof EntityHeaderCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Active: Story = {
  args: {
    title: "Grundschule am Berg",
    subdomain: "gs-am-berg",
    active: true,
    createdAt: "2023-08-01",
    stats: [
      { label: "Schüler", value: 128 },
      { label: "Mitarbeitende", value: 12 },
    ],
  },
};

export const Inactive: Story = {
  args: {
    title: "Grundschule am Berg",
    subdomain: "gs-am-berg",
    active: false,
    activeLabel: "Aktiv",
    inactiveLabel: "Inaktiv",
  },
};

export const Minimal: Story = {
  args: {
    title: "Ohne Zusatzinfos",
    active: true,
  },
};

export const WithActionsAndSubtitle: Story = {
  args: {
    title: "Grundschule am Berg",
    subdomain: "gs-am-berg",
    active: true,
    subtitle: "Betreibende Organisation: Träger für Bildung e.V.",
    createdAt: new Date("2022-01-15"),
    stats: [{ label: "Schüler", value: 128 }],
    actions: (
      <>
        <Button type="button" variant="outline" size="sm">
          Bearbeiten
        </Button>
        <Button type="button" variant="danger" size="sm">
          Deaktivieren
        </Button>
      </>
    ),
  },
};

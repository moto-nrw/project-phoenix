import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import { Button } from "~/components/ui/button";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import { roomsConcept } from "~/lib/concepts/rooms";

import { DatabasePageLayout } from "./database-page-layout";

const meta = {
  title: "components/database/DatabasePageLayout",
  component: DatabasePageLayout,
} satisfies Meta<typeof DatabasePageLayout>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    loading: false,
    sessionLoading: false,
    children: (
      <div className="p-8">
        <h1 className="text-xl font-semibold">Database page content</h1>
        <p className="text-gray-600">
          Rendered inside DatabasePageLayout when neither the session nor the
          page data is loading.
        </p>
      </div>
    ),
  },
};

export const Loading: Story = {
  args: {
    loading: true,
    sessionLoading: false,
    children: <div>Content</div>,
  },
};

export const SessionLoading: Story = {
  args: {
    loading: false,
    sessionLoading: true,
    children: <div>Content</div>,
  },
};

/**
 * Datenbankseite mit Kopfkarte und Such- und Filterzeile, wie sie die
 * Stammdaten-Seiten (Kinder, Räume, Gruppen …) und die Katalogseiten zeigen.
 */
export const WithSearch: Story = {
  args: {
    loading: false,
    sessionLoading: false,
    intro: {
      title: "Räume",
      description: "24 Räume · 3 Kategorien",
      actions: (
        <Button type="button" size="md">
          Raum anlegen
        </Button>
      ),
    },
    search: (
      <PageHeaderWithSearch
        embedded
        title=""
        badge={{
          icon: (
            <MotoDuotoneIcon
              icon={roomsConcept.icon}
              tone={roomsConcept.tone}
              size={20}
            />
          ),
          count: 12,
          label: "Räume",
        }}
        search={{ value: "", onChange: fn(), placeholder: "Räume suchen…" }}
        filters={[
          {
            id: "category",
            label: "Kategorie",
            type: "dropdown",
            value: "gruppenraum",
            onChange: fn(),
            options: [
              { value: "all", label: "Alle Kategorien" },
              { value: "gruppenraum", label: "Gruppenraum" },
              { value: "mensa", label: "Mensa" },
            ],
          },
        ]}
        activeFilters={[
          { id: "category", label: "Gruppenraum", onRemove: fn() },
        ]}
        onClearAllFilters={fn()}
      />
    ),
    children: <div className="p-8 text-sm text-gray-600">Liste der Räume</div>,
  },
};

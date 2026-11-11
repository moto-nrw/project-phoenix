import type { Decorator, Meta, StoryObj } from "@storybook/nextjs-vite";
import { Plus, Users } from "lucide-react";
import { expect, fn, screen, userEvent, within } from "storybook/test";

import { withNotBookedSlots } from "~storybook/fixtures/not-booked-slots";
import { Button, ButtonLink } from "./button";
import type { ActiveFilter, FilterConfig } from "./page-header/types";
import { PagePathProvider } from "./page-path";
import { SectionCard } from "./section-card";
import { TenantPage, TenantPageListSearch } from "./tenant-page";

const meta = {
  title: "components/ui/TenantPage",
  component: TenantPage,
} satisfies Meta<typeof TenantPage>;

export default meta;

type Story = StoryObj<typeof meta>;

const withTenantPagePath: Decorator = (Story) => (
  <PagePathProvider resolve={(path) => `/storybook${path}`}>
    <Story />
  </PagePathProvider>
);

/**
 * Kopfkarte mit Link und Knopf nebeneinander: Die Kopfkarte setzt beide auf
 * ihre Bedienhöhe, 40 px, unter `sm` 44 px (#5531).
 */
export const HeadWithLinkAndButton: Story = {
  args: {
    title: "Kinder",
    stats: "116 Kinder · 107 zuhause · 9 krank",
    actions: (
      <>
        <ButtonLink href="/students/import" variant="outline" size="md">
          Importieren
        </ButtonLink>
        <Button type="button" size="md" className="gap-1.5">
          <Plus className="h-4 w-4" aria-hidden />
          Kind anlegen
        </Button>
      </>
    ),
    children: (
      <SectionCard title="Liste">
        <p className="text-sm text-gray-600">Inhalt der Seite</p>
      </SectionCard>
    ),
  },
};

/**
 * Offener Erklärungs-Popover (`AnchoredPopover`): `play` klickt das Info-Symbol
 * am Titel. Vorlage für offene Popover-Zustände im Pixel-Vergleich (#6562).
 */
export const InfoOpened: Story = {
  args: {
    title: "Kinder",
    stats: "116 Kinder · 107 zuhause · 9 krank",
    info: "Hier sehen Sie alle Kinder Ihrer Schule und wo sie gerade sind.",
    children: (
      <SectionCard title="Liste">
        <p className="text-sm text-gray-600">Inhalt der Seite</p>
      </SectionCard>
    ),
  },
  play: async ({ canvasElement }) => {
    await userEvent.click(
      within(canvasElement).getByRole("button", {
        name: "Erklärung zu Kinder",
      }),
    );
    await expect(
      await screen.findByRole("dialog", { name: "Erklärung zu Kinder" }),
    ).toBeVisible();
  },
};

const statusFilter: FilterConfig = {
  id: "status",
  label: "Status",
  type: "dropdown",
  value: "present",
  onChange: fn(),
  options: [
    { value: "all", label: "Alle" },
    { value: "present", label: "Anwesend" },
    { value: "home", label: "Zuhause" },
  ],
};

const groupFilter: FilterConfig = {
  id: "group",
  label: "Gruppe",
  type: "buttons",
  value: "all",
  onChange: fn(),
  options: [
    { value: "all", label: "Alle" },
    { value: "sonne", label: "Sonne" },
    { value: "mond", label: "Mond" },
  ],
};

const activeStatusFilter: ActiveFilter[] = [
  { id: "status", label: "Anwesend", onRemove: fn() },
];

const pageContent = (
  <SectionCard title="Liste">
    <p className="text-sm text-gray-600">Inhalt der Seite</p>
  </SectionCard>
);

/**
 * Such- und Filterzeile der Kopfkarte wie auf den Datenbankseiten: Suchfeld,
 * Filter in der Zeile und Zähler-Plakette. Ohne Chips: Ihr Entfernen-X
 * rasterte auf dem Telefon in dieser Zeile nicht stabil (5 Pixel, #6578). Die
 * Chips zeigt `DatabasePageLayout` `WithSearch`.
 */
export const SearchWithFiltersAndBadge: Story = {
  args: {
    title: "Kinder",
    stats: "116 Kinder",
    search: { value: "", onChange: fn(), placeholder: "Kinder suchen…" },
    filters: [statusFilter],
    onClearAllFilters: fn(),
    badge: { icon: <Users className="h-5 w-5" />, count: 9, label: "Kinder" },
    children: pageContent,
  },
};

/** Kebab-Menü in der Suchzeile, ohne Filter. */
export const SearchWithOverflowMenu: Story = {
  args: {
    title: "Räume",
    search: { value: "Mensa", onChange: fn(), placeholder: "Räume suchen…" },
    overflowMenu: [
      { label: "Exportieren", onClick: fn() },
      { label: "Drucken", onClick: fn() },
    ],
    children: pageContent,
  },
};

const quietFilterArgs = {
  title: "Kindersuche",
  search: { value: "", onChange: fn(), placeholder: "Kind suchen…" },
  filters: [statusFilter, groupFilter],
  activeFilters: activeStatusFilter,
  onClearAllFilters: fn(),
  badge: { icon: <Users className="h-5 w-5" />, count: 42, label: "Kinder" },
  filterPanel: {
    sections: [
      { title: "Anwesenheit", filterIds: ["status"] },
      { title: "Gruppe", filterIds: ["group"] },
    ],
  },
  primaryAction: (
    <Button type="button" size="md">
      Anmelden
    </Button>
  ),
  children: pageContent,
};

/**
 * Ruhige Filter wie auf der Kindersuche: eine Filter-Schaltfläche mit Zahl,
 * Hauptaktion in der Suchzeile, Filter erst ab `xl` als Popover.
 */
export const QuietFilters: Story = { args: quietFilterArgs };

/** Wie `QuietFilters`, mit geöffneter Filterfläche. */
export const QuietFiltersOpened: Story = {
  args: quietFilterArgs,
  play: async ({ canvasElement }) => {
    const visibleFilterButton = within(canvasElement)
      .getAllByRole("button", { name: /^Filter/ })
      .find((button) => button.offsetParent !== null);
    if (!visibleFilterButton) throw new Error("Kein sichtbarer Filterknopf");
    await userEvent.click(visibleFilterButton);
    await expect(visibleFilterButton).toHaveAttribute("aria-expanded", "true");
  },
};

/** Die Such- und Filterzeile als eigene Fläche über der Liste (#4353). */
export const ListSearch: Story = {
  args: {
    title: "Aktuelle Aufsicht",
    children: (
      <TenantPageListSearch
        label="Kinder suchen"
        search={{ value: "", onChange: fn(), placeholder: "Kind suchen…" }}
        filters={[groupFilter]}
        activeFilters={[]}
        onClearAllFilters={fn()}
      />
    ),
  },
};

/**
 * Unterseite mit Rückweg über dem Titel. Im Tenant-Bereich führt der Link
 * auf die Seite der Schule.
 */
export const WithBackLink: Story = {
  args: {
    title: "Raum 101",
    back: true,
    backHref: "/rooms",
    backLabel: "Zurück zu den Räumen",
    children: pageContent,
  },
  decorators: [withTenantPagePath],
};

/** Ohne eigenes Ziel führt der Rückweg zur Datenverwaltung. */
export const WithDefaultBackLink: Story = {
  args: {
    title: "Räume",
    back: true,
    children: pageContent,
  },
  decorators: [withTenantPagePath],
};

/**
 * Seite eines Moduls, das die Schule nicht gebucht hat (#4894): unter dem
 * Titel steht die Erklärung statt des Inhalts; Statuszeile und Aktionen
 * entfallen.
 */
export const ModuleNotBookedPage: Story = {
  args: {
    title: "Dienstplan",
    stats: "12 Dienste diese Woche",
    actions: (
      <Button type="button" size="md">
        Dienst anlegen
      </Button>
    ),
    children: pageContent,
  },
  decorators: [withNotBookedSlots("dienstplan")],
};

/**
 * Eine Anfrage der Seite wird mit `module.not_booked` abgelehnt: der Rumpf
 * zeigt die Erklärung, der Kopf bleibt.
 */
export const ModuleNotBookedLoadError: Story = {
  args: {
    title: "Kinder",
    stats: "116 Kinder · 107 zuhause · 9 krank",
    error: {
      message: "Das Modul ist nicht gebucht.",
      notBookedModule: "zeiterfassung",
    },
    children: pageContent,
  },
  decorators: [withNotBookedSlots()],
};

import type { Decorator, Meta, StoryObj } from "@storybook/nextjs-vite";
import { Plus, SlidersHorizontal, Users } from "lucide-react";
import { expect, fn, screen, userEvent, within } from "storybook/test";

import { withNotBookedSlots } from "~storybook/fixtures/not-booked-slots";
import { Button, ButtonLink } from "./button";
import { CollectionViewSwitch } from "./collection-view-switch";
import { CustomSelect } from "./custom-select";
import { ISODatePicker } from "./date-picker";
import { Input } from "./input";
import { OverflowMenu } from "./page-header/OverflowMenu";
import type { ActiveFilter, FilterConfig } from "./page-header/types";
import { PagePathProvider } from "./page-path";
import { SectionCard } from "./section-card";
import { SegmentedControl } from "./segmented-control";
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

/**
 * Hervorgehobener Titel mit Element davor und Info-Knopf: Titelgrade und
 * Kopf-Polster kommen aus der Kopfgeometrie (#6579).
 */
export const ProminentWithLeading: Story = {
  args: {
    title: "Guten Morgen, Frau Berg",
    prominent: true,
    leading: (
      <span className="flex h-12 w-12 items-center justify-center rounded-full bg-gray-100">
        <Users className="h-6 w-6 text-gray-600" aria-hidden />
      </span>
    ),
    info: "Ihre Startseite zeigt, was heute ansteht.",
    stats: "Mittwoch, 9. September",
    children: pageContent,
  },
};

/**
 * Reine Symbol-Bedienelemente neben Textknöpfen: Unter `sm` bleiben Kebab,
 * Symbolknopf und Ansichts-Umschalter neben der Statuszeile, jeder Textknopf
 * bekommt eine eigene volle Zeile (#6579).
 */
export const IconAndTextActions: Story = {
  args: {
    title: "Alle Kinder",
    stats: "116 Kinder · 107 zuhause · 9 krank",
    actions: (
      <>
        <div data-icon-only="">
          <CollectionViewSwitch value="tiles" onChange={fn()} />
        </div>
        <Button
          type="button"
          variant="outline"
          size="icon"
          aria-label="Anpassen"
        >
          <SlidersHorizontal className="h-4 w-4" aria-hidden />
        </Button>
        <Button type="button" size="md">
          Kind anlegen
        </Button>
        <OverflowMenu items={[{ label: "Exportieren", onClick: fn() }]} />
      </>
    ),
    children: pageContent,
  },
};

/**
 * Symbol-Bedienelemente mit eigenem Tour-Ziel oder eigenen Ereignissen: der
 * Kebab mit Tour-Stopp und der Anpassen-Knopf der Startseite, der beim
 * Zeigen vorlädt (#6579).
 */
export const SymbolActionsWithTourAndPreload: Story = {
  args: {
    title: "Kinder",
    stats: "116 Kinder · 107 zuhause · 9 krank",
    actions: (
      <>
        <div data-icon-only="" onPointerEnter={fn()} onFocus={fn()}>
          <Button
            type="button"
            variant="outline"
            size="icon"
            aria-label="Anpassen"
            tooltip="Anpassen"
          >
            <SlidersHorizontal className="h-4 w-4" aria-hidden />
          </Button>
        </div>
        <Button type="button" size="md">
          Kind anlegen
        </Button>
        <div data-icon-only="" data-setup-tour="student-menu">
          <OverflowMenu items={[{ label: "Importieren", onClick: fn() }]} />
        </div>
        <div
          className="contents"
          data-icon-only=""
          data-setup-tour="child-search-menu"
        >
          <OverflowMenu items={[{ label: "Abwesenheiten", onClick: fn() }]} />
        </div>
      </>
    ),
    children: pageContent,
  },
};

/**
 * Bedienband der Kopfkarte (`searchSlot`) mit Auswahl, Feld, Datumsauswahl,
 * Umschalter und Knopf: alle auf der Bedienhöhe der Kopfkarte (#6579).
 */
export const ControlsInSearchSlot: Story = {
  args: {
    title: "Planung",
    searchSlot: (
      <div className="flex flex-wrap items-center gap-2">
        <SegmentedControl
          ariaLabel="Zeitraum"
          items={[
            { value: "week", label: "Woche" },
            { value: "month", label: "Monat" },
          ]}
          value="week"
          onChange={fn()}
        />
        <CustomSelect
          ariaLabel="Gruppe"
          value="sonne"
          onChange={fn()}
          options={[
            { value: "sonne", label: "Sonne" },
            { value: "mond", label: "Mond" },
          ]}
          className="w-40"
        />
        <div className="w-48">
          <Input
            aria-label="Suche"
            placeholder="Suchen"
            controlSize="compact"
          />
        </div>
        <ISODatePicker ariaLabel="Datum" value="2026-09-09" onChange={fn()} />
        <Button type="button" variant="outline" size="md">
          Heute
        </Button>
      </div>
    ),
    children: pageContent,
  },
};

const tabItems = [
  { value: "overview", label: "Übersicht" },
  { value: "groups", label: "Gruppen", badge: 3 },
  { value: "rooms", label: "Räume" },
  { value: "staff", label: "Personal" },
  { value: "absences", label: "Abwesenheiten" },
  { value: "settings", label: "Einstellungen", disabled: true },
  { value: "history", label: "Verlauf" },
  { value: "archive", label: "Archiv" },
  { value: "documents", label: "Dokumente" },
  { value: "statistics", label: "Statistik" },
];

/**
 * Seitenreiter: Auf dem Desktop steht der Überhang unter „Mehr“, auf dem
 * Telefon scrollt das Band (#6579).
 */
export const WithTabs: Story = {
  args: {
    title: "Schule",
    tabs: { value: "groups", onChange: fn(), items: tabItems },
    children: pageContent,
  },
};

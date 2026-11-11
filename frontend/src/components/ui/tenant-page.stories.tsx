import type { Decorator, Meta, StoryObj } from "@storybook/nextjs-vite";
import { Plus } from "lucide-react";
import { expect, screen, userEvent, within } from "storybook/test";

import { withNotBookedSlots } from "~storybook/fixtures/not-booked-slots";
import { Button, ButtonLink } from "./button";
import { PagePathProvider } from "./page-path";
import { SectionCard } from "./section-card";
import { TenantPage } from "./tenant-page";

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

const pageContent = (
  <SectionCard title="Liste">
    <p className="text-sm text-gray-600">Inhalt der Seite</p>
  </SectionCard>
);

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

import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ReactNode } from "react";

import { Button } from "./button";
import {
  ResponsiveActionList,
  ResponsiveActionRow,
} from "./responsive-action-list";
import { SectionCard } from "./section-card";

const meta = {
  title: "components/ui/ResponsiveActionList",
  component: ResponsiveActionList,
  args: { children: null },
} satisfies Meta<typeof ResponsiveActionList>;

export default meta;

type Story = StoryObj<typeof meta>;

function RowText({
  title,
  detail,
}: Readonly<{ title: string; detail: string }>) {
  return (
    <div className="min-w-0">
      <p className="text-sm font-semibold text-gray-900">{title}</p>
      <p className="mt-0.5 text-sm text-gray-600">{detail}</p>
    </div>
  );
}

function RowActions({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
      {children}
    </div>
  );
}

function Action({ label }: Readonly<{ label: string }>) {
  return (
    <Button type="button" variant="surface" size="compact">
      {label}
    </Button>
  );
}

function Frame({ children }: Readonly<{ children: ReactNode }>) {
  return <SectionCard title="Offene Hinweise">{children}</SectionCard>;
}

/** Die Regelzeile: Text links, Aktionen in einer bündigen Spalte rechts. */
export const Default: Story = {
  render: () => (
    <Frame>
      <ResponsiveActionList>
        <ResponsiveActionRow>
          <RowText
            title="„Fußball-AG“"
            detail="12 Kinder · 3a, 3b · Montag und Mittwoch"
          />
          <RowActions>
            <Action label="Regeltermin anlegen" />
          </RowActions>
        </ResponsiveActionRow>
        <ResponsiveActionRow>
          <RowText
            title="„Hausaufgabenbetreuung mit einem längeren Namen“"
            detail="4 Kinder · 1a · Dienstag"
          />
          <RowActions>
            <Action label="Ansehen" />
            <Action label="Regeltermin anlegen" />
          </RowActions>
        </ResponsiveActionRow>
        <ResponsiveActionRow>
          <RowText title="„Kunst“" detail="7 Kinder · 2b · Freitag" />
          <RowActions>
            <Action label="Regeltermin anlegen" />
          </RowActions>
        </ResponsiveActionRow>
      </ResponsiveActionList>
    </Frame>
  ),
};

/** Zeile mit größerem Abstand zwischen Text und Aktionen (Ferienwoche). */
export const WiderGap: Story = {
  render: () => (
    <Frame>
      <ResponsiveActionList>
        <ResponsiveActionRow className="gap-3">
          <RowText title="Woche 1" detail="20 von 24 Plätzen vergeben" />
          <RowActions>
            <Action label="Plätze vergeben" />
          </RowActions>
        </ResponsiveActionRow>
        <ResponsiveActionRow className="gap-3">
          <RowText title="Woche 2" detail="24 von 24 Plätzen vergeben" />
          <RowActions>
            <Action label="Plätze vergeben" />
          </RowActions>
        </ResponsiveActionRow>
      </ResponsiveActionList>
    </Frame>
  ),
};

/** Oben bündig, wenn der Text mehrere Zeilen hat (Kostenträger). */
export const TopAligned: Story = {
  render: () => (
    <Frame>
      <ResponsiveActionList>
        <ResponsiveActionRow align="start">
          <div className="min-w-0 text-sm text-gray-700">
            <p className="font-semibold text-gray-900">Jugendamt Musterstadt</p>
            <p>Musterstraße 1</p>
            <p>12345 Musterstadt</p>
          </div>
          <RowActions>
            <Action label="Bearbeiten" />
          </RowActions>
        </ResponsiveActionRow>
        <ResponsiveActionRow align="start">
          <div className="min-w-0 text-sm text-gray-700">
            <p className="font-semibold text-gray-900">Sozialamt Musterstadt</p>
            <p>Rathausplatz 2</p>
          </div>
          <RowActions>
            <Action label="Bearbeiten" />
          </RowActions>
        </ResponsiveActionRow>
      </ResponsiveActionList>
    </Frame>
  ),
};

/** Menü neben dem Text auch auf dem Telefon (übernommene Korrekturen). */
export const ActionBesideText: Story = {
  render: () => (
    <Frame>
      <ResponsiveActionList>
        <ResponsiveActionRow
          align="start"
          className="flex-row items-start justify-between gap-3 first:pt-3 last:pb-3"
        >
          <div className="min-w-0 text-sm">
            <p className="font-medium text-gray-900">Mia Muster</p>
            <p className="text-gray-700 tabular-nums">−12,50 €</p>
            <p className="text-gray-700">Erstattung Mittagessen</p>
          </div>
          <Action label="Ändern" />
        </ResponsiveActionRow>
        <ResponsiveActionRow
          align="start"
          className="flex-row items-start justify-between gap-3 first:pt-3 last:pb-3"
        >
          <div className="min-w-0 text-sm">
            <p className="font-medium text-gray-900">Ben Beispiel</p>
            <p className="text-gray-700 tabular-nums">4,00 €</p>
            <p className="text-gray-700">Nachberechnung Ausflug</p>
          </div>
          <Action label="Ändern" />
        </ResponsiveActionRow>
      </ResponsiveActionList>
    </Frame>
  ),
};

/** Blockzeile mit größerem Innenabstand (Vertragsstände). */
export const BlockRow: Story = {
  render: () => (
    <Frame>
      <ResponsiveActionList>
        <ResponsiveActionRow align="start" className="block gap-[normal] py-4">
          <RowText title="Fassung 2" detail="gilt ab 01.08.2026" />
          <RowActions>
            <Action label="Ansehen" />
          </RowActions>
        </ResponsiveActionRow>
        <ResponsiveActionRow align="start" className="block gap-[normal] py-4">
          <RowText title="Fassung 1" detail="gilt bis 31.07.2026" />
          <RowActions>
            <Action label="Ansehen" />
          </RowActions>
        </ResponsiveActionRow>
      </ResponsiveActionList>
    </Frame>
  ),
};

/** Eine hervorgehobene Zeile auf getönter Fläche (Preisliste). */
export const HighlightedRow: Story = {
  render: () => (
    <Frame>
      <ResponsiveActionList>
        <ResponsiveActionRow className="bg-moto-orange/10 rounded-xl px-4 first:pt-3 last:pb-3">
          <p className="text-sm text-gray-900">
            „Frühbetreuung“ hat gebuchte Tage ohne Preis.
          </p>
          <div className="flex sm:justify-end">
            <Action label="Preise ergänzen" />
          </div>
        </ResponsiveActionRow>
      </ResponsiveActionList>
    </Frame>
  ),
};

/** Dichte Zeilen (Warteliste). */
export const DenseRows: Story = {
  render: () => (
    <Frame>
      <ResponsiveActionList>
        <ResponsiveActionRow className="py-2 first:pt-2 last:pb-2">
          <RowText title="1. Mia Muster" detail="3a · seit 02.09.2026" />
          <RowActions>
            <Action label="Platz geben" />
          </RowActions>
        </ResponsiveActionRow>
        <ResponsiveActionRow className="py-2 first:pt-2 last:pb-2">
          <RowText title="2. Ben Beispiel" detail="2b · seit 05.09.2026" />
          <RowActions>
            <Action label="Platz geben" />
          </RowActions>
        </ResponsiveActionRow>
      </ResponsiveActionList>
    </Frame>
  ),
};

/** Zeile mit eigener Textfarbe für Fließtext (Auswirkungen des Betreuungsplans). */
export const MutedText: Story = {
  render: () => (
    <Frame>
      <ResponsiveActionList>
        <ResponsiveActionRow className="text-sm text-gray-700">
          <span>Mia Muster ist am Montag nicht mehr in der Fußball-AG.</span>
          <RowActions>
            <Action label="Ansehen" />
          </RowActions>
        </ResponsiveActionRow>
        <ResponsiveActionRow className="text-sm text-gray-700">
          <span>Ben Beispiel ist am Freitag nicht mehr in „Kunst“.</span>
          <RowActions>
            <Action label="Ansehen" />
          </RowActions>
        </ResponsiveActionRow>
      </ResponsiveActionList>
    </Frame>
  ),
};

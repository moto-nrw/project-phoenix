import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { ReactNode } from "react";

import { Button } from "./button";
import { ResponsiveActionList } from "./responsive-action-list";
import { SectionCard } from "./section-card";

const meta = {
  title: "components/ui/ResponsiveActionList",
  component: ResponsiveActionList,
  args: { children: null },
} satisfies Meta<typeof ResponsiveActionList>;

export default meta;

type Story = StoryObj<typeof meta>;

const ROW =
  "flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center";

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
        <li className={ROW}>
          <RowText
            title="„Fußball-AG“"
            detail="12 Kinder · 3a, 3b · Montag und Mittwoch"
          />
          <RowActions>
            <Action label="Regeltermin anlegen" />
          </RowActions>
        </li>
        <li className={ROW}>
          <RowText
            title="„Hausaufgabenbetreuung mit einem längeren Namen“"
            detail="4 Kinder · 1a · Dienstag"
          />
          <RowActions>
            <Action label="Ansehen" />
            <Action label="Regeltermin anlegen" />
          </RowActions>
        </li>
        <li className={ROW}>
          <RowText title="„Kunst“" detail="7 Kinder · 2b · Freitag" />
          <RowActions>
            <Action label="Regeltermin anlegen" />
          </RowActions>
        </li>
      </ResponsiveActionList>
    </Frame>
  ),
};

/** Zeile mit größerem Abstand zwischen Text und Aktionen (Ferienwoche). */
export const WiderGap: Story = {
  render: () => (
    <Frame>
      <ResponsiveActionList>
        <li className="flex flex-col gap-3 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center">
          <RowText title="Woche 1" detail="20 von 24 Plätzen vergeben" />
          <RowActions>
            <Action label="Plätze vergeben" />
          </RowActions>
        </li>
        <li className="flex flex-col gap-3 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center">
          <RowText title="Woche 2" detail="24 von 24 Plätzen vergeben" />
          <RowActions>
            <Action label="Plätze vergeben" />
          </RowActions>
        </li>
      </ResponsiveActionList>
    </Frame>
  ),
};

/** Oben bündig, wenn der Text mehrere Zeilen hat (Kostenträger). */
export const TopAligned: Story = {
  render: () => (
    <Frame>
      <ResponsiveActionList>
        <li className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-start">
          <div className="min-w-0 text-sm text-gray-700">
            <p className="font-semibold text-gray-900">Jugendamt Musterstadt</p>
            <p>Musterstraße 1</p>
            <p>12345 Musterstadt</p>
          </div>
          <RowActions>
            <Action label="Bearbeiten" />
          </RowActions>
        </li>
        <li className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-start">
          <div className="min-w-0 text-sm text-gray-700">
            <p className="font-semibold text-gray-900">Sozialamt Musterstadt</p>
            <p>Rathausplatz 2</p>
          </div>
          <RowActions>
            <Action label="Bearbeiten" />
          </RowActions>
        </li>
      </ResponsiveActionList>
    </Frame>
  ),
};

/** Menü neben dem Text auch auf dem Telefon (übernommene Korrekturen). */
export const ActionBesideText: Story = {
  render: () => (
    <Frame>
      <ResponsiveActionList>
        <li className="flex items-start justify-between gap-3 py-3 sm:col-span-2 sm:grid sm:grid-cols-subgrid">
          <div className="min-w-0 text-sm">
            <p className="font-medium text-gray-900">Mia Muster</p>
            <p className="text-gray-700 tabular-nums">−12,50 €</p>
            <p className="text-gray-700">Erstattung Mittagessen</p>
          </div>
          <Action label="Ändern" />
        </li>
        <li className="flex items-start justify-between gap-3 py-3 sm:col-span-2 sm:grid sm:grid-cols-subgrid">
          <div className="min-w-0 text-sm">
            <p className="font-medium text-gray-900">Ben Beispiel</p>
            <p className="text-gray-700 tabular-nums">4,00 €</p>
            <p className="text-gray-700">Nachberechnung Ausflug</p>
          </div>
          <Action label="Ändern" />
        </li>
      </ResponsiveActionList>
    </Frame>
  ),
};

/** Blockzeile mit größerem Innenabstand (Vertragsstände). */
export const BlockRow: Story = {
  render: () => (
    <Frame>
      <ResponsiveActionList>
        <li className="py-4 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-start">
          <RowText title="Fassung 2" detail="gilt ab 01.08.2026" />
          <RowActions>
            <Action label="Ansehen" />
          </RowActions>
        </li>
        <li className="py-4 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-start">
          <RowText title="Fassung 1" detail="gilt bis 31.07.2026" />
          <RowActions>
            <Action label="Ansehen" />
          </RowActions>
        </li>
      </ResponsiveActionList>
    </Frame>
  ),
};

/** Eine hervorgehobene Zeile auf getönter Fläche (Preisliste). */
export const HighlightedRow: Story = {
  render: () => (
    <Frame>
      <ResponsiveActionList>
        <li className="bg-moto-orange/10 flex flex-col gap-2 rounded-xl px-4 py-3 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center">
          <p className="text-sm text-gray-900">
            „Frühbetreuung“ hat gebuchte Tage ohne Preis.
          </p>
          <div className="flex sm:justify-end">
            <Action label="Preise ergänzen" />
          </div>
        </li>
      </ResponsiveActionList>
    </Frame>
  ),
};

/** Dichte Zeilen (Warteliste). */
export const DenseRows: Story = {
  render: () => (
    <Frame>
      <ResponsiveActionList>
        <li className="flex flex-col gap-2 py-2 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center">
          <RowText title="1. Mia Muster" detail="3a · seit 02.09.2026" />
          <RowActions>
            <Action label="Platz geben" />
          </RowActions>
        </li>
        <li className="flex flex-col gap-2 py-2 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center">
          <RowText title="2. Ben Beispiel" detail="2b · seit 05.09.2026" />
          <RowActions>
            <Action label="Platz geben" />
          </RowActions>
        </li>
      </ResponsiveActionList>
    </Frame>
  ),
};

/** Zeile mit eigener Textfarbe für Fließtext (Auswirkungen des Betreuungsplans). */
export const MutedText: Story = {
  render: () => (
    <Frame>
      <ResponsiveActionList>
        <li className="flex flex-col gap-2 py-3 text-sm text-gray-700 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center">
          <span>Mia Muster ist am Montag nicht mehr in der Fußball-AG.</span>
          <RowActions>
            <Action label="Ansehen" />
          </RowActions>
        </li>
        <li className="flex flex-col gap-2 py-3 text-sm text-gray-700 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center">
          <span>Ben Beispiel ist am Freitag nicht mehr in „Kunst“.</span>
          <RowActions>
            <Action label="Ansehen" />
          </RowActions>
        </li>
      </ResponsiveActionList>
    </Frame>
  ),
};

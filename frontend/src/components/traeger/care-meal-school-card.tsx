"use client";

// Die Mensa-Abrechnung einer Schule (#4686): welcher Preis jetzt gilt, je
// Essen oder als Pauschale, und alle Preise mit ihrem Beginn. Ohne Preis
// rechnet der Monatslauf die Mensa der Schule nicht ab.

import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";
import {
  type CareMealTariff,
  type CareMealTariffs,
  mealTariffOn,
} from "~/lib/traeger-care-meal-api";
import { formatMonth } from "./care-billing-format";
import {
  MEAL_MODE_LABELS,
  MEAL_STAND_LABELS,
  type MealStandStatus,
  mealPriceText,
  mealStandStatus,
  mealStands,
} from "./care-meal-format";
import { formatEuro } from "./contribution-format";

const STAND_TONES: Record<MealStandStatus, StatusBadgeTone> = {
  current: "green",
  planned: "blue",
  earlier: "gray",
};

function columns(currentMonth: string): DataTableColumn<CareMealTariff>[] {
  return [
    {
      key: "from",
      header: "Gültig ab",
      stacked: "title",
      render: (tariff) => (
        <span className="font-medium text-gray-900">
          1. {formatMonth(tariff.validFrom)}
        </span>
      ),
    },
    {
      key: "mode",
      header: "Abrechnungsart",
      stacked: "field",
      render: (tariff) => (
        <span className="text-gray-700">{MEAL_MODE_LABELS[tariff.mode]}</span>
      ),
    },
    {
      key: "amount",
      header: "Betrag",
      align: "right",
      stacked: "field",
      render: (tariff) => (
        <span className="text-gray-900 tabular-nums">
          {formatEuro(tariff.amountCents)}
          <span className="text-gray-500">
            {tariff.mode === "per_meal" ? " je Essen" : " je Kind und Monat"}
          </span>
        </span>
      ),
    },
    {
      key: "status",
      header: "Stand",
      align: "right",
      stacked: "meta",
      render: (tariff) => {
        const status = mealStandStatus(tariff, currentMonth);
        return (
          <StatusBadge
            tone={STAND_TONES[status]}
            label={MEAL_STAND_LABELS[status]}
          />
        );
      },
    },
  ];
}

export function CareMealSchoolCard({
  data,
  school,
  currentMonth,
  onEdit,
  onDelete,
}: Readonly<{
  data: CareMealTariffs;
  school: { id: string; name: string };
  currentMonth: string;
  onEdit: () => void;
  onDelete: (tariff: CareMealTariff) => void;
}>) {
  const stands = mealStands(data.tariffs, school.id);
  const current = mealTariffOn(data.tariffs, school.id, currentMonth);
  const billedUntil = data.billedUntil[school.id];
  const deletable = stands.filter(
    (tariff) => !billedUntil || tariff.validFrom > billedUntil,
  );

  const description = current
    ? `${mealPriceText(current)}, seit 1. ${formatMonth(current.validFrom)}.`
    : stands.length > 0
      ? `Ab 1. ${formatMonth(stands[0]?.validFrom ?? currentMonth)} geplant.`
      : "Noch kein Mensa-Preis. Ohne Preis rechnet der Monatslauf die Mensa dieser Schule nicht ab.";

  return (
    <SectionCard
      title={school.name}
      titleBadge={
        current ? (
          <StatusBadge tone="green" label={MEAL_MODE_LABELS[current.mode]} />
        ) : undefined
      }
      description={
        billedUntil
          ? `${description} Abgerechnet bis ${formatMonth(billedUntil)}; diese Monate behalten ihren Preis.`
          : description
      }
      actions={
        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="md"
            tooltip="Öffnet ein Fenster für einen neuen Mensa-Preis dieser Schule."
            onClick={onEdit}
          >
            Preis festlegen
          </Button>
          {deletable.length > 0 ? (
            <OverflowMenu
              ariaLabel={`Weitere Aktionen: ${school.name}`}
              items={deletable.map((tariff) => ({
                label: `Preis ab ${formatMonth(tariff.validFrom)} löschen`,
                description: "Nur für diese Schule, moto fragt vorher nach",
                destructive: true,
                onClick: () => onDelete(tariff),
              }))}
            />
          ) : null}
        </div>
      }
    >
      {stands.length > 0 ? (
        <DataTable
          columns={columns(currentMonth)}
          rows={[...stands].reverse()}
          getRowKey={(tariff) => tariff.validFrom}
          stackedOnMobile
        />
      ) : null}
    </SectionCard>
  );
}

"use client";

// Freie Plätze je Angebot auf der Seite einer Anmeldephase (#4782). Dieselben
// Zahlen wie in der Angebotsliste. Die Zahlen sind nur zur Information: ein
// Ladefehler blendet den Abschnitt aus und blockiert nichts auf der Seite.

import { useEffect, useMemo, useState } from "react";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import {
  offeringCapacityColumn,
  offeringOccupancyColumn,
  offeringStatusColumn,
} from "~/components/enrollment/offering-occupancy";
import { type CareOffering, listCareOfferings } from "~/lib/care-offering-api";
import {
  type CareOfferingBookingStats,
  fetchCareOfferingBookingStats,
  totalFreePlaces,
} from "~/lib/care-offering-booking-stats";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "PhaseOfferingOccupancy" });

interface OccupancyData {
  offerings: CareOffering[];
  stats: Record<string, CareOfferingBookingStats>;
}

export function PhaseOfferingOccupancy({
  phaseId,
}: Readonly<{ phaseId: string }>) {
  const [data, setData] = useState<OccupancyData | null>(null);

  useEffect(() => {
    let cancelled = false;
    setData(null);
    void Promise.all([
      listCareOfferings(phaseId),
      fetchCareOfferingBookingStats(phaseId),
    ])
      .then(([offerings, stats]) => {
        if (!cancelled) setData({ offerings, stats });
      })
      .catch((err: unknown) => {
        // Bleibt still: die Zahlen sind eine Zusatzinfo, die Seite arbeitet
        // ohne sie weiter. Ohne Belegung zeigt der Abschnitt nichts.
        logger.warn("phase_offering_occupancy_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [phaseId]);

  const columns = useMemo<DataTableColumn<CareOffering>[]>(() => {
    const stats = data?.stats ?? {};
    return [
      {
        key: "name",
        header: "Betreuungsangebot",
        stacked: "title",
        sortValue: (offering) => offering.name,
        render: (offering) => (
          <span className="font-medium text-gray-900">{offering.name}</span>
        ),
      },
      offeringCapacityColumn(),
      offeringOccupancyColumn(stats),
      { ...offeringStatusColumn(stats), stacked: "meta" },
    ];
  }, [data]);

  if (!data || data.offerings.length === 0) return null;

  const freePlaces = totalFreePlaces(data.offerings, data.stats);
  return (
    <SectionCard
      bare
      title="Freie Plätze je Angebot"
      titleBadge={
        freePlaces == null ? undefined : (
          <StatusBadge label={`${freePlaces} frei`} tone="gray" />
        )
      }
      description={
        "Hier sehen Sie nur die freien Plätze. „Belegt“ zählt die Kinder am vollsten Tag bis zum Ende der Phase. Anmeldungen prüfen Sie in der Liste darunter."
      }
    >
      <DataTable
        columns={columns}
        rows={data.offerings}
        getRowKey={(offering) => offering.id}
        stackedOnMobile
        defaultSortKey="name"
        defaultSortDirection="asc"
      />
    </SectionCard>
  );
}

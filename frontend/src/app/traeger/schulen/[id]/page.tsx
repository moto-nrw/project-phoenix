"use client";

// Ansicht einer Schule im Träger-Portal (#5686): Zahlen von jetzt und
// Stammdaten, ohne die Träger-Navigation zu verlassen. Nur Zahlen, keine Namen.
// Die Schul-App öffnet sich als eigene Aktion in einem neuen Tab.

import { ExternalLink } from "lucide-react";
import { useParams } from "next/navigation";
import { useState } from "react";
import {
  SCHOOL_SWITCH_MISSING_HINT,
  useOpenTraegerSchool,
} from "~/components/traeger/use-open-traeger-school";
import { useTraegerSchool } from "~/components/traeger/use-traeger-school";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import { formatBerlinTime } from "~/lib/date-helpers";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { childrenConcept } from "~/lib/concepts/children";
import { presentConcept } from "~/lib/concepts/present";
import { schoolsConcept } from "~/lib/concepts/schools";
import { sickConcept } from "~/lib/concepts/sick";
import { staffConcept } from "~/lib/concepts/staff";
import { supervisionConcept } from "~/lib/concepts/supervision";
import type { TraegerSchoolView } from "~/lib/traeger-api";
import { TRAEGER_HANDOFF_OPEN } from "~/lib/traeger-handoff";
import { traegerPath } from "~/lib/traeger-url";
import { ConceptIconTile } from "~/components/ui/concept-icon-tile";

type SchoolTab = "uebersicht" | "stammdaten";

const SCHOOL_TABS = [
  { value: "uebersicht", label: "Übersicht" },
  { value: "stammdaten", label: "Stammdaten" },
] as const;

const EMPTY_VALUE = "–";

const ratioFormat = new Intl.NumberFormat("de-DE", {
  maximumFractionDigits: 1,
});

function OverviewTab({
  figures,
}: Readonly<{ figures: TraegerSchoolView["figures"] | null }>) {
  const loading = figures === null;
  const ratio = figures?.careRatio ?? null;
  return (
    // Wie auf dem Dashboard: oben die Kinder in drei Spalten, darunter das
    // Personal in zwei breiteren, damit lange Labels neben dem Symbol in
    // einer Zeile bleiben.
    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-6">
      <div className="lg:col-span-2">
        <StatCard
          label="Kinder gesamt"
          value={figures?.childrenTotal ?? 0}
          hint="Alle Kinder, die die Schule gerade betreut."
          icon={<MotoConceptIcon concept={childrenConcept} />}
          loading={loading}
          tone="neutral"
        />
      </div>
      <div className="lg:col-span-2">
        <StatCard
          label="Jetzt anwesend"
          value={figures?.childrenPresent ?? 0}
          hint="Kinder, die gerade in der Schule sind."
          icon={<MotoConceptIcon concept={presentConcept} />}
          loading={loading}
          tone="green"
        />
      </div>
      <div className="lg:col-span-2">
        <StatCard
          label="Heute krank"
          value={figures?.childrenSick ?? 0}
          hint="Kinder mit einer Krankmeldung für heute."
          icon={<MotoConceptIcon concept={sickConcept} />}
          loading={loading}
          tone="orange"
        />
      </div>
      <div className="lg:col-span-3">
        <StatCard
          label="Betreuungskräfte anwesend"
          value={figures?.staffPresent ?? 0}
          hint="Zählt, wer in der Zeiterfassung angemeldet ist. Homeoffice zählt nicht."
          icon={<MotoConceptIcon concept={staffConcept} />}
          loading={loading}
          tone="neutral"
        />
      </div>
      <div className="lg:col-span-3">
        <StatCard
          label="Betreuungsschlüssel"
          value={
            ratio === null ? EMPTY_VALUE : `1 : ${ratioFormat.format(ratio)}`
          }
          hint={
            ratio === null
              ? "Gerade ist keine Betreuungskraft in der Zeiterfassung angemeldet."
              : "Eine anwesende Betreuungskraft kommt auf so viele anwesende Kinder."
          }
          icon={<MotoConceptIcon concept={supervisionConcept} />}
          loading={loading}
          tone="neutral"
        />
      </div>
    </div>
  );
}

function MasterDataTab({
  school,
}: Readonly<{ school: TraegerSchoolView | null }>) {
  const place = [school?.zip, school?.city].filter(Boolean).join(" ");
  return (
    <>
      <SectionCard
        leading={<ConceptIconTile concept={schoolsConcept} />}
        title="Anschrift und Kontakt"
        description="Nur zum Lesen. Änderungen übernimmt das moto-Team."
        loading={school === null}
      >
        <DataGrid columns={2}>
          <DataField label="Name">{school?.name ?? EMPTY_VALUE}</DataField>
          <DataField label="Anschrift">
            {school?.address || place ? (
              <>
                {school?.address}
                {school?.address && place ? <br /> : null}
                {place}
              </>
            ) : (
              EMPTY_VALUE
            )}
          </DataField>
          <DataField label="Telefon">{school?.phone || EMPTY_VALUE}</DataField>
          <DataField label="E-Mail">{school?.email || EMPTY_VALUE}</DataField>
        </DataGrid>
      </SectionCard>
      <SectionCard
        title="Größe der Schule"
        description="Alle Kinder, die die Schule gerade betreut, und alle Betreuungskräfte ohne externe Kräfte."
        loading={school === null}
      >
        <DataGrid columns={2}>
          <DataField label="Kinder">
            {school?.figures.childrenTotal ?? EMPTY_VALUE}
          </DataField>
          <DataField label="Betreuungskräfte">
            {school?.figures.staffTotal ?? EMPTY_VALUE}
          </DataField>
        </DataGrid>
      </SectionCard>
    </>
  );
}

export default function TraegerSchoolPage() {
  const params = useParams<{ id: string }>();
  const { school, error } = useTraegerSchool(params?.id ?? "");
  const { canOpen, opening, open } = useOpenTraegerSchool();
  const [tab, setTab] = useState<SchoolTab>("uebersicht");

  return (
    <TenantPage
      title={school?.name ?? "Schule"}
      leading={
        <ConceptIconTile
          concept={schoolsConcept}
          size="page"
          className="max-sm:hidden"
        />
      }
      back
      backHref={traegerPath("/traeger")}
      backLabel="Zurück zu den Schulen"
      info={
        <>
          Zahlen und Stammdaten dieser Schule. moto zeigt hier keine Namen. Die
          Zahlen aktualisieren sich jede Minute.
          {!canOpen && ` ${SCHOOL_SWITCH_MISSING_HINT}`}
        </>
      }
      stats={
        school ? (
          <TenantPageStats
            items={[
              {
                value: `Stand ${formatBerlinTime(school.figures.asOf)} Uhr`,
                label: "",
              },
            ]}
          />
        ) : null
      }
      statsLoading={school === null && !error}
      error={error}
      actions={
        canOpen && school ? (
          <Button
            type="button"
            variant="surface"
            size="md"
            className="gap-2"
            aria-label={`${TRAEGER_HANDOFF_OPEN}: ${school.name}, neuer Tab`}
            title="Öffnet in einem neuen Tab"
            disabled={opening !== null}
            onClick={() => void open(school)}
          >
            {opening === null ? (
              <>
                {TRAEGER_HANDOFF_OPEN}
                <ExternalLink className="h-4 w-4" aria-hidden="true" />
              </>
            ) : (
              "Wird geöffnet …"
            )}
          </Button>
        ) : null
      }
      tabs={{
        value: tab,
        onChange: (value) => setTab(value as SchoolTab),
        items: SCHOOL_TABS,
        label: "Bereiche der Schule",
      }}
    >
      {tab === "uebersicht" ? (
        <OverviewTab figures={school?.figures ?? null} />
      ) : (
        <MasterDataTab school={school} />
      )}
    </TenantPage>
  );
}

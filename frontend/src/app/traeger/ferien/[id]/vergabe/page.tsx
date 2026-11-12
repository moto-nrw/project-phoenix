"use client";

// Vergabe nach Kriterien (#4700): moto sortiert die offenen Anmeldungen jeder
// Woche nach den Kriterien des Trägers vor, dann nach Eingang. Das ist nur
// ein Vorschlag. Das Büro entscheidet jede Anmeldung einzeln oder übernimmt
// den Vorschlag einer Woche oder aller Wochen; eine Abweichung braucht eine
// Begründung. Eltern bekommen nach der Entscheidung eine E-Mail. Danach
// gelten Warteliste und Nachrücken wie bisher.

import { CheckCircle2, ListChecks } from "lucide-react";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  type AllocationDecisionTarget,
  HolidayAllocationDecisionModal,
  isStaleAllocationError,
} from "~/components/traeger/holiday-allocation-decision-modal";
import { HolidayAllocationHistory } from "~/components/traeger/holiday-allocation-history";
import {
  criterionTone,
  openWeeks,
  proposalDecisions,
  proposalSummary,
  proposalSummaryText,
} from "~/components/traeger/holiday-allocation-model";
import { HolidayAllocationWeekCard } from "~/components/traeger/holiday-allocation-week";
import {
  HolidayHeadFacts,
  holidayPeriodTabs,
} from "~/components/traeger/holiday-period-head";
import { schoolNames } from "~/components/traeger/holiday-registration-format";
import { useTraegerHolidayPeriod } from "~/components/traeger/use-traeger-resources";
import { useTraegerSchools } from "~/components/traeger/use-traeger-schools";
import { Button, ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  decideHolidayAllocation,
  fetchHolidayAllocation,
  type HolidayAllocationBoard,
  type HolidayAllocationWeek,
  MAX_ALLOCATION_DECISIONS,
} from "~/lib/traeger-holiday-allocation-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerHolidayAllocationPage" });

/** Which weeks „Vorschlag übernehmen“ covers: one week or all open ones. */
type ProposalScope = { readonly weekId: string } | "all";

export default function TraegerHolidayAllocationPage() {
  const params = useParams<{ id: string }>();
  const id = params?.id ?? "";
  const {
    data: period,
    preview,
    loadError: periodError,
  } = useTraegerHolidayPeriod(id);
  const { schools } = useTraegerSchools();
  const names = useMemo(() => schoolNames(schools), [schools]);
  const [board, setBoard] = useState<HolidayAllocationBoard | null>(null);
  const [target, setTarget] = useState<AllocationDecisionTarget | null>(null);
  const [proposalScope, setProposalScope] = useState<ProposalScope | null>(
    null,
  );
  const [taking, setTaking] = useState(false);
  const boardLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = boardLoad;
  const actionErrors = useApiErrorDisplay();
  const toast = useToast();
  const byCriteria = period?.allocationMode === "criteria";

  const load = useCallback(async () => {
    try {
      setBoard(await fetchHolidayAllocation(id));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_holiday_allocation_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Vergabe",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, id, showLoadError]);

  useEffect(() => {
    // A period by arrival has no allocation list; the page says so instead.
    if (byCriteria) void load();
  }, [byCriteria, load]);

  const weeks = useMemo(() => (board ? openWeeks(board) : []), [board]);
  const pendingTotal = weeks.reduce(
    (sum, week) => sum + week.pending.length,
    0,
  );
  const proposalWeeks: readonly HolidayAllocationWeek[] =
    proposalScope === null
      ? []
      : proposalScope === "all"
        ? weeks
        : weeks.filter((week) => week.weekId === proposalScope.weekId);
  const summary = proposalSummary(proposalWeeks);

  // The list changed meanwhile: read it again and say why through the path.
  const reloadAfterStale = useCallback(
    (err: unknown) => {
      setTarget(null);
      setProposalScope(null);
      void actionErrors.show(err, { object: "die Vergabe" });
      void load();
    },
    [actionErrors, load],
  );

  const decideOne = async (reason: string) => {
    if (!target) return;
    const next = await decideHolidayAllocation(id, [
      {
        bookingId: target.entry.bookingId,
        placement: target.placement,
        proposal: target.entry.proposal,
        reason,
      },
    ]);
    setBoard(next);
    setTarget(null);
    toast.success(
      target.placement === "placed"
        ? `${target.entry.childFirstName} hat einen Platz. Die Eltern bekommen eine E-Mail.`
        : `${target.entry.childFirstName} steht auf der Warteliste. Die Eltern bekommen eine E-Mail.`,
    );
  };

  const takeProposal = async () => {
    setTaking(true);
    try {
      if (!board) return;
      // Keep the confirmed placements and expected proposals across all batches.
      // The backend rejects stale proposals; new bookings need a new decision.
      const confirmed = proposalDecisions(proposalWeeks);
      for (
        let offset = 0;
        offset < confirmed.length;
        offset += MAX_ALLOCATION_DECISIONS
      ) {
        const choices = confirmed.slice(
          offset,
          offset + MAX_ALLOCATION_DECISIONS,
        );
        const next = await decideHolidayAllocation(id, choices);
        // Preserve completed batches even when a later batch needs review.
        setBoard(next);
      }
      setProposalScope(null);
      toast.success(
        `Der Vorschlag ist übernommen. ${summary.registrations === 1 ? "Die Eltern bekommen" : `Die Eltern von ${summary.registrations} Anmeldungen bekommen`} eine E-Mail.`,
      );
    } catch (err) {
      logger.error("traeger_holiday_allocation_proposal_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      if (isStaleAllocationError(err)) {
        reloadAfterStale(err);
      } else {
        setProposalScope(null);
        void actionErrors.show(err, { object: "der Vorschlag" });
        void load();
      }
    } finally {
      setTaking(false);
    }
  };

  const loadError = boardLoad.error ?? periodError;
  const loading = !loadError && (!period || (byCriteria && !board));
  const busy = taking || target !== null;

  return (
    <TenantPage
      title={period?.name ?? preview?.name ?? "Ferienzeitraum"}
      back
      backHref={traegerPath("/traeger/ferien")}
      backLabel="Zurück zu den Ferien"
      info={
        <p>
          moto sortiert die offenen Anmeldungen jeder Woche nach Ihren
          Kriterien. Bei gleichen Kriterien zählt der Eingang. Das ist nur ein
          Vorschlag. Ihr Büro entscheidet über jeden Platz. Weichen Sie vom
          Vorschlag ab, schreiben Sie kurz, warum. Die Eltern bekommen nach der
          Entscheidung eine E-Mail. Danach rücken Kinder von der Warteliste
          automatisch nach, wenn ein Platz frei wird.
        </p>
      }
      stats={
        board ? (
          <HolidayHeadFacts>
            <span>
              {pendingTotal === 1
                ? "1 Anmeldung offen"
                : `${pendingTotal} Anmeldungen offen`}
            </span>
            <span>
              {board.decisions.length === 1
                ? "1 Entscheidung"
                : `${board.decisions.length} Entscheidungen`}
            </span>
          </HolidayHeadFacts>
        ) : null
      }
      statsLoading={byCriteria && !board && !loadError}
      tabs={holidayPeriodTabs(
        id,
        "vergabe",
        period?.registrationCount,
        period?.allocationMode,
      )}
      actions={
        weeks.length > 1 ? (
          <Button
            type="button"
            variant="primary"
            size="md"
            disabled={busy}
            tooltip="moto fragt vorher nach, dann bekommen alle betroffenen Eltern eine E-Mail."
            onClick={() => setProposalScope("all")}
          >
            Vorschlag für alle Wochen übernehmen
          </Button>
        ) : null
      }
      error={loadError}
      loading={loading}
      empty={
        period && !byCriteria
          ? {
              title: "Dieser Ferienzeitraum vergibt Plätze nach Eingang",
              description:
                "Hier gibt es nichts zu entscheiden. Wer sich zuerst anmeldet, bekommt zuerst einen Platz. Das Verfahren steht im Reiter „Planung“.",
            }
          : undefined
      }
    >
      {byCriteria && board ? (
        <div className="flex flex-col gap-6">
          <SectionCard
            title="So sortiert moto vor"
            description="Zuerst zählt das Kriterium mit der höchsten Priorität, dann das nächste. Bei Gleichstand zählt der Eingang. Sie entscheiden."
            action={
              <ButtonLink
                href={traegerPath("/traeger/ferien/kriterien")}
                variant="outline"
                size="md"
                className="gap-2"
                tooltip="Öffnet die Kriterien für die Platzvergabe in allen Ferien."
              >
                <ListChecks className="h-4 w-4" aria-hidden="true" />
                Kriterien ändern
              </ButtonLink>
            }
          >
            {board.criteria.length === 0 ? (
              <p className="text-sm text-gray-700">
                Es ist kein Kriterium eingeschaltet. moto sortiert darum nur
                nach Eingang.
              </p>
            ) : (
              <ol
                className="flex flex-wrap gap-2"
                aria-label="Kriterien nach Priorität"
              >
                {board.criteria.map((criterion) => (
                  <li key={criterion.id} className="max-w-full min-w-0">
                    <StatusBadge
                      wrap
                      className="[overflow-wrap:anywhere]"
                      tone={criterionTone(criterion.position)}
                      label={`${criterion.position}. ${criterion.name}`}
                    />
                  </li>
                ))}
              </ol>
            )}
          </SectionCard>

          {weeks.length === 0 ? (
            <SectionCard>
              <EmptyState
                tone="success"
                icon={<CheckCircle2 aria-hidden="true" />}
                title="Alles entschieden"
                description="Es gibt keine offene Anmeldung. Neue Anmeldungen erscheinen hier, vorsortiert nach Ihren Kriterien."
                variant="compact"
              />
            </SectionCard>
          ) : (
            weeks.map((week) => (
              <HolidayAllocationWeekCard
                key={week.weekId}
                periodId={id}
                week={week}
                hostSchoolName={names.get(week.hostSchoolId) ?? "Schule"}
                criteria={board.criteria}
                busy={busy}
                onDecide={(entry, placement) =>
                  setTarget({ week, entry, placement })
                }
                onTakeProposal={() => setProposalScope({ weekId: week.weekId })}
              />
            ))
          )}

          <HolidayAllocationHistory
            periodId={id}
            decisions={board.decisions}
            defaultCollapsed={weeks.length > 0}
          />
        </div>
      ) : null}

      <HolidayAllocationDecisionModal
        target={target}
        onClose={() => setTarget(null)}
        onConfirm={decideOne}
        onStale={reloadAfterStale}
      />
      <ConfirmationModal
        isOpen={proposalScope !== null}
        title="Vorschlag übernehmen?"
        confirmText="Vorschlag übernehmen"
        confirmVariant="success"
        isConfirmLoading={taking}
        isDismissDisabled={taking}
        loadingText="Übernimmt …"
        onClose={() => setProposalScope(null)}
        onConfirm={() => void takeProposal()}
      >
        <div className="space-y-2 text-sm text-gray-700">
          <p>{proposalSummaryText(summary)}</p>
          <p>
            {summary.registrations === 1
              ? "Die Eltern bekommen eine E-Mail."
              : `Die Eltern von ${summary.registrations} Anmeldungen bekommen eine E-Mail.`}
          </p>
        </div>
      </ConfirmationModal>
    </TenantPage>
  );
}

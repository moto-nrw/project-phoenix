"use client";

// Regeln des Monatsbeitrags (#4685): Einkommensstufen, Geschwister,
// Alleinerziehende und Höchstbetrag, je eine Karte in der Reihenfolge, in
// der der Monatslauf sie anwendet. Die Nachweise der Kinder liegen auf der
// Seite „Nachweise“.

import { Tags } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { billingTabs } from "~/components/traeger/billing-family-format";
import { formatMonth, monthOf } from "~/components/traeger/care-billing-format";
import { CareRuleModal } from "~/components/traeger/care-rule-modal";
import { CareRulesCard } from "~/components/traeger/care-rules-card";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { EmptyState } from "~/components/ui/empty-state";
import { TenantPage, TenantPageStats } from "~/components/ui/tenant-page";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { berlinTodayISO } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  CARE_RULE_KINDS,
  type CareRule,
  type CareRuleKind,
  type CareRuleSet,
  deleteCareRule,
  fetchCareRules,
} from "~/lib/traeger-care-reductions-api";

const logger = createLogger({ component: "TraegerErmaessigungenPage" });

export default function TraegerErmaessigungenPage() {
  const currentMonth = monthOf(berlinTodayISO());
  const [data, setData] = useState<CareRuleSet | null>(null);
  const [editing, setEditing] = useState<{
    kind: CareRuleKind;
    rule: CareRule | null;
  } | null>(null);
  const [deleting, setDeleting] = useState<CareRule | null>(null);
  const [busy, setBusy] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const deleteErrors = useApiFormError();
  const { success } = useToast();

  const load = useCallback(async () => {
    try {
      setData(await fetchCareRules());
      clearLoadError();
    } catch (err) {
      logger.error("traeger_care_rules_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Ermäßigungen",
        retry: () => void load(),
      });
    }
  }, [clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const remove = async () => {
    if (!deleting) return;
    setBusy(true);
    deleteErrors.clear();
    try {
      setData(await deleteCareRule(deleting.id));
      success(`Die Regel „${deleting.name}“ ist gelöscht.`);
      setDeleting(null);
    } catch (err) {
      logger.error("care_rule_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void deleteErrors.show(err, { object: "die Regel" });
    } finally {
      setBusy(false);
    }
  };

  const active = data
    ? data.rules.filter(
        (rule) =>
          rule.validFrom <= currentMonth &&
          (rule.validUntil === null || rule.validUntil >= currentMonth),
      ).length
    : 0;

  return (
    <TenantPage
      title="Abrechnung"
      info="Hier legen Sie fest, wann Familien weniger zahlen. Der Monatslauf rechnet in dieser Reihenfolge: Preis aus der Preisliste, Einkommensstufe, Geschwister, Alleinerziehende, Höchstbetrag. Jede Ermäßigung steht als eigene Zeile auf der Rechnung."
      stats={
        data && data.rules.length > 0 ? (
          <TenantPageStats
            items={[
              {
                value: data.rules.length,
                label: data.rules.length === 1 ? "Regel" : "Regeln",
              },
              {
                value: active,
                label: `gelten im ${formatMonth(currentMonth)}`,
              },
            ]}
          />
        ) : null
      }
      statsLoading={!data && !pageLoad.error}
      tabs={billingTabs("ermaessigungen")}
      error={pageLoad.error}
      loading={!pageLoad.error && !data}
      overlays={
        data ? (
          <>
            <CareRuleModal
              data={data}
              open={editing}
              currentMonth={currentMonth}
              onClose={() => setEditing(null)}
              onSaved={(next) => {
                setData(next);
                setEditing(null);
              }}
            />
            <ConfirmDeleteModal
              isOpen={deleting !== null}
              onClose={() => {
                setDeleting(null);
                deleteErrors.clear();
              }}
              onConfirm={() => void remove()}
              title="Regel löschen?"
              description={
                deleting
                  ? `„${deleting.name}“ gilt ab dem nächsten Monatslauf nicht mehr. Ausgestellte Rechnungen bleiben, wie sie sind.`
                  : ""
              }
              confirmLabel="Löschen"
              loadingLabel="Wird gelöscht …"
              gate={{ mode: "twoStep", firstStepLabel: "Ja, löschen" }}
              loading={busy}
              error={deleteErrors.error}
            />
          </>
        ) : null
      }
    >
      {data && data.schools.length === 0 ? (
        <EmptyState
          icon={<Tags className="h-12 w-12" aria-hidden="true" />}
          title="Noch keine Schule."
          description="Ermäßigungen gibt es für die Schulen Ihres Trägers."
        />
      ) : null}
      {data && data.schools.length > 0
        ? CARE_RULE_KINDS.map((kind) => (
            <CareRulesCard
              key={kind}
              kind={kind}
              data={data}
              currentMonth={currentMonth}
              onCreate={() => setEditing({ kind, rule: null })}
              onEdit={(rule) => setEditing({ kind, rule })}
              onDelete={setDeleting}
            />
          ))
        : null}
    </TenantPage>
  );
}

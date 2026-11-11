"use client";

// SEPA-Mandate (#4695). Mit `contractId` die Mandate der Familie dieses
// Vertrags, mit dem Knopf „Mandat erfassen“, solange keines gilt; ohne die
// Mandate aller Familien des Trägers mit einem Link zum Vertrag. Jedes Mandat
// zeigt die IBAN nur maskiert. Ein gültiges Mandat lässt sich widerrufen; das
// widerrufene bleibt im Verlauf. Nur die Rolle „Abrechnung“ sieht die Karte;
// jede andere bekommt 403, und die Karte bleibt weg.

import { Eye, Undo2 } from "lucide-react";
import { useCallback, useEffect, useState } from "react";
import { Button } from "~/components/ui/button";
import { ISODatePicker } from "~/components/ui/date-picker";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import NavigationLink from "~/components/ui/navigation-link";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import {
  berlinTodayISO,
  formatBerlinDate,
  formatBerlinTime,
  formatDate,
} from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type SepaMandate,
  type SepaMandateEvent,
  fetchSepaMandateHistory,
  fetchSepaMandates,
  revokeSepaMandate,
} from "~/lib/traeger-sepa-api";
import { fetchTraegerBuero } from "~/lib/traeger-api";
import { traegerPath } from "~/lib/traeger-url";
import { SepaMandateModal } from "./sepa-mandate-modal";
import {
  MANDATE_KIND_LABELS,
  mandateEventActor,
  mandateEventLabel,
  mandateFamily,
  mandateState,
  nextDebitLabel,
} from "./sepa-format";

const logger = createLogger({ component: "SepaMandatesCard" });

/**
 * Loads the mandates; null while loading. A session without the right
 * „Abrechnung“ gets 403: then `forbidden` is set and nothing is shown.
 */
export function useSepaMandates(enabled: boolean, contractId?: string) {
  const [mandates, setMandates] = useState<SepaMandate[] | null>(null);
  const [forbidden, setForbidden] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;

  const load = useCallback(async () => {
    if (!enabled) return;
    try {
      setMandates(await fetchSepaMandates(contractId));
      setForbidden(false);
      clearLoadError();
    } catch (err) {
      if (err instanceof ApiError && err.status === 403) {
        // Mandates belong to the right „Abrechnung“; other roles do not
        // see the card at all.
        setForbidden(true);
        return;
      }
      logger.error("sepa_mandates_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "die Liste der Mandate",
        retry: () => void load(),
      });
    }
  }, [enabled, contractId, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  return { mandates, forbidden, error: pageLoad.error, load };
}

function RevokeModal({
  mandate,
  onClose,
  onRevoked,
}: Readonly<{
  mandate: SepaMandate | null;
  onClose: () => void;
  onRevoked: () => void;
}>) {
  const today = berlinTodayISO();
  const [revokedOn, setRevokedOn] = useState(today);
  const [saving, setSaving] = useState(false);
  const { show } = useApiErrorDisplay();
  const { success } = useToast();

  useEffect(() => {
    if (mandate) setRevokedOn(today);
  }, [mandate, today]);

  const confirm = async () => {
    if (!mandate) return;
    setSaving(true);
    try {
      await revokeSepaMandate(mandate.id, revokedOn);
      success(`Das Mandat ${mandate.reference} ist widerrufen.`);
      onRevoked();
    } catch (err) {
      logger.error("sepa_mandate_revoke_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void show(err, { object: "das Mandat" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <ConfirmationModal
      isOpen={mandate !== null}
      onClose={onClose}
      onConfirm={() => void confirm()}
      title="Mandat widerrufen"
      confirmText="Widerrufen"
      confirmVariant="danger"
      isConfirmLoading={saving}
      isConfirmDisabled={!revokedOn}
    >
      <div className="space-y-4">
        <p className="text-sm text-gray-700">
          Ab dem nächsten Lauf buchen Sie bei dieser Familie nicht mehr ab. Sie
          bekommt dann eine Rechnung zum Überweisen, bis ein neues Mandat da
          ist. Ausgestellte Rechnungen bleiben, wie sie sind.
        </p>
        <ISODatePicker
          id="sepa-mandate-revoked"
          label="Widerruf erhalten am"
          required
          value={revokedOn}
          min={mandate?.signedOn}
          max={today}
          onChange={setRevokedOn}
        />
      </div>
    </ConfirmationModal>
  );
}

function HistoryModal({
  mandate,
  onClose,
}: Readonly<{ mandate: SepaMandate | null; onClose: () => void }>) {
  const [events, setEvents] = useState<SepaMandateEvent[] | null>(null);
  const [emails, setEmails] = useState<ReadonlyMap<string, string>>(new Map());
  const { show } = useApiErrorDisplay();

  useEffect(() => {
    if (!mandate) return;
    // A trail that arrives after the dialog moved on to another mandate
    // belongs to that one; it is dropped.
    let current = true;
    setEvents(null);
    fetchSepaMandateHistory(mandate.id)
      .then((loaded) => {
        if (current) setEvents(loaded);
      })
      .catch((err: unknown) => {
        logger.error("sepa_mandate_history_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!current) return;
        setEvents([]);
        void show(err, { object: "die Liste der Änderungen" });
      });
    // The people of the Träger-Büro name the accounts; a role that may not
    // read them sees the account number instead.
    fetchTraegerBuero()
      .then((buero) => {
        if (!current) return;
        setEmails(
          new Map(
            buero.members.map((member) => [member.accountId, member.email]),
          ),
        );
      })
      .catch((err: unknown) => {
        logger.warn("sepa_mandate_history_people_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      current = false;
    };
  }, [mandate, show]);

  return (
    <Modal
      isOpen={mandate !== null}
      onClose={onClose}
      title={mandate ? `Verlauf: ${mandate.reference}` : "Verlauf"}
      description="Wer das Mandat wann angesehen oder geändert hat, ohne Kontodaten."
      size="md"
    >
      {events === null ? (
        <p className="text-sm text-gray-600">Wird geladen …</p>
      ) : events.length === 0 ? (
        <p className="text-sm text-gray-600">Noch keine Einträge.</p>
      ) : (
        <ol className="divide-y divide-gray-100">
          {[...events].reverse().map((event) => (
            <li
              key={event.id}
              className="flex items-baseline justify-between gap-4 py-2.5 text-sm"
            >
              <span className="min-w-0">
                <span className="block text-gray-900">
                  {mandateEventLabel(event)}
                </span>
                <span className="block wrap-anywhere text-gray-500">
                  {mandateEventActor(event, emails)}
                </span>
              </span>
              <span className="shrink-0 text-gray-500 tabular-nums">
                {formatBerlinDate(event.occurredAt)},{" "}
                {formatBerlinTime(event.occurredAt)} Uhr
              </span>
            </li>
          ))}
        </ol>
      )}
    </Modal>
  );
}

/** The menu of one mandate: its trail and, while in force, the revocation. */
function MandateMenu({
  mandate,
  onHistory,
  onRevoke,
}: Readonly<{
  mandate: SepaMandate;
  onHistory: (mandate: SepaMandate) => void;
  onRevoke: (mandate: SepaMandate) => void;
}>) {
  return (
    <OverflowMenu
      ariaLabel={`Aktionen für das Mandat ${mandate.reference}`}
      items={[
        {
          icon: <Eye className="size-4" aria-hidden />,
          label: "Verlauf ansehen",
          onClick: () => onHistory(mandate),
        },
        ...(mandate.revokedOn
          ? []
          : [
              {
                label: "Widerrufen",
                icon: <Undo2 className="size-4" aria-hidden />,
                destructive: true,
                onClick: () => onRevoke(mandate),
              },
            ]),
      ]}
    />
  );
}

/**
 * The mandates of one family in a narrow column: the one in force as fields,
 * the earlier ones as short lines below.
 */
function FamilyMandates({
  mandates,
  onHistory,
  onRevoke,
}: Readonly<{
  mandates: readonly SepaMandate[];
  onHistory: (mandate: SepaMandate) => void;
  onRevoke: (mandate: SepaMandate) => void;
}>) {
  const inForce = mandates.find((mandate) => !mandate.revokedOn);
  const earlier = mandates.filter((mandate) => mandate.revokedOn);
  return (
    <div className="space-y-5">
      {inForce ? (
        <div className="flex items-start gap-3">
          <div className="min-w-0 flex-1">
            <DataGrid columns={1}>
              <DataField label="Mandatsreferenz">
                <span className="font-mono break-all">{inForce.reference}</span>
                <span className="text-gray-500">
                  {" "}
                  · {MANDATE_KIND_LABELS[inForce.kind]}
                </span>
              </DataField>
              <DataField label="Konto">
                <span
                  className="block font-mono tabular-nums md:whitespace-nowrap"
                  data-analytics-block
                >
                  {inForce.ibanMasked}
                </span>
                <span className="block text-gray-600">
                  Kontoinhaber: {inForce.accountHolder}
                </span>
              </DataField>
              <DataField label="Unterschrieben am">
                <span className="tabular-nums">
                  {formatDate(inForce.signedOn)}
                </span>
              </DataField>
              <DataField label="Art der nächsten Abbuchung">
                {nextDebitLabel(inForce)}
              </DataField>
              <DataField label="Gläubiger-ID">
                <span className="font-mono">{inForce.creditorId}</span>
                {inForce.creditorCurrent ? null : (
                  <span className="text-moto-red-strong block">
                    Gilt nicht mehr: Ihre Gläubiger-ID hat sich geändert.
                    Widerrufen Sie dieses Mandat und erfassen Sie das neue.
                  </span>
                )}
              </DataField>
            </DataGrid>
          </div>
          <span className="ml-auto shrink-0">
            <MandateMenu
              mandate={inForce}
              onHistory={onHistory}
              onRevoke={onRevoke}
            />
          </span>
        </div>
      ) : null}
      {earlier.length > 0 ? (
        <div className="space-y-2">
          <p className="text-sm font-semibold text-gray-600">Frühere Mandate</p>
          <ul className="divide-y divide-gray-100">
            {earlier.map((mandate) => (
              <li
                key={mandate.id}
                className="flex items-center justify-between gap-3 py-2 text-sm"
              >
                <span className="min-w-0">
                  <span className="block font-mono break-all text-gray-900">
                    {mandate.reference}
                  </span>
                  <span className="block text-gray-500">
                    Widerrufen am {formatDate(mandate.revokedOn ?? "")}
                  </span>
                </span>
                <MandateMenu
                  mandate={mandate}
                  onHistory={onHistory}
                  onRevoke={onRevoke}
                />
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  );
}

export function SepaMandatesCard({
  contractId,
  holder = "",
  mandates: given,
  scopeLabel = "Alle",
  onChanged,
}: Readonly<{
  /** The contract whose family the card shows; none shows every family. */
  contractId?: string;
  /** The name the form proposes as account holder. */
  holder?: string;
  /** Mandates the page loaded itself; the card loads them otherwise. */
  mandates?: SepaMandate[];
  /** Which mandates the page shows, named in the card („Gültige“). */
  scopeLabel?: string;
  onChanged?: () => void;
}>) {
  const own = useSepaMandates(given === undefined, contractId);
  const mandates = given ?? own.mandates;
  const [creating, setCreating] = useState(false);
  const [revoking, setRevoking] = useState<SepaMandate | null>(null);
  const [history, setHistory] = useState<SepaMandate | null>(null);

  if (own.forbidden) return null;

  const changed = () => {
    void own.load();
    onChanged?.();
  };
  const inForce = mandates?.find((mandate) => !mandate.revokedOn);
  const forFamily = contractId !== undefined;

  const columns: DataTableColumn<SepaMandate>[] = [
    ...(forFamily
      ? []
      : [
          {
            key: "family",
            header: "Familie",
            stacked: "title" as const,
            sortValue: (mandate: SepaMandate) => mandateFamily(mandate),
            render: (mandate: SepaMandate) => (
              <NavigationLink
                href={traegerPath(
                  `/traeger/abrechnung/lastschrift/${mandate.contractId}`,
                )}
                className="font-medium text-gray-900 underline-offset-2 hover:underline md:whitespace-nowrap"
              >
                {mandateFamily(mandate)}
              </NavigationLink>
            ),
          },
        ]),
    {
      key: "reference",
      header: "Mandatsreferenz",
      stacked: forFamily ? "title" : "field",
      render: (mandate) => (
        <span className="block min-w-0">
          {/* A reference is one ID: it stays whole from the tablet on. */}
          <span className="block font-mono wrap-anywhere text-gray-900 md:whitespace-nowrap">
            {mandate.reference}
          </span>
          <span className="block text-sm text-gray-500">
            {MANDATE_KIND_LABELS[mandate.kind]}
          </span>
          {/* On a tablet the next debit stands under the kind, named: the
              table has no room for its own column there. Its value stays
              whole; the line breaks after the colon. */}
          {mandate.revokedOn ? null : (
            <span className="hidden text-sm text-gray-500 md:block xl:hidden">
              Nächste Abbuchung:{" "}
              <span className="whitespace-nowrap">
                {nextDebitLabel(mandate)}
              </span>
            </span>
          )}
        </span>
      ),
    },
    {
      key: "account",
      header: "Konto",
      stacked: "field",
      // On a phone the account takes the row's full width: the masked IBAN
      // stays on one line, and „Kontoinhaber“ names the second line.
      stackedLabel: false,
      render: (mandate) => (
        <span className="block min-w-0" data-analytics-block>
          {/* The phone row shows its label above the full-width IBAN;
              screen readers hear the field's own label. */}
          <span className="block text-sm text-gray-500 md:hidden" aria-hidden>
            Konto
          </span>
          {/* One line on every width: a split IBAN is misread. */}
          <span className="block font-mono whitespace-nowrap text-gray-900 tabular-nums">
            {/* The last two groups are the digits that tell accounts apart:
                they stay together. */}
            {mandate.ibanMasked.replace(/ (\S+)$/, "\u00A0$1")}
          </span>
          <span className="block truncate text-sm text-gray-500">
            Kontoinhaber: {mandate.accountHolder}
          </span>
        </span>
      ),
    },
    {
      key: "signed",
      header: "Unterschrieben",
      stacked: "field",
      sortValue: (mandate) => mandate.signedOn,
      render: (mandate) => (
        <span className="whitespace-nowrap tabular-nums">
          {formatDate(mandate.signedOn)}
        </span>
      ),
    },
    {
      // Its own column from the laptop on, so first and later debits
      // compare down the list; on a phone its own field.
      key: "next",
      header: "Nächste Abbuchung",
      stacked: "field",
      className: "max-xl:hidden",
      headerClassName: "max-xl:hidden",
      render: (mandate) => nextDebitLabel(mandate),
    },
    {
      key: "state",
      header: "Stand",
      stacked: "meta",
      render: (mandate) => {
        const state = mandateState(mandate);
        return (
          <span className="flex flex-col items-start gap-1">
            <StatusBadge label={state.label} tone={state.tone} />
            {mandate.revokedOn ? (
              <span className="text-sm whitespace-nowrap text-gray-500">
                seit {formatDate(mandate.revokedOn)}
              </span>
            ) : null}
          </span>
        );
      },
    },
    {
      key: "actions",
      header: "",
      align: "right",
      stacked: "meta",
      stackedLabel: false,
      render: (mandate) => (
        <MandateMenu
          mandate={mandate}
          onHistory={setHistory}
          onRevoke={setRevoking}
        />
      ),
    },
  ];

  const createAction =
    forFamily && mandates !== null && !inForce ? (
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={() => setCreating(true)}
      >
        Mandat erfassen
      </Button>
    ) : null;

  return (
    <SectionCard
      title={forFamily ? "SEPA-Lastschriftmandat" : "Mandate"}
      titleBadge={
        forFamily && mandates !== null ? (
          inForce ? (
            <StatusBadge
              tone={mandateState(inForce).tone}
              label={mandateState(inForce).label}
            />
          ) : (
            <StatusBadge tone="orange" label="Kein gültiges Mandat" />
          )
        ) : null
      }
      description={
        forFamily
          ? "Damit buchen Sie den Beitrag dieser Familie ab. Die IBAN sehen Sie nur verdeckt."
          : `${scopeLabel} Mandate Ihrer Familien. Ein neues Mandat erfassen Sie beim Vertrag der Familie unter „Mandat erfassen“.`
      }
      actions={createAction}
      loading={mandates === null && !own.error}
      overflow="visible"
    >
      {own.error && given === undefined ? (
        <LoadErrorAlert error={own.error} actionLayout="responsive" />
      ) : forFamily && mandates && mandates.length > 0 ? (
        <FamilyMandates
          mandates={mandates}
          onHistory={setHistory}
          onRevoke={setRevoking}
        />
      ) : mandates && mandates.length > 0 ? (
        <DataTable
          columns={columns}
          rows={mandates}
          getRowKey={(mandate) => mandate.id}
          rowHasInteractiveControls
          // Seven columns on a laptop: narrower cells keep every one in view.
          dense
          stackedOnMobile
        />
      ) : mandates ? (
        <EmptyState
          variant="compact"
          title="Noch kein Mandat"
          description={
            forFamily
              ? "Ohne Mandat bekommt die Familie eine Rechnung zum Überweisen."
              : "Erfassen Sie ein Mandat beim Vertrag der Familie."
          }
        />
      ) : null}
      {forFamily ? (
        <SepaMandateModal
          open={creating}
          contractId={contractId}
          holder={holder}
          onClose={() => setCreating(false)}
          onSaved={() => {
            setCreating(false);
            changed();
          }}
        />
      ) : null}
      <RevokeModal
        mandate={revoking}
        onClose={() => setRevoking(null)}
        onRevoked={() => {
          setRevoking(null);
          changed();
        }}
      />
      <HistoryModal mandate={history} onClose={() => setHistory(null)} />
    </SectionCard>
  );
}

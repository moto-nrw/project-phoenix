"use client";

// Die Karten der Seite „Konten und Kostenstellen“ (#4821): auf welches
// Erlöskonto moto jede Leistung und jedes Angebot bucht, und welche
// Kostenstelle ab welchem Tag gilt. Eine Rechnung schreibt die Werte beim
// Ausstellen fest; eine Änderung hier gilt für neue Rechnungen.

import { Coins, Landmark, Layers } from "lucide-react";
import { useState } from "react";
import {
  costCenterError,
  costCenterOn,
  costCenterTimeline,
  effectiveRevenue,
  nextCostCenter,
  ownRevenueAccount,
  REVENUE_SERVICES,
  revenueAccountError,
} from "~/components/traeger/booking-accounts-format";
import { DATEV_SETTINGS_PATH } from "~/components/traeger/finance-export-datev";
import { Button, ButtonLink } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { ISODatePicker } from "~/components/ui/date-picker";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { InsetPanel } from "~/components/ui/inset-panel";
import { Input } from "~/components/ui/input";
import { ListGroupHeader } from "~/components/ui/list-group-header";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import type {
  BookingAccounts,
  CostCenter,
  CostCenterScope,
  RevenueScope,
} from "~/lib/traeger-finance-exports-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "BookingAccountsCards" });

type SubmitLike = { preventDefault: () => void };

const ROW =
  "flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center";
const ACTIONS =
  "flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end";

function errorText(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

/** The account as the screen shows it: the number and where it comes from. */
function AccountValue({
  account,
  source,
}: Readonly<{
  account: string;
  source: ReturnType<typeof effectiveRevenue>["source"];
}>) {
  if (source === "missing") {
    return <StatusBadge tone="warning" label="Kein Konto" />;
  }
  return (
    <span className="flex flex-wrap items-center gap-2">
      <span className="font-mono text-sm font-semibold text-gray-900 tabular-nums">
        {account}
      </span>
      {source === "own" ? null : (
        <StatusBadge
          tone="gray"
          size="xs"
          label={
            source === "service" ? "wie Monatsbeiträge" : "Standard-Erlöskonto"
          }
        />
      )}
    </span>
  );
}

/** One revenue account row; „Ändern“ opens its field in the row. */
function RevenueRow({
  title,
  hint,
  scope,
  scopeId,
  page,
  onSave,
}: Readonly<{
  title: string;
  hint: string;
  scope: RevenueScope;
  scopeId: string;
  page: BookingAccounts;
  onSave: (
    scope: RevenueScope,
    scopeId: string,
    account: string,
  ) => Promise<void>;
}>) {
  const [draft, setDraft] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const errors = useApiFormError();
  const effective = effectiveRevenue(page, scope, scopeId);
  const own = ownRevenueAccount(page, scope, scopeId);
  const fieldId = `revenue-${scope}-${scopeId}`;

  const submit = async (event: SubmitLike) => {
    event.preventDefault();
    if (draft === null) return;
    const invalid = revenueAccountError(draft);
    if (invalid) {
      errors.invalid("Bitte prüfen Sie das Erlöskonto.", { account: invalid });
      return;
    }
    setSaving(true);
    try {
      await onSave(scope, scopeId, draft.trim());
      errors.clear();
      setDraft(null);
    } catch (err) {
      logger.error("traeger_revenue_account_save_failed", {
        error: errorText(err),
      });
      await errors.show(err, { object: `das Erlöskonto für ${title}` });
    } finally {
      setSaving(false);
    }
  };

  if (draft !== null) {
    return (
      <li className="py-3 first:pt-0 last:pb-0 sm:col-span-2">
        <form onSubmit={(event) => void submit(event)} noValidate>
          <InsetPanel padding="md" className="flex flex-col gap-3">
            <Input
              id={fieldId}
              name="account"
              label={`Erlöskonto für ${title}`}
              hint="Leer lassen: Es gilt das Konto darüber."
              inputMode="numeric"
              autoComplete="off"
              controlSize="compact"
              className="font-mono sm:max-w-48"
              value={draft}
              error={errors.fieldError("account")}
              onChange={(event) => setDraft(event.target.value)}
            />
            <FormErrorAlert message={errors.error} />
            <EditActions
              onCancel={() => {
                errors.clear();
                setDraft(null);
              }}
              saving={saving}
            />
          </InsetPanel>
        </form>
      </li>
    );
  }
  return (
    <li className={ROW}>
      <div className="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-center sm:gap-6">
        <div className="min-w-0 sm:w-64">
          <p className="text-sm font-semibold text-gray-900">{title}</p>
          <p className="mt-0.5 text-sm text-gray-600">{hint}</p>
        </div>
        <AccountValue account={effective.account} source={effective.source} />
      </div>
      <div className={ACTIONS}>
        <Button
          type="button"
          variant="surface"
          size="compact"
          aria-label={`Erlöskonto für ${title} ändern`}
          tooltip={`Öffnet das Erlöskonto für ${title} zum Bearbeiten.`}
          onClick={() => setDraft(own)}
        >
          Ändern
        </Button>
      </div>
    </li>
  );
}

/**
 * The revenue accounts: the standard of the DATEV values, one per Leistung
 * and one per Angebot of the monthly contributions.
 */
export function RevenueAccountsCard({
  page,
  onSave,
}: Readonly<{
  page: BookingAccounts;
  onSave: (
    scope: RevenueScope,
    scopeId: string,
    account: string,
  ) => Promise<void>;
}>) {
  const schoolName = (id: string) =>
    page.schools.find((school) => school.id === id)?.name ?? "Schule";
  return (
    <SectionCard
      title="Erlöskonten"
      icon={Coins}
      description="Auf diese Sachkonten bucht moto die Beiträge. Hat eine Leistung oder ein Angebot kein eigenes Konto, gilt das Konto darüber."
    >
      <ResponsiveActionList>
        <li className={ROW}>
          <div className="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-center sm:gap-6">
            <div className="min-w-0 sm:w-64">
              <p className="text-sm font-semibold text-gray-900">
                Standard-Erlöskonto
              </p>
              <p className="mt-0.5 text-sm text-gray-600">
                Gilt für alles ohne eigenes Konto
              </p>
            </div>
            <AccountValue
              account={page.standardAccount}
              source={page.standardAccount ? "own" : "missing"}
            />
          </div>
          <div className={ACTIONS}>
            <ButtonLink
              href={traegerPath(DATEV_SETTINGS_PATH)}
              variant="surface"
              size="compact"
              aria-label="Standard-Erlöskonto in den DATEV-Werten ändern"
              tooltip="Öffnet die DATEV-Werte in den Einstellungen."
            >
              DATEV-Werte
            </ButtonLink>
          </div>
        </li>
        {REVENUE_SERVICES.map((service) => (
          <RevenueRow
            key={service.scope}
            title={service.label}
            hint={service.hint}
            scope={service.scope}
            scopeId="0"
            page={page}
            onSave={onSave}
          />
        ))}
      </ResponsiveActionList>
      {page.offerings.length > 0 ? (
        <div className="mt-5 flex flex-col gap-3">
          <ListGroupHeader
            title="Angebote der Monatsbeiträge"
            meta="Eigenes Konto nur, wenn Ihr Steuerbüro es so will"
          />
          <ResponsiveActionList>
            {page.offerings.map((offering) => (
              <RevenueRow
                key={offering.offeringId}
                title={offering.moduleNames.join(", ") || "Angebot"}
                hint={schoolName(offering.schoolId)}
                scope="offering"
                scopeId={offering.offeringId}
                page={page}
                onSave={onSave}
              />
            ))}
          </ResponsiveActionList>
        </div>
      ) : null}
    </SectionCard>
  );
}

/** What holds now and what comes next, in words. */
function CostCenterState({
  timeline,
  today,
  required,
}: Readonly<{
  timeline: readonly CostCenter[];
  today: string;
  required: boolean;
}>) {
  const current = costCenterOn(timeline, today);
  const next = nextCostCenter(timeline, today);
  return (
    <span className="flex max-w-full min-w-0 flex-col gap-1">
      {current?.code ? (
        <span className="flex max-w-full min-w-0 flex-wrap items-center gap-2">
          <span className="max-w-full min-w-0 font-mono text-sm font-semibold break-all text-gray-900">
            {current.code}
          </span>
          <span className="text-sm text-gray-600">
            seit {formatDate(current.validFrom)}
          </span>
        </span>
      ) : required ? (
        <StatusBadge tone="warning" label="Keine Kostenstelle" />
      ) : (
        <span className="text-sm text-gray-600">Keine Kostenstelle</span>
      )}
      {next ? (
        <span className="text-sm text-gray-600">
          Ab {formatDate(next.validFrom)}:{" "}
          {next.code ? (
            <span className="font-mono break-all text-gray-900">
              {next.code}
            </span>
          ) : (
            "keine Kostenstelle mehr"
          )}
        </span>
      ) : null}
    </span>
  );
}

/** The form for a new cost center of a scope, inside its row. */
function CostCenterForm({
  title,
  today,
  onCancel,
  onAdd,
}: Readonly<{
  title: string;
  today: string;
  onCancel: () => void;
  onAdd: (validFrom: string, code: string) => Promise<void>;
}>) {
  const [validFrom, setValidFrom] = useState(today);
  const [code, setCode] = useState("");
  const [saving, setSaving] = useState(false);
  const errors = useApiFormError();

  const submit = async (event: SubmitLike) => {
    event.preventDefault();
    const invalid = costCenterError(code);
    if (!validFrom || invalid) {
      errors.invalid("Bitte prüfen Sie die markierten Felder.", {
        ...(validFrom ? {} : { valid_from: "Bitte wählen Sie einen Tag." }),
        ...(invalid ? { code: invalid } : {}),
      });
      return;
    }
    setSaving(true);
    try {
      await onAdd(validFrom, code.trim());
    } catch (err) {
      logger.error("traeger_cost_center_add_failed", { error: errorText(err) });
      await errors.show(err, { object: `die Kostenstelle für ${title}` });
      setSaving(false);
    }
  };

  return (
    <form onSubmit={(event) => void submit(event)} noValidate>
      <InsetPanel padding="md" className="flex flex-col gap-3">
        <div className="grid gap-3 sm:grid-cols-[minmax(0,12rem)_minmax(0,16rem)]">
          <ISODatePicker
            label="Gilt ab"
            value={validFrom}
            onChange={setValidFrom}
            error={errors.fieldError("valid_from")}
          />
          <Input
            name="code"
            label="Kostenstelle"
            hint="Leer lassen: Ab diesem Tag gibt es keine."
            autoComplete="off"
            controlSize="compact"
            className="font-mono"
            value={code}
            error={errors.fieldError("code")}
            onChange={(event) => setCode(event.target.value)}
          />
        </div>
        <FormErrorAlert message={errors.error} />
        <EditActions onCancel={onCancel} saving={saving} />
      </InsetPanel>
    </form>
  );
}

/** One scope with its cost centers, the form to add one and the history. */
function CostCenterRow({
  title,
  hint,
  scope,
  scopeId,
  page,
  today,
  required,
  onAdd,
  onDelete,
}: Readonly<{
  title: string;
  hint: string;
  scope: CostCenterScope;
  scopeId: string;
  page: BookingAccounts;
  today: string;
  required: boolean;
  onAdd: (
    scope: CostCenterScope,
    scopeId: string,
    validFrom: string,
    code: string,
  ) => Promise<void>;
  onDelete: (center: CostCenter, title: string) => void;
}>) {
  const [adding, setAdding] = useState(false);
  const [history, setHistory] = useState(false);
  const timeline = costCenterTimeline(page.costCenters, scope, scopeId);
  const label = scope === "offering" ? "KOST2" : "KOST1";
  return (
    <li className={ROW}>
      <div className="flex min-w-0 flex-col gap-1 sm:flex-row sm:items-center sm:gap-6">
        <div className="min-w-0 sm:w-64">
          <p className="text-sm font-semibold text-gray-900">{title}</p>
          <p className="mt-0.5 text-sm text-gray-600">{hint}</p>
        </div>
        <div className="flex max-w-full min-w-0 flex-col items-start gap-1">
          <CostCenterState
            timeline={timeline}
            today={today}
            required={required}
          />
          {timeline.length > 0 ? (
            <Button
              type="button"
              variant="link"
              size="compact"
              aria-expanded={history}
              aria-label={`Verlauf der Kostenstellen für ${title}`}
              tooltip={
                history
                  ? "Blendet die früheren und geplanten Kostenstellen wieder aus."
                  : "Zeigt alle früheren und geplanten Kostenstellen mit Startdatum."
              }
              onClick={() => setHistory((shown) => !shown)}
            >
              {history
                ? "Verlauf ausblenden"
                : `Verlauf ansehen (${timeline.length})`}
            </Button>
          ) : null}
        </div>
      </div>
      <div className={ACTIONS}>
        <Button
          type="button"
          variant="surface"
          size="compact"
          aria-label={`Neue ${label} für ${title}`}
          disabled={adding}
          tooltip="Öffnet ein Formular für eine Kostenstelle ab einem gewählten Tag."
          onClick={() => setAdding(true)}
        >
          Neue Kostenstelle
        </Button>
      </div>
      {adding ? (
        <div className="mt-3 sm:col-span-2">
          <CostCenterForm
            title={title}
            today={today}
            onCancel={() => setAdding(false)}
            onAdd={async (validFrom, code) => {
              await onAdd(scope, scopeId, validFrom, code);
              setAdding(false);
            }}
          />
        </div>
      ) : null}
      {history && timeline.length > 0 ? (
        <InsetPanel padding="sm" className="mt-3 min-w-0 sm:col-span-2">
          <ol
            className="divide-y divide-gray-100"
            aria-label={`Verlauf für ${title}`}
          >
            {[...timeline].reverse().map((center) => (
              <li
                key={center.id}
                className="flex items-center justify-between gap-3 py-2"
              >
                <span className="flex min-w-0 flex-wrap items-baseline gap-x-3 gap-y-0.5 text-sm">
                  <span className="text-gray-600 tabular-nums">
                    ab {formatDate(center.validFrom)}
                  </span>
                  {center.code ? (
                    <span className="max-w-full min-w-0 font-mono break-all text-gray-900">
                      {center.code}
                    </span>
                  ) : (
                    <span className="text-gray-600">keine Kostenstelle</span>
                  )}
                  {center.validFrom > today ? (
                    <StatusBadge tone="blue" size="xs" label="geplant" />
                  ) : null}
                </span>
                <OverflowMenu
                  ariaLabel={`Aktionen für die Kostenstelle ab ${formatDate(center.validFrom)} von ${title}`}
                  triggerSize="sm"
                  items={[
                    {
                      label: "Löschen",
                      description: "Ausgestellte Rechnungen behalten sie.",
                      destructive: true,
                      onClick: () => onDelete(center, title),
                    },
                  ]}
                />
              </li>
            ))}
          </ol>
        </InsetPanel>
      ) : null}
    </li>
  );
}

type CostCenterScopeRow = {
  readonly key: string;
  readonly title: string;
  readonly hint: string;
  readonly scope: CostCenterScope;
  readonly scopeId: string;
};

/**
 * The cost centers: KOST1 of each school and of the holiday care, or KOST2
 * of each Angebot. A cost center holds from its day until the next one.
 */
export function CostCentersCard({
  kind,
  page,
  today,
  onAdd,
  onDeleted,
}: Readonly<{
  kind: "kost1" | "kost2";
  page: BookingAccounts;
  today: string;
  onAdd: (
    scope: CostCenterScope,
    scopeId: string,
    validFrom: string,
    code: string,
  ) => Promise<void>;
  onDeleted: (id: string) => Promise<void>;
}>) {
  const [deleting, setDeleting] = useState<{
    center: CostCenter;
    title: string;
  } | null>(null);
  const [busy, setBusy] = useState(false);
  const deleteErrors = useApiFormError();
  const { success } = useToast();
  const schoolName = (id: string) =>
    page.schools.find((school) => school.id === id)?.name ?? "Schule";
  const rows: CostCenterScopeRow[] =
    kind === "kost1"
      ? [
          ...page.schools.map((school) => ({
            key: `school-${school.id}`,
            title: school.name,
            hint: "Monatsbeiträge, Mensa und Ferienwochen an dieser Schule",
            scope: "school" as const,
            scopeId: school.id,
          })),
          {
            key: "holiday",
            title: "Ferienbetreuung",
            hint: "Ersatz für Ferienwochen einer Schule ohne Kostenstelle",
            scope: "holiday",
            scopeId: "0",
          },
        ]
      : page.offerings.map((offering) => ({
          key: `offering-${offering.offeringId}`,
          title: offering.moduleNames.join(", ") || "Angebot",
          hint: schoolName(offering.schoolId),
          scope: "offering" as const,
          scopeId: offering.offeringId,
        }));
  const required = kind === "kost1" && page.costCenterRequired;

  const confirmDelete = async () => {
    if (!deleting) return;
    setBusy(true);
    try {
      await onDeleted(deleting.center.id);
      success("Die Kostenstelle ist gelöscht.");
      deleteErrors.clear();
      setDeleting(null);
    } catch (err) {
      logger.error("traeger_cost_center_delete_failed", {
        error: errorText(err),
      });
      await deleteErrors.show(err, { object: "die Kostenstelle" });
    } finally {
      setBusy(false);
    }
  };

  return (
    <SectionCard
      title={
        kind === "kost1"
          ? "Kostenstellen (KOST1)"
          : "Zweite Kostenstelle (KOST2)"
      }
      icon={kind === "kost1" ? Landmark : Layers}
      titleBadge={
        kind === "kost1" ? (
          required ? (
            <StatusBadge tone="orange" label="Pflicht" />
          ) : (
            <StatusBadge tone="outline" label="Freiwillig" />
          )
        ) : (
          <StatusBadge tone="outline" label="Freiwillig" />
        )
      }
      description={
        kind === "kost1"
          ? "Jede Schule und die Ferienbetreuung können eine Kostenstelle haben. Sie gilt ab ihrem Tag bis zur nächsten."
          : "Nur, wenn Ihr Steuerbüro die Angebote getrennt auswerten will."
      }
    >
      {rows.length === 0 ? (
        <p className="text-sm text-gray-600">
          Angebote stehen hier, sobald sie einen Preis haben.
        </p>
      ) : (
        <ResponsiveActionList>
          {rows.map((row) => (
            <CostCenterRow
              key={row.key}
              title={row.title}
              hint={row.hint}
              scope={row.scope}
              scopeId={row.scopeId}
              page={page}
              today={today}
              required={required}
              onAdd={onAdd}
              onDelete={(center, title) => setDeleting({ center, title })}
            />
          ))}
        </ResponsiveActionList>
      )}
      <ConfirmDeleteModal
        isOpen={deleting !== null}
        title="Kostenstelle löschen?"
        description={
          deleting
            ? `${deleting.center.code || "„Keine Kostenstelle“"} ab ${formatDate(deleting.center.validFrom)} für ${deleting.title}. Schon ausgestellte Rechnungen behalten ihre Kostenstelle.`
            : ""
        }
        gate={{ mode: "twoStep", firstStepLabel: "Löschen" }}
        onConfirm={confirmDelete}
        onClose={() => {
          deleteErrors.clear();
          setDeleting(null);
        }}
        loading={busy}
        error={deleteErrors.error}
      />
    </SectionCard>
  );
}

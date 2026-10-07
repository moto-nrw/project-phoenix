"use client";

import {
  Suspense,
  use,
  useCallback,
  useEffect,
  useMemo,
  useState,
} from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { createLogger } from "~/lib/logger";
import { formatDate } from "~/lib/date-helpers";
import type { SettingsSchema } from "~/lib/settings-api";
import {
  fetchOperatorSettingsSchema,
  setOperatorSettingValue,
  resetOperatorSettingValue,
  revealOperatorSettingValue,
} from "~/lib/operator/operator-settings-api";
import { operatorProvisioningService } from "~/lib/operator/provisioning-api";
import { resolveOperatorBackHref } from "~/lib/operator/back-href";
import { SettingsCategory } from "~/components/settings/settings-category";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { useApiLoadError } from "~/contexts/ToastContext";
import { ConfirmationModal } from "~/components/ui/modal";
import { Skeleton } from "~/components/ui/skeleton";
import { useBookingAuthorityImpact } from "./use-booking-authority-impact";
import {
  ConceptPageHeader,
  ConceptSectionHeader,
} from "~/components/ui/concept-section-header";
import type { MotoConceptKey } from "~/lib/moto-concepts";

const logger = createLogger({ component: "OperatorSchoolSettingsPage" });

// Tab labels (match the tenant settings UI mapping)
const TAB_LABELS: Record<string, string> = {
  operations: "Betrieb",
  reminders: "Erinnerungen",
  gdpr: "Datenschutz",
  security: "Sicherheit",
  devices: "Geräte",
  system: "System",
  general: "Allgemein",
  startseite: "Startseite für alle",
};

// Konzept je Einstellungs-Tab, fuer die Kachel im Sektions-Header. Faellt auf
// das neutrale "settings"-Konzept zurueck, wenn kein spezifischeres passt.
const TAB_CONCEPTS: Record<string, MotoConceptKey> = {
  operations: "settings",
  reminders: "notifications",
  gdpr: "permissions",
  security: "permissions",
  devices: "devices",
  system: "settings",
  general: "settings",
};

interface PageProps {
  readonly params: Promise<{ id: string }>;
}

export default function OperatorSchoolSettingsPage({ params }: PageProps) {
  return (
    <Suspense fallback={null}>
      <OperatorSchoolSettingsPageContent params={params} />
    </Suspense>
  );
}

function OperatorSchoolSettingsPageContent({ params }: PageProps) {
  const { id: schoolId } = use(params);
  const { status: sessionStatus } = useSession();
  const searchParams = useSearchParams();
  const backHref = useMemo(
    () =>
      resolveOperatorBackHref(searchParams.get("back"), "/operator/schools"),
    [searchParams],
  );
  const backLabel =
    backHref === "/operator/schools" ? "Zu den Schulen" : "Zurück zur Schule";

  const [schema, setSchema] = useState<SettingsSchema | null>(null);
  const [schoolName, setSchoolName] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const schemaLoad = useApiLoadError();
  const { show: showSchemaError, clear: clearSchemaError } = schemaLoad;

  const loadSchema = useCallback(async () => {
    setLoading(true);
    clearSchemaError();
    try {
      const data = await fetchOperatorSettingsSchema(schoolId);
      setSchema(data);
    } catch (err) {
      logger.error("operator_settings_load_failed", {
        school_id: schoolId,
        error: err instanceof Error ? err.message : String(err),
      });
      void showSchemaError(err, {
        object: "die Liste der Einstellungen",
        retry: () => void loadSchema(),
      });
    } finally {
      setLoading(false);
    }
  }, [schoolId, showSchemaError, clearSchemaError]);

  // The heading names the school; a failed lookup is shown in place (#2519).
  const schoolNameLoad = useApiLoadError();
  const { show: showSchoolNameError, clear: clearSchoolNameError } =
    schoolNameLoad;
  const loadSchoolName = useCallback(async () => {
    clearSchoolNameError();
    try {
      const schools = await operatorProvisioningService.listSchools();
      const school = schools.find((s) => s.id === schoolId);
      if (school) setSchoolName(school.name);
    } catch (err) {
      logger.warn("operator_school_name_lookup_failed", {
        school_id: schoolId,
        error: err instanceof Error ? err.message : String(err),
      });
      void showSchoolNameError(err, {
        object: "die Bezeichnung der Schule",
        retry: () => void loadSchoolName(),
      });
    }
  }, [schoolId, showSchoolNameError, clearSchoolNameError]);

  // After a save the schema is read again for the server state and DependsOn.
  // A failed refresh keeps the shown values and reports the stale list.
  const refreshSchema = useCallback(async () => {
    try {
      const fresh = await fetchOperatorSettingsSchema(schoolId);
      clearSchemaError();
      setSchema(fresh);
    } catch (err) {
      logger.warn("operator_settings_refresh_failed", {
        school_id: schoolId,
        error: err instanceof Error ? err.message : String(err),
      });
      void showSchemaError(err, {
        object: "die Liste der Einstellungen",
        retry: () => void loadSchema(),
      });
    }
  }, [schoolId, showSchemaError, clearSchemaError, loadSchema]);

  useEffect(() => {
    if (sessionStatus !== "authenticated") return;
    void loadSchema();
    void loadSchoolName();
  }, [sessionStatus, loadSchema, loadSchoolName]);

  // Save and reset throw the ApiError of a failed request; the field shows
  // it on the shared error path (#2519).
  const handleSave = useCallback(
    async (key: string, value: unknown): Promise<void> => {
      await setOperatorSettingValue(schoolId, key, value);
      logger.info("operator_setting_saved", { school_id: schoolId, key });
      void refreshSchema();
    },
    [schoolId, refreshSchema],
  );

  const handleReset = useCallback(
    async (key: string): Promise<void> => {
      await resetOperatorSettingValue(schoolId, key);
      logger.info("operator_setting_reset", { school_id: schoolId, key });
      void refreshSchema();
    },
    [schoolId, refreshSchema],
  );

  const handleReveal = useCallback(
    (key: string) => revealOperatorSettingValue(schoolId, key),
    [schoolId],
  );

  const bookingAuthority = useBookingAuthorityImpact(schoolId, handleSave);
  const bookingAuthorityBlockers =
    bookingAuthority.state.impact?.blockingChildren ?? [];

  return (
    <div className="mx-auto w-full max-w-4xl min-w-0 px-4 py-6 sm:px-6 lg:px-8">
      <div className="mb-6">
        <Link
          href={backHref}
          className="mb-4 inline-flex items-center gap-1 text-sm text-gray-600 hover:text-gray-900"
        >
          <svg
            className="h-4 w-4"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M15 19l-7-7 7-7"
            />
          </svg>
          {backLabel}
        </Link>
        <ConceptPageHeader
          title={`Einstellungen${schoolName ? ` · ${schoolName}` : ""}`}
          concept="settings"
          subtitle="Sie bearbeiten diese Einstellungen als Operator stellvertretend für die Schule."
        />
      </div>

      <LoadErrorAlert error={schoolNameLoad.error} className="mb-4" />
      <LoadErrorAlert error={schemaLoad.error} className="mb-4" />

      {loading && (
        <div className="space-y-6">
          {Array.from({ length: 2 }).map((_, idx) => (
            <div
              key={idx}
              className="moto-content-surface rounded-2xl border p-6 shadow-sm"
            >
              <Skeleton className="mb-4 h-5 w-32 rounded" />
              <Skeleton className="h-20 w-full rounded" />
            </div>
          ))}
        </div>
      )}

      {!loading && schema && (schema.tabs?.length ?? 0) === 0 && (
        <div className="py-8 text-center text-sm text-gray-500">
          Keine Einstellungen für diese Schule verfügbar.
        </div>
      )}

      {!loading &&
        schema?.tabs?.map((tab) => (
          <section key={tab.key} className="mb-8">
            <ConceptSectionHeader
              title={TAB_LABELS[tab.key] ?? tab.label}
              concept={TAB_CONCEPTS[tab.key] ?? "settings"}
              className="mb-4"
            />
            <div className="space-y-6">
              {tab.categories.map((category) => (
                <SettingsCategory
                  key={category.key}
                  category={category}
                  onSave={handleSave}
                  onReset={handleReset}
                  onBookingAuthorityEnable={bookingAuthority.request}
                  audience="operator"
                  revealFn={handleReveal}
                />
              ))}
            </div>
          </section>
        ))}

      <ConfirmationModal
        isOpen={bookingAuthority.state.isOpen}
        onClose={bookingAuthority.close}
        onConfirm={() => void bookingAuthority.confirm()}
        title="Buchungsmodus aktivieren?"
        confirmText="Buchungsmodus aktivieren"
        cancelText="Abbrechen"
        isConfirmLoading={bookingAuthority.state.isSaving}
        isConfirmDisabled={
          bookingAuthority.state.isLoading ||
          bookingAuthority.state.impact === null ||
          bookingAuthorityBlockers.length > 0 ||
          bookingAuthority.loadError !== null
        }
      >
        <div className="space-y-4 text-sm text-gray-700">
          <p>
            Nach dem Aktivieren bestimmen die Buchungen, an welchen Tagen die
            Kinder betreut werden.
          </p>
          {bookingAuthority.state.isLoading ? (
            <p>Auswirkungen werden geprüft …</p>
          ) : null}
          <LoadErrorAlert error={bookingAuthority.loadError} />
          <FormErrorAlert message={bookingAuthority.saveError} />
          {bookingAuthority.state.impact &&
          bookingAuthorityBlockers.length > 0 ? (
            <div>
              <p className="font-medium text-gray-900">
                Aktivieren nicht möglich: Für {bookingAuthorityBlockers.length}
                {bookingAuthorityBlockers.length === 1
                  ? " Kind ist aktuell keine Betreuung gebucht."
                  : " Kinder sind aktuell keine Betreuung gebucht."}
              </p>
              <ul className="mt-2 list-disc space-y-1 pl-5">
                {bookingAuthorityBlockers.map((child) => (
                  <li key={child.studentId}>
                    {child.firstName} {child.lastName}
                    {child.schoolClass ? " · " + child.schoolClass : ""}
                  </li>
                ))}
              </ul>
              <p className="mt-2">
                Bitte klären Sie diese Buchungen zuerst. Das Aktivieren kann
                nicht übersprungen werden.
              </p>
            </div>
          ) : null}
          {bookingAuthority.state.impact &&
          bookingAuthorityBlockers.length === 0 ? (
            <p className="font-medium text-gray-900">
              Alle aktuell betreuten Kinder haben mindestens einen gebuchten
              Betreuungstag.
            </p>
          ) : null}
          {bookingAuthority.state.impact ? (
            <div>
              <p className="font-medium text-gray-900">
                Geplante Abschlüsse:{" "}
                {bookingAuthority.state.impact.plannedCompletions.length}
              </p>
              {bookingAuthority.state.impact.plannedCompletions.length === 0 ? (
                <p className="mt-1">Es werden keine Abschlüsse geplant.</p>
              ) : (
                <>
                  <p className="mt-1">
                    moto legt diese Abschlüsse sofort an. Die Kinder bleiben bis
                    zum letzten gebuchten Betreuungstag in den Arbeitslisten.
                  </p>
                  <ul className="mt-2 list-disc space-y-1 pl-5">
                    {bookingAuthority.state.impact.plannedCompletions.map(
                      (child) => (
                        <li key={child.studentId}>
                          {child.firstName} {child.lastName}
                          {child.firstBookinglessDay
                            ? " · ab " + formatDate(child.firstBookinglessDay)
                            : ""}
                        </li>
                      ),
                    )}
                  </ul>
                </>
              )}
            </div>
          ) : null}
        </div>
      </ConfirmationModal>
    </div>
  );
}

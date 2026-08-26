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
import { Alert } from "~/components/ui/alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { Skeleton } from "~/components/ui/skeleton";
import { BookingAuthorityImpactDetails } from "./booking-authority-impact-details";
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
  const [error, setError] = useState<string | null>(null);

  const loadSchema = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await fetchOperatorSettingsSchema(schoolId);
      setSchema(data);
    } catch (err) {
      logger.error("operator_settings_load_failed", {
        school_id: schoolId,
        error: err instanceof Error ? err.message : String(err),
      });
      setError("Einstellungen konnten nicht geladen werden.");
    } finally {
      setLoading(false);
    }
  }, [schoolId]);

  useEffect(() => {
    if (sessionStatus !== "authenticated") return;
    void loadSchema();
    void operatorProvisioningService
      .listSchools()
      .then((schools) => {
        const school = schools.find((s) => s.id === schoolId);
        if (school) setSchoolName(school.name);
      })
      .catch((err: unknown) => {
        logger.warn("operator_school_name_lookup_failed", {
          school_id: schoolId,
          error: err instanceof Error ? err.message : String(err),
        });
      });
  }, [sessionStatus, schoolId, loadSchema]);

  const handleSave = useCallback(
    async (key: string, value: unknown): Promise<string | null> => {
      const errorMsg = await setOperatorSettingValue(schoolId, key, value);
      if (!errorMsg) {
        logger.info("operator_setting_saved", { school_id: schoolId, key });
        // Re-fetch to reflect the server state and re-evaluate DependsOn
        void fetchOperatorSettingsSchema(schoolId)
          .then((fresh) => {
            if (fresh) setSchema(fresh);
          })
          .catch(() => {
            // Ignore - the optimistic save already succeeded
          });
      }
      return errorMsg;
    },
    [schoolId],
  );

  const handleReset = useCallback(
    async (key: string): Promise<string | null> => {
      const errorMsg = await resetOperatorSettingValue(schoolId, key);
      if (!errorMsg) {
        logger.info("operator_setting_reset", { school_id: schoolId, key });
        void fetchOperatorSettingsSchema(schoolId)
          .then((fresh) => {
            if (fresh) setSchema(fresh);
          })
          .catch(() => {
            // Ignore
          });
      }
      return errorMsg;
    },
    [schoolId],
  );

  const handleReveal = useCallback(
    (key: string) => revealOperatorSettingValue(schoolId, key),
    [schoolId],
  );

  const bookingAuthority = useBookingAuthorityImpact(schoolId, handleSave);
  const bookingAuthorityBlockers =
    bookingAuthority.state.impact?.blockingChildren ?? [];

  return (
    <div className="mx-auto max-w-4xl px-4 py-6 sm:px-6 lg:px-8">
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

      {error && (
        <div className="mb-4">
          <Alert type="error" message={error} />
        </div>
      )}

      {loading && (
        <div className="space-y-6">
          {Array.from({ length: 2 }).map((_, idx) => (
            <div
              key={idx}
              className="rounded-2xl border border-gray-100 bg-white/50 p-6"
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
          bookingAuthority.state.error !== null
        }
      >
        <BookingAuthorityImpactDetails
          impact={bookingAuthority.state.impact}
          isLoading={bookingAuthority.state.isLoading}
          error={bookingAuthority.state.error}
        />
      </ConfirmationModal>
    </div>
  );
}

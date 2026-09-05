"use client";

import { useCallback, useEffect, useState } from "react";
import { FormModal } from "~/components/ui/form-modal";
import { Alert } from "~/components/ui/alert";
import { Input } from "~/components/ui/input";
import {
  DataField,
  DataGrid,
  DetailIcons,
  InfoSection,
} from "~/components/ui/detail-modal-components";
import { useToast } from "~/contexts/ToastContext";
import { MotoDuotoneIcon } from "~/components/ui/moto-duotone-icon";
import {
  caregiverCapabilityService,
  CaregiverCapabilityApiError,
  type CaregiverCapabilityState,
} from "~/lib/caregiver-capability-api";
import { CaregiverBlockerResolutionPanel } from "~/components/teachers/caregiver-blocker-resolution-panel";
import { createLogger } from "~/lib/logger";
import { MOTO_CONCEPTS } from "~/lib/moto-concepts";

const logger = createLogger({ component: "CaregiverCapabilityModal" });

type CaregiverCapabilityScope = "tenant" | "operator";

interface CaregiverCapabilityModalProps {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly scope: CaregiverCapabilityScope;
  readonly accountId: string;
  readonly accountLabel: string;
  readonly schoolId?: string;
  readonly schoolName?: string;
  readonly onUpdated?: (
    state: CaregiverCapabilityState,
  ) => void | Promise<void>;
}

function getRoleSummary(state: CaregiverCapabilityState): string {
  if (state.isActiveCaregiver && state.hasAdminRole) {
    return "Verwaltung + Betreuung";
  }
  if (state.isActiveCaregiver) {
    return "Betreuung aktiv";
  }
  if (state.hasAdminRole && !state.hasUserRole) {
    return "Nur Verwaltung";
  }
  if (state.hasUserRole && !state.hasCaregiverProfile) {
    return "Betreuung wird eingerichtet";
  }
  if (state.hasCaregiverProfile && !state.hasUserRole) {
    return "Betreuung deaktiviert";
  }
  return "Keine Betreuung";
}

export function CaregiverCapabilityModal({
  isOpen,
  onClose,
  scope,
  accountId,
  accountLabel,
  schoolId,
  schoolName,
  onUpdated,
}: CaregiverCapabilityModalProps) {
  const { success: toastSuccess, error: toastError } = useToast();
  const [state, setState] = useState<CaregiverCapabilityState | null>(null);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [position, setPosition] = useState("");
  const [errorMessage, setErrorMessage] = useState("");
  // Zwei Schritte in EINEM Dialog: Übersicht und das Auflösen der offenen
  // Zuordnungen. Ein zweiter Dialog darüber ist portalweit verboten.
  const [step, setStep] = useState<"overview" | "resolve">("overview");

  const needsSchoolId = scope === "operator";
  const operatorSchoolId = scope === "operator" ? schoolId : undefined;

  const loadState = useCallback(async () => {
    if (!accountId || (needsSchoolId && !operatorSchoolId)) {
      return;
    }

    try {
      setLoading(true);
      setErrorMessage("");
      setPosition("");

      let nextState: CaregiverCapabilityState;
      if (scope === "operator") {
        const resolvedSchoolId = operatorSchoolId;
        if (!resolvedSchoolId) {
          return;
        }
        nextState =
          await caregiverCapabilityService.getOperatorAccountCapability(
            resolvedSchoolId,
            accountId,
          );
      } else {
        nextState =
          await caregiverCapabilityService.getTenantAccountCapability(
            accountId,
          );
      }

      setState(nextState);
      setFirstName(nextState.firstName);
      setLastName(nextState.lastName);
    } catch (error) {
      logger.error("failed to load caregiver capability state", {
        error: error instanceof Error ? error.message : String(error),
        accountId,
        schoolId: operatorSchoolId,
        scope,
      });
      setErrorMessage("Die Betreuerfähigkeit konnte nicht geladen werden.");
    } finally {
      setLoading(false);
    }
  }, [accountId, needsSchoolId, operatorSchoolId, scope]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    setStep("overview");
    void loadState();
  }, [isOpen, loadState]);

  const canDisable = Boolean(state?.isActiveCaregiver);
  const needsNames = state != null && !state.hasPerson;

  async function handleEnable() {
    if (!accountId || (needsSchoolId && !operatorSchoolId)) {
      return;
    }

    const trimmedFirstName = firstName.trim();
    const trimmedLastName = lastName.trim();
    const trimmedPosition = position.trim();

    if (needsNames && (!trimmedFirstName || !trimmedLastName)) {
      setErrorMessage(
        "Vorname und Nachname werden benötigt, da noch kein Personalprofil existiert.",
      );
      return;
    }

    try {
      setSaving(true);
      setErrorMessage("");

      let nextState: CaregiverCapabilityState;
      if (scope === "operator") {
        const resolvedSchoolId = operatorSchoolId;
        if (!resolvedSchoolId) {
          return;
        }
        nextState =
          await caregiverCapabilityService.enableOperatorAccountCapability(
            resolvedSchoolId,
            accountId,
            {
              first_name: trimmedFirstName || undefined,
              last_name: trimmedLastName || undefined,
              position: trimmedPosition || undefined,
            },
          );
      } else {
        nextState =
          await caregiverCapabilityService.enableTenantAccountCapability(
            accountId,
            {
              first_name: trimmedFirstName || undefined,
              last_name: trimmedLastName || undefined,
              position: trimmedPosition || undefined,
            },
          );
      }

      setState(nextState);
      toastSuccess("Betreuung wurde erfolgreich aktiviert.");
      await onUpdated?.(nextState);
    } catch (error) {
      logger.error("failed to enable caregiver capability", {
        error: error instanceof Error ? error.message : String(error),
        accountId,
        schoolId: operatorSchoolId,
        scope,
      });
      if (error instanceof CaregiverCapabilityApiError) {
        setErrorMessage(error.message);
      } else {
        setErrorMessage("Die Betreuung konnte nicht aktiviert werden.");
      }
    } finally {
      setSaving(false);
    }
  }

  async function handleDisable() {
    if (!accountId || (needsSchoolId && !operatorSchoolId)) {
      return;
    }

    try {
      setSaving(true);
      setErrorMessage("");

      let nextState: CaregiverCapabilityState;
      if (scope === "operator") {
        const resolvedSchoolId = operatorSchoolId;
        if (!resolvedSchoolId) {
          return;
        }
        nextState =
          await caregiverCapabilityService.disableOperatorAccountCapability(
            resolvedSchoolId,
            accountId,
          );
      } else {
        nextState =
          await caregiverCapabilityService.disableTenantAccountCapability(
            accountId,
          );
      }

      setState(nextState);
      toastSuccess("Betreuung wurde deaktiviert.");
      await onUpdated?.(nextState);
    } catch (error) {
      logger.error("failed to disable caregiver capability", {
        error: error instanceof Error ? error.message : String(error),
        accountId,
        schoolId: operatorSchoolId,
        scope,
      });
      if (error instanceof CaregiverCapabilityApiError) {
        setErrorMessage(error.message);
        if (error.blockers.length > 0) {
          toastError(
            "Betreuung kann nicht deaktiviert werden. Bitte zuerst die offenen Zuordnungen entfernen.",
          );
        }
      } else {
        setErrorMessage("Die Betreuung konnte nicht deaktiviert werden.");
      }
    } finally {
      setSaving(false);
    }
  }

  const showEnableButton = !state?.isActiveCaregiver || needsNames;

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={
        step === "resolve"
          ? `Zuordnungen auflösen: ${accountLabel}`
          : `Betreuung verwalten: ${accountLabel}`
      }
      size="lg"
      footer={
        step === "resolve" ? (
          <button
            type="button"
            onClick={() => {
              setStep("overview");
              void loadState();
            }}
            className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
          >
            Zurück zur Übersicht
          </button>
        ) : (
          <>
            <button
              type="button"
              onClick={onClose}
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
            >
              Schließen
            </button>
            {canDisable ? (
              <button
                type="button"
                onClick={() => void handleDisable()}
                disabled={saving || loading || state?.disableBlocked}
                className="border-moto-red/20 text-moto-red-strong hover:bg-moto-red-soft rounded-lg border px-4 py-2 text-sm font-medium transition-colors disabled:cursor-not-allowed disabled:opacity-50"
              >
                Betreuung deaktivieren
              </button>
            ) : null}
            {showEnableButton ? (
              <button
                type="button"
                onClick={() => void handleEnable()}
                disabled={saving || loading}
                className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {saving ? "Wird gespeichert..." : "Betreuung aktivieren"}
              </button>
            ) : null}
          </>
        )
      }
    >
      {step === "resolve" && state ? (
        <CaregiverBlockerResolutionPanel active state={state} />
      ) : loading ? (
        <div className="py-8 text-sm text-gray-500">Wird geladen...</div>
      ) : state ? (
        <div className="space-y-4">
          {/* Current role overview */}
          <InfoSection
            title="Aktuelle Rolle"
            icon={
              <MotoDuotoneIcon
                icon={MOTO_CONCEPTS.roles.icon}
                tone={MOTO_CONCEPTS.roles.tone}
                size={18}
              />
            }
            accentColor={state.isActiveCaregiver ? "green" : "gray"}
          >
            <DataGrid>
              <DataField label="Status">{getRoleSummary(state)}</DataField>
              <DataField label="E-Mail">
                {state.email || "Nicht verknüpft"}
              </DataField>
              {scope === "operator" && schoolName ? (
                <DataField label="Schule">{schoolName}</DataField>
              ) : null}
            </DataGrid>
          </InfoSection>

          {/* Name fields — only when no person profile exists yet */}
          {needsNames ? (
            <InfoSection
              title="Personaldaten anlegen"
              icon={
                <MotoDuotoneIcon
                  icon={MOTO_CONCEPTS.staff.icon}
                  tone={MOTO_CONCEPTS.staff.tone}
                  size={18}
                />
              }
              accentColor="orange"
            >
              <p className="mb-3 text-xs text-gray-600">
                Für dieses Konto existiert noch kein Personalprofil. Bitte Name
                und optional eine pädagogische Rolle angeben.
              </p>
              <div className="grid gap-3 md:grid-cols-2">
                <Input
                  label="Vorname"
                  name="firstName"
                  value={firstName}
                  onChange={(event) => setFirstName(event.target.value)}
                  placeholder="Vorname"
                />
                <Input
                  label="Nachname"
                  name="lastName"
                  value={lastName}
                  onChange={(event) => setLastName(event.target.value)}
                  placeholder="Nachname"
                />
              </div>
              <div className="mt-3">
                <Input
                  label="Pädagogische Rolle (optional)"
                  name="position"
                  value={position}
                  onChange={(event) => setPosition(event.target.value)}
                  placeholder="z.B. Gruppenleitung, Ergänzungskraft"
                />
              </div>
            </InfoSection>
          ) : null}

          {/* Blocker warnings */}
          {state.isActiveCaregiver && state.disableBlockers.length > 0 ? (
            <InfoSection
              title="Offene Zuordnungen"
              icon={
                <MotoDuotoneIcon
                  icon={MOTO_CONCEPTS.groups.icon}
                  tone={MOTO_CONCEPTS.groups.tone}
                  size={18}
                />
              }
              accentColor="amber"
            >
              <p className="mb-2 text-xs text-gray-600">
                Die Betreuung kann erst deaktiviert werden, wenn folgende
                Zuordnungen entfernt oder an andere Betreuungskräfte übertragen
                wurden:
              </p>
              <ul className="space-y-1.5">
                {state.disableBlockers.map((blocker) => (
                  <li
                    key={blocker}
                    className="text-moto-amber-strong flex items-start gap-2 text-sm"
                  >
                    <span className="text-moto-amber mt-1 h-3 w-3 flex-shrink-0">
                      {DetailIcons.x}
                    </span>
                    {blocker}
                  </li>
                ))}
              </ul>
              <div className="mt-3">
                <button
                  type="button"
                  onClick={() => setStep("resolve")}
                  className="border-moto-amber/40 bg-moto-amber/20 text-moto-amber-strong hover:bg-moto-amber/30 rounded-lg border px-4 py-2 text-sm font-medium transition-colors"
                >
                  Zuordnungen auflösen
                </button>
              </div>
            </InfoSection>
          ) : null}

          {/* Error display */}
          <Alert type="error" message={errorMessage} />
        </div>
      ) : (
        <Alert
          type="error"
          message="Die Betreuerfähigkeit konnte nicht geladen werden."
        />
      )}
    </FormModal>
  );
}

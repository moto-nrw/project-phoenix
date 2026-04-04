"use client";

import { useCallback, useEffect, useState } from "react";
import { FormModal } from "~/components/ui";
import { useToast } from "~/contexts/ToastContext";
import {
  caregiverCapabilityService,
  CaregiverCapabilityApiError,
  type CaregiverCapabilityState,
} from "~/lib/caregiver-capability-api";
import { createLogger } from "~/lib/logger";

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

function StatusPill({
  active,
  label,
}: {
  readonly active: boolean;
  readonly label: string;
}) {
  return (
    <span
      className={`inline-flex rounded-full px-2.5 py-1 text-xs font-medium ${
        active ? "bg-green-100 text-green-700" : "bg-gray-100 text-gray-500"
      }`}
    >
      {label}
    </span>
  );
}

function getCapabilitySummary(state: CaregiverCapabilityState) {
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
    return "Betreuung unvollständig";
  }
  if (state.hasCaregiverProfile && !state.hasUserRole) {
    return "Betreuungsprofil inaktiv";
  }
  return "Noch keine Betreuung";
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

  const needsSchoolId = scope === "operator";

  const loadState = useCallback(async () => {
    if (!accountId || (needsSchoolId && !schoolId)) {
      return;
    }

    try {
      setLoading(true);
      setErrorMessage("");

      const nextState =
        scope === "operator"
          ? await caregiverCapabilityService.getOperatorAccountCapability(
              schoolId!,
              accountId,
            )
          : await caregiverCapabilityService.getTenantAccountCapability(
              accountId,
            );

      setState(nextState);
      setFirstName(nextState.firstName);
      setLastName(nextState.lastName);
    } catch (error) {
      logger.error("failed to load caregiver capability state", {
        error: error instanceof Error ? error.message : String(error),
        accountId,
        schoolId,
        scope,
      });
      setErrorMessage("Die Betreuerfähigkeit konnte nicht geladen werden.");
    } finally {
      setLoading(false);
    }
  }, [accountId, needsSchoolId, schoolId, scope]);

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    void loadState();
  }, [isOpen, loadState]);

  const canDisable = Boolean(state?.hasUserRole);
  const needsNames = state != null && !state.hasPerson;
  const enableButtonLabel =
    state?.isActiveCaregiver || state?.hasUserRole
      ? "Betreuerprofil vervollständigen"
      : "Betreuung aktivieren";

  async function handleEnable() {
    if (!accountId || (needsSchoolId && !schoolId)) {
      return;
    }

    const trimmedFirstName = firstName.trim();
    const trimmedLastName = lastName.trim();
    const trimmedPosition = position.trim();

    if (needsNames && (!trimmedFirstName || !trimmedLastName)) {
      setErrorMessage(
        "Vorname und Nachname werden benötigt, wenn für das Konto noch kein Personalprofil existiert.",
      );
      return;
    }

    try {
      setSaving(true);
      setErrorMessage("");

      const nextState =
        scope === "operator"
          ? await caregiverCapabilityService.enableOperatorAccountCapability(
              schoolId!,
              accountId,
              {
                first_name: trimmedFirstName || undefined,
                last_name: trimmedLastName || undefined,
                position: trimmedPosition || undefined,
              },
            )
          : await caregiverCapabilityService.enableTenantAccountCapability(
              accountId,
              {
                first_name: trimmedFirstName || undefined,
                last_name: trimmedLastName || undefined,
                position: trimmedPosition || undefined,
              },
            );

      setState(nextState);
      toastSuccess("Betreuerfähigkeit wurde erfolgreich aktualisiert.");
      await onUpdated?.(nextState);
    } catch (error) {
      logger.error("failed to enable caregiver capability", {
        error: error instanceof Error ? error.message : String(error),
        accountId,
        schoolId,
        scope,
      });
      if (error instanceof CaregiverCapabilityApiError) {
        setErrorMessage(error.message);
      } else {
        setErrorMessage("Die Betreuerfähigkeit konnte nicht aktiviert werden.");
      }
    } finally {
      setSaving(false);
    }
  }

  async function handleDisable() {
    if (!accountId || (needsSchoolId && !schoolId)) {
      return;
    }

    try {
      setSaving(true);
      setErrorMessage("");

      const nextState =
        scope === "operator"
          ? await caregiverCapabilityService.disableOperatorAccountCapability(
              schoolId!,
              accountId,
            )
          : await caregiverCapabilityService.disableTenantAccountCapability(
              accountId,
            );

      setState(nextState);
      toastSuccess("Betreuerfähigkeit wurde entfernt.");
      await onUpdated?.(nextState);
    } catch (error) {
      logger.error("failed to disable caregiver capability", {
        error: error instanceof Error ? error.message : String(error),
        accountId,
        schoolId,
        scope,
      });
      if (error instanceof CaregiverCapabilityApiError) {
        setErrorMessage(error.message);
        if (error.blockers.length > 0) {
          toastError(
            "Die Betreuerfähigkeit ist noch an aktive Zuordnungen gebunden.",
          );
        }
      } else {
        setErrorMessage("Die Betreuerfähigkeit konnte nicht entfernt werden.");
      }
    } finally {
      setSaving(false);
    }
  }

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title={`Betreuung verwalten${accountLabel ? ` - ${accountLabel}` : ""}`}
      size="lg"
      footer={
        <>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50"
          >
            Schließen
          </button>
          <button
            type="button"
            onClick={() => void handleDisable()}
            disabled={!canDisable || saving || loading || state?.disableBlocked}
            className="rounded-lg border border-red-200 px-4 py-2 text-sm font-medium text-red-700 transition-colors hover:bg-red-50 disabled:cursor-not-allowed disabled:opacity-50"
          >
            Betreuung entziehen
          </button>
          <button
            type="button"
            onClick={() => void handleEnable()}
            disabled={saving || loading}
            className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-800 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {saving ? "Wird gespeichert..." : enableButtonLabel}
          </button>
        </>
      }
    >
      {loading ? (
        <div className="py-8 text-sm text-gray-500">
          Betreuerfähigkeit wird geladen...
        </div>
      ) : state ? (
        <div className="space-y-5">
          <div className="rounded-2xl border border-gray-100 bg-gray-50/80 p-4">
            <div className="text-sm font-medium text-gray-900">
              {getCapabilitySummary(state)}
            </div>
            <div className="mt-2 flex flex-wrap gap-2">
              <StatusPill active={state.hasAdminRole} label="Verwaltung" />
              <StatusPill active={state.hasUserRole} label="User-Rolle" />
              <StatusPill
                active={state.hasCaregiverProfile}
                label="Person/Staff/Teacher"
              />
            </div>
            {scope === "operator" && schoolName ? (
              <p className="mt-3 text-xs text-gray-500">Schule: {schoolName}</p>
            ) : null}
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <label className="space-y-1">
              <span className="text-sm font-medium text-gray-700">Vorname</span>
              <input
                value={firstName}
                onChange={(event) => setFirstName(event.target.value)}
                placeholder="Vorname"
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none"
              />
            </label>
            <label className="space-y-1">
              <span className="text-sm font-medium text-gray-700">
                Nachname
              </span>
              <input
                value={lastName}
                onChange={(event) => setLastName(event.target.value)}
                placeholder="Nachname"
                className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none"
              />
            </label>
          </div>

          <label className="space-y-1">
            <span className="text-sm font-medium text-gray-700">
              Pädagogische Rolle
            </span>
            <input
              value={position}
              onChange={(event) => setPosition(event.target.value)}
              placeholder="z. B. Gruppenleitung"
              className="w-full rounded-lg border border-gray-200 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none"
            />
          </label>

          <div className="rounded-2xl border border-gray-100 bg-white p-4">
            <div className="text-sm font-medium text-gray-900">
              Aktueller Profilstatus
            </div>
            <ul className="mt-3 space-y-2 text-sm text-gray-600">
              <li>Account: {state.email || "Kein Konto verknüpft"}</li>
              <li>Personprofil: {state.hasPerson ? "vorhanden" : "fehlt"}</li>
              <li>Staff-Profil: {state.hasStaff ? "vorhanden" : "fehlt"}</li>
              <li>
                Teacher-Profil: {state.hasTeacher ? "vorhanden" : "fehlt"}
              </li>
            </ul>
          </div>

          {state.disableBlockers.length > 0 ? (
            <div className="rounded-2xl border border-amber-200 bg-amber-50 p-4">
              <div className="text-sm font-semibold text-amber-900">
                Betreuung kann aktuell nicht entzogen werden
              </div>
              <ul className="mt-2 list-disc space-y-1 pl-5 text-sm text-amber-800">
                {state.disableBlockers.map((blocker) => (
                  <li key={blocker}>{blocker}</li>
                ))}
              </ul>
            </div>
          ) : null}

          {errorMessage ? (
            <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
              {errorMessage}
            </div>
          ) : null}
        </div>
      ) : (
        <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
          Die Betreuerfähigkeit konnte nicht geladen werden.
        </div>
      )}
    </FormModal>
  );
}

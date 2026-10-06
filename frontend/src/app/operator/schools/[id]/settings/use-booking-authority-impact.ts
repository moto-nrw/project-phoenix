"use client";

import { useCallback, useState } from "react";
import type { Dispatch, SetStateAction } from "react";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  fetchBookingAuthorityImpact,
  type BookingAuthorityImpact,
} from "~/lib/operator/operator-settings-api";

const logger = createLogger({ component: "BookingAuthorityImpact" });
const BOOKINGS_AUTHORITATIVE_KEY = "enrollment.bookings_authoritative";

interface ImpactState {
  readonly impact: BookingAuthorityImpact | null;
  readonly isOpen: boolean;
  readonly isLoading: boolean;
  readonly isSaving: boolean;
}

const initialState: ImpactState = {
  impact: null,
  isOpen: false,
  isLoading: false,
  isSaving: false,
};

type SaveSetting = (key: string, value: unknown) => Promise<void>;
type LoadErrors = ReturnType<typeof useApiLoadError>;
type FormErrors = ReturnType<typeof useApiFormError>;

/**
 * The impact check before the booking mode is switched on. A failed check
 * (`loadError`) and a failed activation (`saveError`) stay in the dialog
 * on the shared error path (#2519).
 */
export function useBookingAuthorityImpact(
  schoolId: string,
  saveSetting: SaveSetting,
) {
  const [state, setState] = useState<ImpactState>(initialState);
  const loadErrors = useApiLoadError();
  const saveErrors = useApiFormError();
  const request = useImpactRequest(schoolId, setState, loadErrors, saveErrors);
  const confirm = useImpactConfirm(
    state.impact,
    saveSetting,
    setState,
    saveErrors,
  );
  const close = useCallback(
    () => setState((current) => ({ ...current, isOpen: false })),
    [],
  );
  return {
    state,
    loadError: loadErrors.error,
    saveError: saveErrors.error,
    request,
    confirm,
    close,
  };
}

function useImpactRequest(
  schoolId: string,
  setState: Dispatch<SetStateAction<ImpactState>>,
  loadErrors: LoadErrors,
  saveErrors: FormErrors,
) {
  const { show: showLoadError, clear: clearLoadError } = loadErrors;
  const { clear: clearSaveError } = saveErrors;
  const request = useCallback(async () => {
    clearLoadError();
    clearSaveError();
    setState({ ...initialState, isOpen: true, isLoading: true });
    try {
      const impact = await fetchBookingAuthorityImpact(schoolId);
      setState((current) => ({ ...current, impact }));
    } catch (error) {
      logger.warn("booking_authority_impact_failed", {
        school_id: schoolId,
        error: error instanceof Error ? error.message : String(error),
      });
      void showLoadError(error, {
        object: "die Prüfung der Auswirkungen",
        retry: () => void request(),
      });
    } finally {
      setState((current) => ({ ...current, isLoading: false }));
    }
  }, [schoolId, setState, showLoadError, clearLoadError, clearSaveError]);
  return request;
}

function useImpactConfirm(
  impact: BookingAuthorityImpact | null,
  saveSetting: SaveSetting,
  setState: Dispatch<SetStateAction<ImpactState>>,
  saveErrors: FormErrors,
) {
  const { show: showSaveError, clear: clearSaveError } = saveErrors;
  return useCallback(async () => {
    if (!impact || impact.blockingChildren.length > 0) return;
    clearSaveError();
    setState((current) => ({ ...current, isSaving: true }));
    try {
      await saveSetting(BOOKINGS_AUTHORITATIVE_KEY, true);
      setState((current) => ({ ...current, isOpen: false }));
    } catch (error) {
      void showSaveError(error, {
        object: "die Aktivierung des Buchungsmodus",
      });
    } finally {
      setState((current) => ({ ...current, isSaving: false }));
    }
  }, [impact, saveSetting, setState, showSaveError, clearSaveError]);
}

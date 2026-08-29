"use client";

import { useCallback, useState } from "react";
import type { Dispatch, SetStateAction } from "react";
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
  readonly error: string | null;
}

const initialState: ImpactState = {
  impact: null,
  isOpen: false,
  isLoading: false,
  isSaving: false,
  error: null,
};

type SaveSetting = (key: string, value: unknown) => Promise<string | null>;

export function useBookingAuthorityImpact(
  schoolId: string,
  saveSetting: SaveSetting,
) {
  const [state, setState] = useState<ImpactState>(initialState);
  const request = useImpactRequest(schoolId, setState);
  const confirm = useImpactConfirm(state.impact, saveSetting, setState);
  const close = useCallback(
    () => setState((current) => ({ ...current, isOpen: false })),
    [],
  );
  return { state, request, confirm, close };
}

function useImpactRequest(
  schoolId: string,
  setState: Dispatch<SetStateAction<ImpactState>>,
) {
  return useCallback(async () => {
    setState({ ...initialState, isOpen: true, isLoading: true });
    try {
      const impact = await fetchBookingAuthorityImpact(schoolId);
      setState((current) => ({ ...current, impact }));
    } catch (error) {
      logger.warn("booking_authority_impact_failed", {
        school_id: schoolId,
        error: error instanceof Error ? error.message : String(error),
      });
      setState((current) => ({
        ...current,
        error: "Die Auswirkungen konnten nicht geprüft werden.",
      }));
    } finally {
      setState((current) => ({ ...current, isLoading: false }));
    }
  }, [schoolId, setState]);
}

function useImpactConfirm(
  impact: BookingAuthorityImpact | null,
  saveSetting: SaveSetting,
  setState: Dispatch<SetStateAction<ImpactState>>,
) {
  return useCallback(async () => {
    if (!impact || impact.blockingChildren.length > 0) return;
    setState((current) => ({ ...current, isSaving: true, error: null }));
    try {
      const error = await saveSetting(BOOKINGS_AUTHORITATIVE_KEY, true);
      setState((current) => ({
        ...current,
        error,
        isOpen: error ? current.isOpen : false,
      }));
    } finally {
      setState((current) => ({ ...current, isSaving: false }));
    }
  }, [impact, saveSetting, setState]);
}

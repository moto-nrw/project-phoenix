"use client";

import { homeLayoutKeys } from "~/lib/swr/portal-keys";
import { useEffect, useRef } from "react";
import { useSWRAuth } from "~/lib/swr";
import { useSession } from "next-auth/react";

import type {
  HomeBlockPlacement,
  HomeBlockPolicies,
  HomeLayoutOverrides,
} from "~/lib/home-blocks";
import {
  fetchHomeLayout,
  resetHomeLayout,
  saveHomeBlockPolicies,
  saveHomeLayout,
  type HomeLayoutState,
} from "~/lib/home-layout-api";
import { useOptimisticAction } from "~/lib/swr/optimistic";
import { HOME_LAYOUT_REFUSALS } from "~/lib/everyday-action-refusals";
import { ApiError } from "~/lib/api-error";
import { useTenantSlugSafe } from "~/lib/tenant-context";

const EMPTY: HomeLayoutState = {
  blocks: [],
  overrides: {},
  policies: {},
  canManagePolicies: false,
};

const HOME_LAYOUT_POLICIES_CHANGED = "home-layout";

function homeLayoutAccountChanged(accountID: string): string {
  return `home-layout:${accountID}`;
}

/**
 * Auswahl und Vorgabe der Startseite (#2875).
 *
 * Solange die Antwort aussteht, gilt die Empfehlung — also „alles sichtbar".
 * Das ist Absicht: auf die Abfrage zu WARTEN würde die Startseite leer lassen,
 * falls sie hängt, und ein Zwischenspeicher im Browser würde den ersten
 * Rendervorgang vom Server abweichen lassen (Hydrations-Fehler). Der Preis ist
 * eine Datenabfrage beim ersten Aufruf nach einem echten Neuladen; innerhalb
 * einer Sitzung kennt SWR die Auswahl und ausgeblendete Kacheln fragen nichts
 * mehr nach.
 *
 * Beim Speichern liegt die neue Auswahl sofort über den bestätigten Daten und wird danach
 * neu geladen: die Startseite soll sich unter dem Dialog schon geändert haben,
 * wenn er zugeht.
 */
/**
 * How a layout write ended when it did not fail (a failure throws).
 * `discarded`: nothing happened, because another write was still running or
 * the scope (school, account) or data generation moved while it ran. The
 * caller reports no success and keeps its draft open.
 */
export type HomeLayoutWrite = "done" | "discarded";

export function useHomeLayout(): {
  state: HomeLayoutState;
  /** Edit forms must not seed their drafts from optimistic overlays. */
  confirmedPolicies: HomeBlockPolicies;
  isLoading: boolean;
  isReady: boolean;
  save: (
    overrides: HomeLayoutOverrides,
    blocks: readonly HomeBlockPlacement[],
  ) => Promise<HomeLayoutWrite>;
  reset: () => Promise<HomeLayoutWrite>;
  savePolicies: (policies: HomeBlockPolicies) => Promise<HomeLayoutWrite>;
} {
  const { data: session, status } = useSession();
  const tenantSlug = useTenantSlugSafe();
  const accountID = session?.user?.id;
  const cacheKey =
    status === "authenticated" && tenantSlug && accountID
      ? homeLayoutKeys.key()
      : null;
  const { data, isLoading, mutate } = useSWRAuth<HomeLayoutState>(
    cacheKey,
    fetchHomeLayout,
    { keepPreviousData: false, revalidateOnFocus: false },
  );

  useEffect(() => {
    if (!cacheKey || !accountID || typeof window === "undefined") {
      return undefined;
    }

    const revalidate = (event: Event) => {
      const source =
        event instanceof CustomEvent && typeof event.detail?.source === "string"
          ? event.detail.source
          : null;
      if (
        source !== HOME_LAYOUT_POLICIES_CHANGED &&
        source !== homeLayoutAccountChanged(accountID)
      ) {
        return;
      }
      void mutate();
    };

    window.addEventListener("phoenix:tenant-settings-stale", revalidate);
    return () => {
      window.removeEventListener("phoenix:tenant-settings-stale", revalidate);
    };
  }, [accountID, cacheKey, mutate]);

  const action = useOptimisticAction(cacheKey, data);
  const inFlight = useRef(false);
  const change = async (
    update: (current: HomeLayoutState) => HomeLayoutState,
    request: () => Promise<void>,
  ): Promise<HomeLayoutWrite> => {
    if (inFlight.current) return "discarded";
    if (!data)
      throw new ApiError("Home layout is not loaded", 503, {
        code: "general.unavailable",
      });
    inFlight.current = true;
    try {
      const outcome = await action.run({
        optimistic: update,
        request,
        commit: () => {
          void mutate((current) => current && update(current), {
            revalidate: true,
          });
        },
        refusal: {
          codes: HOME_LAYOUT_REFUSALS,
          matches: (key) => key === homeLayoutKeys.key(),
        },
      });
      if (outcome.status === "failed") throw outcome.error;
      return outcome.status;
    } finally {
      inFlight.current = false;
    }
  };

  const save = (
    overrides: HomeLayoutOverrides,
    blocks: readonly HomeBlockPlacement[],
  ) =>
    change(
      (current) => ({ ...current, overrides, blocks }),
      () => saveHomeLayout(overrides, blocks),
    );
  const reset = () =>
    change(
      (current) => ({ ...current, overrides: {}, blocks: [] }),
      resetHomeLayout,
    );
  const savePolicies = (policies: HomeBlockPolicies) =>
    change(
      (current) => ({ ...current, policies }),
      () => saveHomeBlockPolicies(policies),
    );

  // Ein leerer Zustand ist erst nach einer erfolgreichen Antwort eine sichere
  // Grundlage zum Speichern. Während Laden oder nach einem Fehler bliebe eine
  // Bearbeitung sonst fälschlich bei der Standardansicht stehen und könnte
  // die persönliche Anordnung überschreiben.
  return {
    state: action.data ?? EMPTY,
    confirmedPolicies: data?.policies ?? EMPTY.policies,
    isLoading,
    isReady: data !== undefined,
    save,
    reset,
    savePolicies,
  };
}

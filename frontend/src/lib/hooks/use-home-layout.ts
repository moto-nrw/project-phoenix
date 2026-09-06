"use client";

import { useEffect } from "react";
import useSWR from "swr";
import { useSession } from "next-auth/react";

import type { HomeBlockPolicies, HomeLayoutOverrides } from "~/lib/home-blocks";
import {
  fetchHomeLayout,
  homeLayoutSWRKey,
  resetHomeLayout,
  saveHomeBlockPolicies,
  saveHomeLayout,
  type HomeLayoutState,
} from "~/lib/home-layout-api";
import { useTenantSlugSafe } from "~/lib/tenant-context";

const EMPTY: HomeLayoutState = {
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
 * Beim Speichern wird die neue Auswahl sofort in den Cache gelegt und danach
 * neu geladen: die Startseite soll sich unter dem Dialog schon geändert haben,
 * wenn er zugeht.
 */
export function useHomeLayout(): {
  state: HomeLayoutState;
  isLoading: boolean;
  save: (overrides: HomeLayoutOverrides) => Promise<void>;
  reset: () => Promise<void>;
  savePolicies: (policies: HomeBlockPolicies) => Promise<void>;
} {
  const { data: session, status } = useSession();
  const tenantSlug = useTenantSlugSafe();
  const accountID = session?.user?.id;
  const cacheKey =
    status === "authenticated" && tenantSlug && accountID
      ? homeLayoutSWRKey(tenantSlug, accountID)
      : null;
  const { data, isLoading, mutate } = useSWR<HomeLayoutState>(
    cacheKey,
    fetchHomeLayout,
    { revalidateOnFocus: false },
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

  const save = async (overrides: HomeLayoutOverrides) => {
    await saveHomeLayout(overrides);
    await mutate((current) => ({ ...(current ?? EMPTY), overrides }), {
      revalidate: true,
    });
  };

  const reset = async () => {
    await resetHomeLayout();
    await mutate((current) => ({ ...(current ?? EMPTY), overrides: {} }), {
      revalidate: true,
    });
  };

  const savePolicies = async (policies: HomeBlockPolicies) => {
    await saveHomeBlockPolicies(policies);
    await mutate((current) => ({ ...(current ?? EMPTY), policies }), {
      revalidate: true,
    });
  };

  return { state: data ?? EMPTY, isLoading, save, reset, savePolicies };
}

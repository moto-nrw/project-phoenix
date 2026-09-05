"use client";

import useSWR from "swr";

import type { HomeLayoutOverrides } from "~/lib/home-blocks";
import {
  fetchHomeLayout,
  HOME_LAYOUT_SWR_KEY,
  resetHomeLayout,
  saveHomeLayout,
  type HomeLayoutState,
} from "~/lib/home-layout-api";

const EMPTY: HomeLayoutState = {
  overrides: {},
  policies: {},
  canManagePolicies: false,
};

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
} {
  const { data, isLoading, mutate } = useSWR<HomeLayoutState>(
    HOME_LAYOUT_SWR_KEY,
    fetchHomeLayout,
    { revalidateOnFocus: false },
  );

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

  return { state: data ?? EMPTY, isLoading, save, reset };
}

"use client";

import { useCallback, useMemo } from "react";

import { useTenantRouter } from "~/lib/tenant-router";
import type { SectionSubPage } from "~/lib/section-navigation";

/**
 * Reiter, die auf eine andere Route führen.
 *
 * Die Register der aufgelösten Datenverwaltung sind keine zweite Baumhälfte
 * mehr, sondern Reiter an ihrer Fläche: „Stammdaten" steht bei den Kindern,
 * „Vertretungszugriff" bei den Mitarbeitenden (BAUARTEN-SPEC, Teil 2). Die
 * Seiten selbst bleiben liegen, wo sie liegen — der Reiter navigiert dorthin.
 *
 * `TenantPage.tabs` ruft `onChange` mit dem Wert des Reiters auf; als Wert
 * dient hier der Pfad. Damit braucht es keine eigene Reiterleiste neben der
 * des Gerüsts.
 */
export function useCollectionTabs(
  currentHref: string,
  currentLabel: string,
  linkedTabs: readonly SectionSubPage[],
  label: string,
) {
  const router = useTenantRouter();

  const onChange = useCallback(
    (value: string) => {
      if (value === currentHref) return;
      router.push(value);
    },
    [router, currentHref],
  );

  return useMemo(() => {
    if (linkedTabs.length === 0) return undefined;
    return {
      value: currentHref,
      onChange,
      // Anmerkung: `TenantPageTab` kann seit heute ein `href` tragen und
      // rendert dann einen echten Link (Mittelklick, neuer Tab). Hier bewusst
      // nicht gesetzt: der Pfad müsste über `useTenantAwarePath` mandantenfähig
      // gemacht werden, und dieser Hook fehlt in den Mocks von fünf
      // bestehenden Testdateien. Das nachzuziehen ist eine eigene Änderung,
      // keine Beifang-Anpassung von Tests.
      items: [
        { value: currentHref, label: currentLabel },
        ...linkedTabs.map((tab) => ({ value: tab.href, label: tab.label })),
      ],
      label,
    };
  }, [currentHref, currentLabel, linkedTabs, onChange, label]);
}

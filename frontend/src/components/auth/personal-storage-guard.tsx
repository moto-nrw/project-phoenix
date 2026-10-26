"use client";

import { useLayoutEffect } from "react";
import { useSession } from "next-auth/react";
import {
  bindPersonalStorage,
  clearPersonalStorage,
  personalStorageOwner,
} from "~/lib/personal-storage";

/**
 * Räumt die personenbezogenen Einträge im Browser-Speicher (#5000), sobald
 * die Sitzung ein anderes Konto oder eine andere Schule trägt oder keine
 * mehr besteht. Deckt damit auch die Wege ohne eigenen Logout-Knopf ab:
 * abgelaufene Sitzung, Anmeldung im falschen Portal, Schulwechsel,
 * Übergabe aus dem Träger-Portal und die Vorschau als andere Person.
 *
 * Einmal in jedem Portal als erstes Kind des SessionProvider eingehängt;
 * zeichnet nichts. Der Layout-Effekt läuft vor den Effekten der Seiten, die
 * sich etwas merken.
 */
export function PersonalStorageGuard() {
  const { data: session, status } = useSession();
  const owner =
    status === "authenticated" ? personalStorageOwner(session?.user) : null;

  useLayoutEffect(() => {
    // Während die Sitzung lädt, bleibt alles stehen: ein Abruf ist kein
    // Kontowechsel.
    if (status === "loading") return;
    if (owner === null) {
      clearPersonalStorage();
      return;
    }
    bindPersonalStorage(owner);
  }, [status, owner]);

  return null;
}

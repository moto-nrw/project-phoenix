"use client";

import {
  SettingsReturnPathMemory,
  useSettingsReturnHref,
} from "~/components/ui/settings-return-path";
import { traegerPath } from "~/lib/traeger-url";
import { isTraegerSettingsPath } from "./traeger-nav-active";

const RETURN_PATH_KEY = "traeger-settings-return-path";

/** Merkt sich die aktuelle Seite, solange sie außerhalb der Einstellungen liegt. */
export function TraegerReturnPathMemory() {
  return (
    <SettingsReturnPathMemory
      storageKey={RETURN_PATH_KEY}
      isSettingsPath={isTraegerSettingsPath}
    />
  );
}

/** Das Ziel von „Zurück zur App"; ohne gemerkte Seite die Startseite. */
export function useTraegerReturnHref(): string {
  return useSettingsReturnHref(RETURN_PATH_KEY, traegerPath("/traeger"));
}

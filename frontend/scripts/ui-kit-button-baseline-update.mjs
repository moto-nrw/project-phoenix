#!/usr/bin/env node
// Schreibt scripts/ui-kit-button-baseline.json neu (#5531): Funde je Datei der
// Regeln ui-kit/no-hand-rolled-button, ui-kit/no-button-override und
// ui-kit/no-grey-button-fill (#5574, immer leer), ohne die
// Ausnahmen der Allowlist. Nur nach dem Abbau von Funden ausführen; die
// Baseline darf nur sinken. `--check` schreibt nichts und meldet Abweichungen.
import { buttonRatchetAllowlist } from "./ui-kit-button-baseline.mjs";
import { updateRatchetBaseline } from "./ui-kit-ratchet-baseline.mjs";

// no-grey-button-fill (#5574) ist hart null: Die Zeile hier prüft nur ihre
// Allowlist; ein Fund außerhalb davon hebt die Baseline nicht, er schlägt fehl.
updateRatchetBaseline({
  rules: ["no-hand-rolled-button", "no-button-override", "no-grey-button-fill"],
  baselineFile: "ui-kit-button-baseline.json",
  pathEnv: "UI_KIT_BUTTON_BASELINE_PATH",
  countEnv: "UI_KIT_BUTTON_RATCHET",
  allowlist: buttonRatchetAllowlist,
  command: "pnpm run ui-kit:button-baseline",
});

#!/usr/bin/env node
// Schreibt scripts/ui-kit-dot-baseline.json neu (#5598): Funde je Datei der
// Regel ui-kit/no-hand-rolled-dot. Nur nach dem Abbau von Funden ausführen; die
// Baseline darf nur sinken. `--check` schreibt nichts und meldet Abweichungen.
import { updateRatchetBaseline } from "./ui-kit-ratchet-baseline.mjs";

updateRatchetBaseline({
  rules: ["no-hand-rolled-dot"],
  baselineFile: "ui-kit-dot-baseline.json",
  pathEnv: "UI_KIT_DOT_BASELINE_PATH",
  countEnv: "UI_KIT_DOT_RATCHET",
  allowlist: [],
  command: "pnpm run ui-kit:dot-baseline",
});

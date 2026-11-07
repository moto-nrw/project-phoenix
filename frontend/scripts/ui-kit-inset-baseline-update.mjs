#!/usr/bin/env node
// Schreibt scripts/ui-kit-inset-baseline.json neu (#5592). Nur nach dem Abbau
// von Funden ausführen; die Baseline darf nur sinken. `--check` schreibt nichts
// und meldet Abweichungen.
import { updateRatchetBaseline } from "./ui-kit-ratchet-baseline.mjs";

updateRatchetBaseline({
  rules: ["no-hand-rolled-inset"],
  baselineFile: "ui-kit-inset-baseline.json",
  pathEnv: "UI_KIT_INSET_BASELINE_PATH",
  countEnv: "UI_KIT_INSET_RATCHET",
  allowlist: [],
  command: "pnpm run ui-kit:inset-baseline",
});

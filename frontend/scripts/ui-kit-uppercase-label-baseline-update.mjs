#!/usr/bin/env node
// Schreibt scripts/ui-kit-uppercase-label-baseline.json neu (#5590). Nur nach
// dem Abbau von Funden ausführen; die Baseline darf nur sinken. `--check`
// schreibt nichts und meldet Abweichungen.
import { updateRatchetBaseline } from "./ui-kit-ratchet-baseline.mjs";

updateRatchetBaseline({
  rules: ["no-uppercase-label"],
  baselineFile: "ui-kit-uppercase-label-baseline.json",
  pathEnv: "UI_KIT_UPPERCASE_LABEL_BASELINE_PATH",
  countEnv: "UI_KIT_UPPERCASE_LABEL_RATCHET",
  allowlist: [],
  command: "pnpm run ui-kit:uppercase-label-baseline",
});

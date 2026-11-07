#!/usr/bin/env node
// Schreibt scripts/ui-kit-surface-baseline.json neu (#5594). Nur nach dem
// Abbau von Funden ausführen; die Baseline darf nur sinken. `--check` schreibt
// nichts und meldet Abweichungen.
import { surfaceAllowlist } from "./ui-kit-surface-baseline.mjs";
import { updateRatchetBaseline } from "./ui-kit-ratchet-baseline.mjs";

updateRatchetBaseline({
  rules: ["no-hand-rolled-surface", "no-hand-rolled-page-card"],
  baselineFile: "ui-kit-surface-baseline.json",
  pathEnv: "UI_KIT_SURFACE_BASELINE_PATH",
  countEnv: "UI_KIT_SURFACE_RATCHET",
  allowlist: surfaceAllowlist,
  command: "pnpm run ui-kit:surface-baseline",
});

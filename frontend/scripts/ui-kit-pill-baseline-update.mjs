#!/usr/bin/env node
// Schreibt scripts/ui-kit-pill-baseline.json neu (#5584): Funde je Datei der
// Regeln ui-kit/no-hand-rolled-pill, ui-kit/no-pill-dot und
// ui-kit/no-hex-alpha-tint. Nur nach dem Abbau von Funden ausführen; die
// Baseline darf nur sinken. `--check` schreibt nichts und meldet Abweichungen.
import { updateRatchetBaseline } from "./ui-kit-ratchet-baseline.mjs";

updateRatchetBaseline({
  rules: ["no-hand-rolled-pill", "no-pill-dot", "no-hex-alpha-tint"],
  baselineFile: "ui-kit-pill-baseline.json",
  pathEnv: "UI_KIT_PILL_BASELINE_PATH",
  countEnv: "UI_KIT_PILL_RATCHET",
  allowlist: [],
  command: "pnpm run ui-kit:pill-baseline",
});

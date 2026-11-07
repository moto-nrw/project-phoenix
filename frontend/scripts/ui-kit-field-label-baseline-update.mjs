#!/usr/bin/env node
// Schreibt scripts/ui-kit-field-label-baseline.json neu (#5589). Nur nach dem
// Abbau von Funden ausführen; die Baseline darf nur sinken. `--check` schreibt
// nichts und meldet Abweichungen.
import { fieldLabelAllowlist } from "./ui-kit-field-label-baseline.mjs";
import { updateRatchetBaseline } from "./ui-kit-ratchet-baseline.mjs";

updateRatchetBaseline({
  rules: ["no-hand-rolled-field-label"],
  baselineFile: "ui-kit-field-label-baseline.json",
  pathEnv: "UI_KIT_FIELD_LABEL_BASELINE_PATH",
  countEnv: "UI_KIT_FIELD_LABEL_RATCHET",
  allowlist: fieldLabelAllowlist,
  command: "pnpm run ui-kit:field-label-baseline",
});

#!/usr/bin/env node
// Schreibt scripts/ui-kit-notice-baseline.json neu (#5591). Nur nach dem
// Abbau von Funden ausführen; die Baseline darf nur sinken. `--check` schreibt
// nichts und meldet Abweichungen.
import { noticeAllowlist } from "./ui-kit-notice-baseline.mjs";
import { updateRatchetBaseline } from "./ui-kit-ratchet-baseline.mjs";

updateRatchetBaseline({
  rules: ["no-hand-rolled-notice"],
  baselineFile: "ui-kit-notice-baseline.json",
  pathEnv: "UI_KIT_NOTICE_BASELINE_PATH",
  countEnv: "UI_KIT_NOTICE_RATCHET",
  allowlist: noticeAllowlist,
  command: "pnpm run ui-kit:notice-baseline",
});

import { formatDate } from "~/lib/date-helpers";
import type { ResolvedSetting } from "~/lib/settings-api";

/**
 * Who looks at a settings field: the school (tenant settings page), the moto
 * team for one school (operator page) or the Träger-Büro for all its schools
 * (Träger portal, #4652).
 */
export type SettingsAudience = "admin" | "operator" | "traeger";

/*
 * Where a value comes from (ADR 0086): registry default, then the Träger
 * value, then the school's own value. The school schema says so per item in
 * `source`; `traeger_changed_at` is set while a Träger value exists, also
 * beneath a school's own value.
 */

function changedOn(setting: ResolvedSetting): string {
  return setting.traeger_changed_at
    ? ` · geändert am ${formatDate(setting.traeger_changed_at)}`
    : "";
}

/** A Träger value lies beneath the school's own value. */
function hasTraegerValueBeneath(setting: ResolvedSetting): boolean {
  return (
    setting.levels === "beide" &&
    setting.source === "school" &&
    setting.traeger_changed_at != null &&
    setting.traeger_changed_at !== ""
  );
}

/** The line under a field that says who decided its value, if anyone. */
export function settingOriginNote(
  setting: ResolvedSetting,
  audience: SettingsAudience,
): string | null {
  if (audience === "traeger") {
    if (setting.source === "traeger") {
      return `Vom Träger festgelegt${changedOn(setting)}`;
    }
    return setting.levels === "traeger"
      ? "Gilt für alle Schulen. Die Schulen können den Wert nicht ändern."
      : null;
  }
  if (setting.levels === "traeger") {
    return `Legt der Träger fest${changedOn(setting)}`;
  }
  if (setting.source === "traeger") {
    return `Vom Träger festgelegt${changedOn(setting)}`;
  }
  if (hasTraegerValueBeneath(setting)) {
    return audience === "operator"
      ? "Eigener Wert der Schule. „Zurücksetzen“ übernimmt den Wert des Trägers."
      : "Eigener Wert Ihrer Schule. „Zurücksetzen“ übernimmt den Wert des Trägers.";
  }
  return null;
}

/** „Standard“: the value is the registry default, nobody set one. */
export function showsStandardBadge(
  setting: ResolvedSetting,
  audience: SettingsAudience,
): boolean {
  if (audience === "traeger") return setting.is_default;
  // The school's own value of a Träger setting is no standard, even when it
  // equals one: it no longer follows the Träger.
  if (followsTraegerAfterReset(setting, audience)) return false;
  return (
    setting.is_default &&
    setting.source !== "traeger" &&
    !hasTraegerValueBeneath(setting)
  );
}

/**
 * Whether the school's own value of a setting the Träger may set can be
 * removed. Reset is offered for every field type, booleans included, also
 * while the Träger has no value yet: toggling back would keep a value of the
 * school's own, so a later Träger value would never reach the school.
 */
export function followsTraegerAfterReset(
  setting: ResolvedSetting,
  audience: SettingsAudience,
): boolean {
  return (
    audience !== "traeger" &&
    setting.levels === "beide" &&
    setting.source === "school"
  );
}

/**
 * Whether „Zurücksetzen“ brings back the Träger value instead of the
 * default. Such a reset is offered for every field type, booleans included:
 * toggling back would keep the school's own value, so later Träger changes
 * would no longer reach the school.
 */
export function resetRestoresTraegerValue(
  setting: ResolvedSetting,
  audience: SettingsAudience,
): boolean {
  return audience !== "traeger" && hasTraegerValueBeneath(setting);
}

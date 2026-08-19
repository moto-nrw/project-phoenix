import type { BeforeSendFn, Properties } from "posthog-js";
import { isAnalyticsViewId } from "~/lib/analytics-routes";

const allowedEvents = new Set([
  "page_viewed",
  "login_success",
  "login_failed",
  "tenant_switched",
  "suggestion_created",
  "suggestion_voted",
  "group_created",
  "group_updated",
  "user_invited",
  "data_exported",
  "pwa_install_prompt_shown",
  "pwa_install_prompt_accepted",
  "pwa_install_prompt_dismissed",
  "pwa_installed",
]);

const safePropertyValues: Readonly<Record<string, ReadonlySet<string>>> = {
  direction: new Set(["up", "down"]),
  export_type: new Set(["rooms", "emergency", "students"]),
  format: new Set(["pdf", "docx", "xlsx"]),
  reason: new Set(["error", "invalid_credentials"]),
};

const safeSdkProperties = new Set([
  "token",
  "distinct_id",
  "$lib",
  "$lib_version",
]);

function isSafeSchoolID(value: unknown): value is string {
  return typeof value === "string" && /^\d+$/.test(value);
}

function isSafeDeployment(value: unknown): value is string {
  return (
    typeof value === "string" &&
    value.length <= 253 &&
    /^[a-z0-9.-]+(?::\d+)?$/i.test(value)
  );
}

function sanitizeGroups(value: unknown): Properties["$groups"] | undefined {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return undefined;
  }

  const school = (value as Record<string, unknown>).school;
  if (!isSafeSchoolID(school) || Object.keys(value).length !== 1) {
    return undefined;
  }
  return { school };
}

function sanitizeProperties(properties: Properties): Properties {
  const safe: Properties = {};

  for (const [key, value] of Object.entries(properties)) {
    if (safeSdkProperties.has(key) && typeof value === "string") {
      safe[key] = value;
      continue;
    }
    if (typeof value === "string" && safePropertyValues[key]?.has(value)) {
      safe[key] = value;
      continue;
    }
    if (key === "view_id" && isAnalyticsViewId(value)) {
      safe.view_id = value;
      continue;
    }
    if (key === "portal" && value === "tenant") {
      safe.portal = value;
      continue;
    }
    if (key === "deployment" && isSafeDeployment(value)) {
      safe.deployment = value;
      continue;
    }
    if (key === "school_id" && isSafeSchoolID(value)) {
      safe.school_id = value;
      continue;
    }
    if (key === "$groups") {
      const groups = sanitizeGroups(value);
      if (groups) safe.$groups = groups;
    }
  }

  // These flags are forced after filtering so callers cannot override them.
  safe.$geoip_disable = true;
  safe.$process_person_profile = false;
  return safe;
}

/** Final defense before any browser event leaves the application. */
export const sanitizePostHogEvent: BeforeSendFn = (captureResult) => {
  if (!captureResult || !allowedEvents.has(captureResult.event)) return null;

  return {
    uuid: captureResult.uuid,
    event: captureResult.event,
    timestamp: captureResult.timestamp,
    properties: sanitizeProperties(captureResult.properties),
  };
};

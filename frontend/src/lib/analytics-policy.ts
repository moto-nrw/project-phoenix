/**
 * The rules of the usage analytics (Nutzungsanalyse, #3598 / #3601).
 *
 * One pure module decides, for an analytics context, which PostHog init
 * options apply and what the `before_send` filter lets leave the browser.
 * `posthog-client.ts` only loads the SDK, buffers calls, and asks this module.
 * Every privacy rule of the browser side lives here, so one review covers all
 * of them.
 *
 * Floor for real schools, on every surface: no element texts, URLs only as
 * route templates without IDs or query, no IP and no GeoIP, no person
 * profile. A surface this module does not know gets the strictest rules
 * (those of the parents portal).
 */

import type { CaptureResult, PostHogConfig, Properties } from "posthog-js";
import {
  resolveAnalyticsRoute,
  UNKNOWN_ANALYTICS_PATH,
  type AnalyticsRouteSurface,
} from "~/lib/analytics-routes";

const ANALYTICS_SURFACES = [
  "ogs",
  "parents",
  "school",
  "traeger",
  "public",
] as const;

export type AnalyticsSurface = (typeof ANALYTICS_SURFACES)[number];

/** `deployment` of the public demo; any other value is a school deployment. */
const DEMO_DEPLOYMENT = "demo";

/** Same-origin reverse proxy to PostHog EU (`next.config.js` rewrites). */
const POSTHOG_PROXY_PATH = "/ingest";
const POSTHOG_UI_HOST = "https://eu.posthog.com";

export interface AnalyticsContext {
  /** `demo` in the public demo, otherwise the tenant domain. */
  readonly deployment: string;
  /** The portal. An unknown value gets the strictest rules. */
  readonly surface: string;
  /** Analyse-Freigabe of the school; only effective on `ogs`. */
  readonly analyseFreigabe: boolean;
}

/** Values of the `role` property: the role model, never a free-text name. */
const ANALYTICS_ROLES = ["admin", "staff", "lehrkraft", "guardian"] as const;

export type AnalyticsRole = (typeof ANALYTICS_ROLES)[number];

type Tier = "demo" | "ogs" | "ogs_freigabe" | "strict";

interface TierRules {
  /** Autocapture may send the text of the clicked element. */
  readonly elementText: boolean;
}

// Session recording (demo, OGS with Analyse-Freigabe) and pseudonymous IDs
// follow in #3603. Until then no tier records and the filter drops every
// `$snapshot`, even if the PostHog project switches recording on for all.
const TIER_RULES: Readonly<Record<Tier, TierRules>> = {
  // The demo shows invented data only; the element text makes the funnel
  // readable. URLs stay templates there as well.
  demo: { elementText: true },
  ogs: { elementText: false },
  ogs_freigabe: { elementText: false },
  strict: { elementText: false },
};

const KNOWN_SURFACES: ReadonlySet<string> = new Set(ANALYTICS_SURFACES);

function isKnownSurface(surface: string): surface is AnalyticsSurface {
  return KNOWN_SURFACES.has(surface);
}

function tierOf(context: AnalyticsContext): Tier {
  if (!isKnownSurface(context.surface)) return "strict";
  if (context.deployment === DEMO_DEPLOYMENT) return "demo";
  if (context.surface === "ogs") {
    return context.analyseFreigabe ? "ogs_freigabe" : "ogs";
  }
  return "strict";
}

function routeSurfaceOf(context: AnalyticsContext): AnalyticsRouteSurface {
  switch (context.surface) {
    case "ogs":
    case "parents":
    case "school":
      return context.surface;
    default:
      // Public pages, the Träger-Büro, and unknown surfaces try every list;
      // the result is still a template or `/unknown`.
      return "public";
  }
}

export interface AnalyticsHosts {
  readonly operator: string;
  readonly parents: string;
  readonly school: string;
  readonly tenantDomain: string;
}

/**
 * The surface a page load starts on, before a portal registers its session.
 * Returns null on the operator host: the operator portal has no analytics.
 */
export function analyticsSurfaceForHost(
  host: string,
  hosts: AnalyticsHosts,
): AnalyticsSurface | null {
  const hostname = host.split(":")[0] ?? "";
  if (host === hosts.operator) return null;
  if (host === hosts.parents) return "parents";
  if (host === hosts.school) return "school";
  if (hostname.endsWith(`.${hosts.tenantDomain}`)) return "ogs";
  return "public";
}

/** PostHog init options for a context. `before_send` comes from the client. */
export function analyticsInitOptions(
  context: AnalyticsContext,
  ownHostname: string,
): Partial<PostHogConfig> {
  const rules = TIER_RULES[tierOf(context)];
  return {
    api_host: POSTHOG_PROXY_PATH,
    ui_host: POSTHOG_UI_HOST,
    defaults: "2026-01-30",
    // No cookies, no local or session storage: no consent banner needed.
    persistence: "memory",
    disable_persistence: true,
    person_profiles: "never",
    save_referrer: false,
    save_campaign_params: false,
    // The default marks localhost as test user and processes a person.
    internal_or_test_user_hostname: null,
    // Loads the remote config, evaluates no feature flags.
    advanced_disable_feature_flags: true,
    disable_surveys: true,
    disable_product_tours: true,
    disable_conversations: true,
    disable_web_experiments: true,
    capture_pageview: "history_change",
    capture_pageleave: true,
    disable_scroll_properties: false,
    autocapture: true,
    mask_all_text: !rules.elementText,
    mask_all_element_attributes: true,
    capture_heatmaps: true,
    capture_dead_clicks: true,
    rageclick: true,
    capture_performance: false,
    capture_exceptions: false,
    disable_session_recording: true,
    // The backend links its events to the browser session (#3602).
    tracing_headers: [ownHostname],
  };
}

// --- before_send filter ---------------------------------------------------

const PAGE_EVENTS: ReadonlySet<string> = new Set(["$pageview", "$pageleave"]);
const ELEMENT_EVENTS: ReadonlySet<string> = new Set([
  "$autocapture",
  "$rageclick",
  "$dead_click",
]);
const HEATMAP_EVENT = "$$heatmap";

const CUSTOM_EVENTS: ReadonlySet<string> = new Set([
  "login_success",
  "login_failed",
  "tenant_switched",
  "group_created",
  "group_updated",
  "user_invited",
  "data_exported",
  "pwa_install_prompt_shown",
  "pwa_install_prompt_accepted",
  "pwa_install_prompt_dismissed",
  "pwa_installed",
  // Public demo (#3467): the visitor is the demo access, never a person.
  "demo_entered",
  "demo_role_switched",
  "demo_restarted",
  "demo_start_clicked",
]);

const SAFE_VALUES: Readonly<Record<string, ReadonlySet<string>>> = {
  role: new Set(ANALYTICS_ROLES),
  demo_role: new Set(["caregiver", "lead", "parent", "all"]),
  direction: new Set(["up", "down"]),
  export_type: new Set(["rooms", "emergency", "students"]),
  format: new Set(["pdf", "docx", "xlsx"]),
  reason: new Set(["error", "invalid_credentials"]),
  $device_type: new Set(["Desktop", "Mobile", "Tablet", "Console", "Wearable"]),
  $event_type: new Set(["click", "submit", "change", "touch"]),
  $dead_swipe_direction: new Set(["up", "down", "left", "right"]),
};

/** SDK identifiers: anonymous runtime IDs, UUIDs, the project token. */
const SAFE_ID_KEYS: ReadonlySet<string> = new Set([
  "token",
  "distinct_id",
  "$session_id",
  "$window_id",
  "$pageview_id",
  "$prev_pageview_id",
  "$insert_id",
]);

/** Short technical labels such as browser, OS, or library version. */
const SAFE_LABEL_KEYS: ReadonlySet<string> = new Set([
  "$lib",
  "$lib_version",
  "$browser",
  "$os",
  "$os_version",
  "$browser_language",
]);

const SAFE_NUMBER_KEYS: ReadonlySet<string> = new Set([
  "$browser_version",
  "$screen_height",
  "$screen_width",
  "$viewport_height",
  "$viewport_width",
  "$ce_version",
]);

const PREV_PAGEVIEW_NUMBER =
  /^\$prev_pageview_(duration|(last|max)_(scroll|content)(_percentage)?)$/;
const DEAD_CLICK_NUMBER = /^\$dead_(click|swipe)_[a-z_]+$/;

function isSafeId(value: unknown): value is string {
  return typeof value === "string" && /^[\w.:-]{1,128}$/.test(value);
}

function isSafeLabel(value: unknown): value is string {
  return typeof value === "string" && /^[\w .()/+-]{1,64}$/.test(value);
}

function isSafeSchoolID(value: unknown): value is string {
  return typeof value === "string" && /^\d+$/.test(value);
}

function isSafeHost(value: unknown): value is string {
  return (
    typeof value === "string" &&
    value.length <= 253 &&
    /^[a-z0-9.-]+(?::\d+)?$/i.test(value)
  );
}

function isFiniteNumber(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value);
}

function parseHttpUrl(value: unknown): URL | null {
  if (typeof value !== "string") return null;
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:" ? url : null;
  } catch {
    return null;
  }
}

function templatePath(surface: AnalyticsRouteSurface, pathname: string) {
  return resolveAnalyticsRoute(surface, pathname) ?? UNKNOWN_ANALYTICS_PATH;
}

/** Origin plus route template; query, fragment, and IDs never survive. */
function templateUrl(
  surface: AnalyticsRouteSurface,
  value: unknown,
): string | undefined {
  const url = parseHttpUrl(value);
  return url
    ? `${url.origin}${templatePath(surface, url.pathname)}`
    : undefined;
}

function templatePathname(
  surface: AnalyticsRouteSurface,
  value: unknown,
): string | undefined {
  return typeof value === "string" ? templatePath(surface, value) : undefined;
}

// --- $elements_chain -------------------------------------------------------
//
// Autocapture serializes the clicked element and its ancestors as
// `tag.class1.class2:attr="value"attr="value";parent…`. Even with
// `mask_all_element_attributes` the SDK adds the `href` of links, and
// `data-ph-capture-attribute-*` ends up in the properties. The filter rebuilds
// the chain from tag, classes, and position only (plus the text in the demo),
// and drops the whole chain when it does not parse.

const CHAIN_POSITION_KEYS: ReadonlySet<string> = new Set([
  "nth-child",
  "nth-of-type",
]);

function splitChain(chain: string): string[] {
  const elements: string[] = [];
  let start = 0;
  let inValue = false;
  for (let index = 0; index < chain.length; index++) {
    const char = chain[index];
    if (inValue) {
      if (char === "\\" && chain[index + 1] === '"') index++;
      else if (char === '"') inValue = false;
    } else if (char === '"') {
      inValue = true;
    } else if (char === ";") {
      elements.push(chain.slice(start, index));
      start = index + 1;
    }
  }
  elements.push(chain.slice(start));
  return elements;
}

const CHAIN_KEY = /[\w-]+/y;

function parseChainElement(
  element: string,
): { selector: string; attributes: Array<[string, string]> } | null {
  const firstQuote = element.indexOf('"');
  if (firstQuote < 2 || element[firstQuote - 1] !== "=") return null;

  let keyStart = firstQuote - 1;
  while (keyStart > 0 && /[\w-]/.test(element[keyStart - 1] ?? "")) keyStart--;
  if (element[keyStart - 1] !== ":") return null;

  const selector = element.slice(0, keyStart - 1);
  if (!/^[a-z][a-z0-9-]*(\.[^\s"]+)*$/i.test(selector)) return null;

  const attributes: Array<[string, string]> = [];
  let index = keyStart;
  while (index < element.length) {
    CHAIN_KEY.lastIndex = index;
    const key = CHAIN_KEY.exec(element)?.[0];
    if (
      !key ||
      element.slice(index + key.length, index + key.length + 2) !== '="'
    ) {
      return null;
    }
    index += key.length + 2;
    let value = "";
    let closed = false;
    while (index < element.length) {
      const char = element[index];
      if (char === "\\" && element[index + 1] === '"') {
        value += '\\"';
        index += 2;
      } else if (char === '"') {
        closed = true;
        index++;
        break;
      } else {
        value += char;
        index++;
      }
    }
    if (!closed) return null;
    attributes.push([key, value]);
  }
  return { selector, attributes };
}

function sanitizeElementsChain(
  value: unknown,
  keepText: boolean,
): string | undefined {
  if (typeof value !== "string" || value === "" || value.length > 20_000) {
    return undefined;
  }
  const sanitized: string[] = [];
  for (const element of splitChain(value)) {
    const parsed = parseChainElement(element);
    if (!parsed) return undefined;
    const kept = parsed.attributes.filter(
      ([key, attributeValue]) =>
        (CHAIN_POSITION_KEYS.has(key) && /^\d+$/.test(attributeValue)) ||
        (keepText && key === "text"),
    );
    sanitized.push(
      `${parsed.selector}:${kept.map(([key, v]) => `${key}="${v}"`).join("")}`,
    );
  }
  return sanitized.join(";");
}

// --- $$heatmap ---------------------------------------------------------------

interface HeatmapPoint {
  x: number;
  y: number;
  target_fixed: boolean;
  type: string;
}

function sanitizeHeatmapData(
  surface: AnalyticsRouteSurface,
  value: unknown,
): Record<string, HeatmapPoint[]> | undefined {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return undefined;
  }
  const byUrl: Record<string, HeatmapPoint[]> = {};
  for (const [rawUrl, points] of Object.entries(value)) {
    const url = templateUrl(surface, rawUrl);
    if (!url || !Array.isArray(points)) continue;
    for (const point of points as unknown[]) {
      if (typeof point !== "object" || point === null) continue;
      const { x, y, target_fixed, type } = point as Record<string, unknown>;
      if (!isFiniteNumber(x) || !isFiniteNumber(y)) continue;
      if (typeof type !== "string" || !/^[a-z]{1,20}$/.test(type)) continue;
      (byUrl[url] ??= []).push({
        x,
        y,
        target_fixed: target_fixed === true,
        type,
      });
    }
  }
  return Object.keys(byUrl).length > 0 ? byUrl : undefined;
}

// --- properties ----------------------------------------------------------

function sanitizeCommon(
  key: string,
  value: unknown,
  surface: AnalyticsRouteSurface,
): unknown {
  if (SAFE_ID_KEYS.has(key)) return isSafeId(value) ? value : undefined;
  if (SAFE_LABEL_KEYS.has(key)) return isSafeLabel(value) ? value : undefined;
  if (SAFE_NUMBER_KEYS.has(key)) {
    return isFiniteNumber(value) ? value : undefined;
  }
  if (typeof value === "string" && SAFE_VALUES[key]?.has(value)) return value;

  switch (key) {
    case "$current_url":
      return templateUrl(surface, value);
    case "$pathname":
      return templatePathname(surface, value);
    case "$referrer":
      return value === "$direct" ? value : templateUrl(surface, value);
    case "$host":
      return isSafeHost(value) ? value : undefined;
    case "school_id":
      return isSafeSchoolID(value) ? value : undefined;
    // The demo source is a campaign label from the website (`messe`,
    // `website`); anything else could be typed-in personal data.
    case "src":
      return typeof value === "string" && /^[a-z0-9_-]{1,40}$/i.test(value)
        ? value
        : undefined;
    default:
      return undefined;
  }
}

function sanitizeEventSpecific(
  event: string,
  key: string,
  value: unknown,
  surface: AnalyticsRouteSurface,
  rules: TierRules,
): unknown {
  if (PAGE_EVENTS.has(event)) {
    if (key === "$prev_pageview_pathname") {
      return templatePathname(surface, value);
    }
    if (PREV_PAGEVIEW_NUMBER.test(key)) {
      return isFiniteNumber(value) ? value : undefined;
    }
  }
  if (ELEMENT_EVENTS.has(event)) {
    if (key === "$elements_chain") {
      return sanitizeElementsChain(value, rules.elementText);
    }
    if (key === "$el_text" && rules.elementText) {
      return typeof value === "string" ? value.slice(0, 400) : undefined;
    }
    if (DEAD_CLICK_NUMBER.test(key)) {
      return isFiniteNumber(value) || typeof value === "boolean"
        ? value
        : undefined;
    }
  }
  if (event === HEATMAP_EVENT && key === "$heatmap_data") {
    return sanitizeHeatmapData(surface, value);
  }
  return undefined;
}

function sanitizeProperties(
  context: AnalyticsContext,
  event: string,
  properties: Properties,
): Properties {
  const surface = routeSurfaceOf(context);
  const rules = TIER_RULES[tierOf(context)];
  const safe: Properties = {};

  for (const [key, value] of Object.entries(properties)) {
    const sanitized =
      sanitizeCommon(key, value, surface) ??
      sanitizeEventSpecific(event, key, value, surface, rules);
    if (sanitized !== undefined) safe[key] = sanitized;
  }

  // Forced after filtering so neither a caller nor a registered property can
  // override them.
  safe.deployment = isSafeHost(context.deployment)
    ? context.deployment
    : "unknown";
  safe.surface = isKnownSurface(context.surface) ? context.surface : "unknown";
  safe.$geoip_disable = true;
  safe.$process_person_profile = false;
  return safe;
}

function isAllowedEvent(event: string): boolean {
  return (
    PAGE_EVENTS.has(event) ||
    ELEMENT_EVENTS.has(event) ||
    event === HEATMAP_EVENT ||
    CUSTOM_EVENTS.has(event)
  );
}

/**
 * Final defense before any browser event leaves the application. Builds a
 * new event from allowlisted properties only; everything else is dropped.
 */
export function filterAnalyticsEvent(
  context: AnalyticsContext,
  captureResult: CaptureResult | null,
): CaptureResult | null {
  if (!captureResult || !isAllowedEvent(captureResult.event)) return null;

  return {
    uuid: captureResult.uuid,
    event: captureResult.event,
    timestamp: captureResult.timestamp,
    properties: sanitizeProperties(
      context,
      captureResult.event,
      captureResult.properties,
    ),
  };
}

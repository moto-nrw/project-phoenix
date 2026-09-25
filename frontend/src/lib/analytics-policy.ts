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
 *
 * Session recording (#3603) runs in two places only: the public demo, with
 * inputs masked and the `/start` form blocked, and the OGS portal of a school
 * with Analyse-Freigabe, with every text, input, image, and attribute masked
 * and a pseudonymous user ID. Both guards hold on their own: the client never
 * starts the recorder elsewhere, and the filter drops every `$snapshot` and
 * every person event outside those contexts, even if the PostHog project
 * switches recording on for all.
 */

import type { CaptureResult, PostHogConfig, Properties } from "posthog-js";
import { isPseudonym } from "~/lib/analytics-pseudonym";
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
  /**
   * Share of sessions recorded with the Analyse-Freigabe, 0 to 100. Missing
   * or 0 records nothing.
   */
  readonly recordingSamplePercent?: number;
  /**
   * Pseudonymous ID of the signed-in OGS account (`analytics-pseudonym.ts`).
   * Only effective with the Analyse-Freigabe on `ogs`.
   */
  readonly person?: string | null;
  /** Role of the signed-in session, the only person property. */
  readonly role?: AnalyticsRole;
}

/** Values of the `role` property: the role model, never a free-text name. */
const ANALYTICS_ROLES = ["admin", "staff", "lehrkraft", "guardian"] as const;

export type AnalyticsRole = (typeof ANALYTICS_ROLES)[number];

type Tier = "demo" | "ogs" | "ogs_freigabe" | "strict";

type Recording = "demo" | "school" | null;

interface TierRules {
  /** Autocapture may send the text of the clicked element. */
  readonly elementText: boolean;
  /** Which session recording the tier runs, if any. */
  readonly recording: Recording;
  /** The session may be a pseudonymous person. */
  readonly person: boolean;
}

const TIER_RULES: Readonly<Record<Tier, TierRules>> = {
  // The demo shows invented data only; the element text makes the funnel
  // readable, and every session is recorded. URLs stay templates there as
  // well. The visitor is the demo access, never a person profile.
  demo: { elementText: true, recording: "demo", person: false },
  ogs: { elementText: false, recording: null, person: false },
  ogs_freigabe: { elementText: false, recording: "school", person: true },
  strict: { elementText: false, recording: null, person: false },
};

/**
 * Marks an element the recorder replaces with an empty box of the same size,
 * in every recording. The `/start` form carries it: names, addresses, and
 * organisations of prospects are never recorded, not even masked.
 */
export const ANALYTICS_BLOCK_ATTRIBUTE = "data-analytics-block";
const BLOCK_SELECTOR = `[${ANALYTICS_BLOCK_ATTRIBUTE}]`;

/**
 * With the Analyse-Freigabe every image, video, and embedded document is
 * blocked as well: a child's photo must not reach a recording in any form.
 */
const SCHOOL_BLOCK_SELECTOR = [
  "img",
  "picture",
  "video",
  "audio",
  "canvas",
  "svg image",
  "iframe",
  "object",
  "embed",
  BLOCK_SELECTOR,
].join(", ");

/**
 * Attributes a school recording keeps: layout and state, never content.
 * Everything else (`alt`, `title`, `aria-label`, `id`, `src`, `style`, `data-*`
 * ...) is masked, because a name or a photo URL can hide in any of them.
 * `href` keeps its route template.
 */
const SCHOOL_KEPT_ATTRIBUTES: ReadonlySet<string> = new Set([
  "class",
  "type",
  "role",
  "rel",
  "width",
  "height",
  "viewBox",
  "d",
  "fill",
  "stroke",
  "stroke-width",
  "stroke-linecap",
  "stroke-linejoin",
  "fill-rule",
  "clip-rule",
  "xmlns",
  "aria-hidden",
  "aria-expanded",
  "aria-selected",
  "aria-checked",
  "aria-disabled",
  "data-state",
  "disabled",
  "hidden",
]);

const MASKED_ATTRIBUTE = "*";

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

function recordingSampleRate(context: AnalyticsContext): number {
  const percent = context.recordingSamplePercent ?? 0;
  return Number.isFinite(percent)
    ? Math.min(Math.max(percent, 0), 100) / 100
    : 0;
}

/** The recording a context runs; null records nothing. */
function recordingOf(context: AnalyticsContext): Recording {
  const recording = TIER_RULES[tierOf(context)].recording;
  if (recording === "school" && recordingSampleRate(context) === 0) return null;
  return recording;
}

/**
 * The pseudonymous ID the session identifies with, or null. Only the OGS
 * portal of a school with Analyse-Freigabe has one.
 */
export function analyticsIdentity(context: AnalyticsContext): string | null {
  return TIER_RULES[tierOf(context)].person && isPseudonym(context.person)
    ? context.person
    : null;
}

type SessionRecordingOptions = NonNullable<PostHogConfig["session_recording"]>;
type CapturedNetworkRequest = Parameters<
  NonNullable<SessionRecordingOptions["maskCapturedNetworkRequestFn"]>
>[0];

function recordingOptions(
  context: AnalyticsContext,
  recording: "demo" | "school",
): SessionRecordingOptions {
  const scope = urlScopeOf(context);
  // The recorder passes the page URL of the recording (and of each network
  // entry, if network capture ever runs) through this function: it leaves as
  // the route template on the deployment host, without headers or bodies.
  const maskRequest = (
    request: CapturedNetworkRequest,
  ): CapturedNetworkRequest => ({
    ...request,
    name: templateUrl(scope, request.name) ?? `https://${scope.host}/unknown`,
    requestHeaders: undefined,
    responseHeaders: undefined,
    requestBody: undefined,
    responseBody: undefined,
  });
  // Relative link targets resolve against a placeholder; templateUrl swaps
  // the host for the deployment either way.
  const templateHref = (value: string) => {
    try {
      return (
        templateUrl(
          scope,
          new URL(value, "https://placeholder.invalid").href,
        ) ?? MASKED_ATTRIBUTE
      );
    } catch {
      return MASKED_ATTRIBUTE;
    }
  };

  const common: SessionRecordingOptions = {
    maskAllInputs: true,
    recordHeaders: false,
    recordBody: false,
    // Canvas recording is the only part of the recorder that starts a Blob
    // worker; with it off the CSP needs no `worker-src blob:`.
    captureCanvas: { recordCanvas: false },
    maskCapturedNetworkRequestFn: maskRequest,
  };

  if (recording === "demo") {
    return {
      ...common,
      maskTextSelector: null,
      blockSelector: BLOCK_SELECTOR,
      // Links in the demo show invented data, but their paths still carry
      // IDs; they leave as route templates like every other URL.
      maskAttributeFn: (name, value) =>
        name === "href" ? templateHref(value) : value,
      sampleRate: 1,
    };
  }
  return {
    ...common,
    maskTextSelector: "*",
    blockSelector: SCHOOL_BLOCK_SELECTOR,
    maskAttributeFn: (name, value) => {
      if (name === "href") return templateHref(value);
      return SCHOOL_KEPT_ATTRIBUTES.has(name) ? value : MASKED_ATTRIBUTE;
    },
    sampleRate: recordingSampleRate(context),
  };
}

/**
 * The options that follow the context while the page runs: login, logout,
 * and a school change apply them through `set_config`, so the recorder stops
 * the moment the Analyse-Freigabe is gone from the context.
 */
export function analyticsRuntimeOptions(
  context: AnalyticsContext,
): Partial<PostHogConfig> {
  const rules = TIER_RULES[tierOf(context)];
  const recording = recordingOf(context);
  return {
    mask_all_text: !rules.elementText,
    disable_session_recording: recording === null,
    enable_recording_console_log: false,
    session_recording: recording ? recordingOptions(context, recording) : {},
  };
}

/**
 * `identified_only` on the OGS portal of a real school, where the
 * Analyse-Freigabe can arrive after the SDK started; `never` everywhere else.
 * The SDK resets this value to the init option when its remote config loads,
 * so it is set once, here. Without an `identify` call `identified_only` sends
 * no person either, and the filter forces `$process_person_profile: false`
 * outside the Freigabe.
 */
function personProfilesOf(
  context: AnalyticsContext,
): PostHogConfig["person_profiles"] {
  return context.surface === "ogs" && context.deployment !== DEMO_DEPLOYMENT
    ? "identified_only"
    : "never";
}

/** PostHog init options for a context. `before_send` comes from the client. */
export function analyticsInitOptions(
  context: AnalyticsContext,
  ownHostname: string,
): Partial<PostHogConfig> {
  return {
    api_host: POSTHOG_PROXY_PATH,
    ui_host: POSTHOG_UI_HOST,
    defaults: "2026-01-30",
    // No cookies, no local or session storage: no consent banner needed.
    persistence: "memory",
    disable_persistence: true,
    person_profiles: personProfilesOf(context),
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
    mask_all_element_attributes: true,
    capture_heatmaps: true,
    capture_dead_clicks: true,
    rageclick: true,
    // No network timing and no web vitals, in recordings neither.
    capture_performance: false,
    capture_exceptions: false,
    // The backend links its events to the browser session (#3602).
    tracing_headers: [ownHostname],
    ...analyticsRuntimeOptions(context),
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
const SNAPSHOT_EVENT = "$snapshot";
// `identify` sends `$identify`, a later change of person properties `$set`.
const PERSON_EVENTS: ReadonlySet<string> = new Set(["$identify", "$set"]);

// Core actions that end in a successful write (login_success, group_created,
// data_exported, ...) come from the backend after the write (#3602); the
// browser sends only what the backend cannot see.
const CUSTOM_EVENTS: ReadonlySet<string> = new Set([
  "login_failed",
  "tenant_switched",
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

/**
 * The only host events may name: the deployment. The real origin of the OGS
 * portal is `{slug}.{tenantDomain}` and would name the school.
 */
function analyticsHost(deployment: string): string {
  return isSafeHost(deployment) ? deployment : "unknown";
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

/** What a URL may still carry: the route list and the deployment host. */
interface UrlScope {
  readonly surface: AnalyticsRouteSurface;
  readonly host: string;
}

/**
 * Deployment host plus route template; the real origin, query, fragment, and
 * IDs never survive.
 */
function templateUrl(scope: UrlScope, value: unknown): string | undefined {
  const url = parseHttpUrl(value);
  return url
    ? `${url.protocol}//${scope.host}${templatePath(scope.surface, url.pathname)}`
    : undefined;
}

function templatePathname(scope: UrlScope, value: unknown): string | undefined {
  return typeof value === "string"
    ? templatePath(scope.surface, value)
    : undefined;
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
  scope: UrlScope,
  value: unknown,
): Record<string, HeatmapPoint[]> | undefined {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return undefined;
  }
  const byUrl: Record<string, HeatmapPoint[]> = {};
  for (const [rawUrl, points] of Object.entries(value)) {
    const url = templateUrl(scope, rawUrl);
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

function sanitizeCommon(key: string, value: unknown, scope: UrlScope): unknown {
  if (SAFE_ID_KEYS.has(key)) return isSafeId(value) ? value : undefined;
  if (SAFE_LABEL_KEYS.has(key)) return isSafeLabel(value) ? value : undefined;
  if (SAFE_NUMBER_KEYS.has(key)) {
    return isFiniteNumber(value) ? value : undefined;
  }
  if (typeof value === "string" && SAFE_VALUES[key]?.has(value)) return value;

  switch (key) {
    case "$current_url":
      return templateUrl(scope, value);
    case "$pathname":
      return templatePathname(scope, value);
    case "$referrer":
      return value === "$direct" ? value : templateUrl(scope, value);
    // Never the real host: it names the school on the OGS portal.
    case "$host":
      return isSafeHost(value) ? scope.host : undefined;
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
  scope: UrlScope,
  rules: TierRules,
): unknown {
  if (PAGE_EVENTS.has(event)) {
    if (key === "$prev_pageview_pathname") {
      return templatePathname(scope, value);
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
    return sanitizeHeatmapData(scope, value);
  }
  if (event === SNAPSHOT_EVENT) {
    return sanitizeSnapshotProperty(scope, key, value);
  }
  // The anonymous ID the page had before `identify`, never another person.
  if (event === "$identify" && key === "$anon_distinct_id") {
    return isSafeId(value) && !isPseudonym(value) ? value : undefined;
  }
  return undefined;
}

// --- $snapshot ---------------------------------------------------------------
//
// The recorder masks the page itself (recordingOptions) and passes its page
// URLs through `maskCapturedNetworkRequestFn`. The filter checks the URLs of
// the rrweb meta and custom page events once more; the DOM data passes as the
// recorder masked it, compressed or not.

const RRWEB_META = 4;
const RRWEB_CUSTOM = 5;

function templateHrefOf(scope: UrlScope, value: unknown): string {
  return templateUrl(scope, value) ?? `https://${scope.host}/unknown`;
}

function sanitizeSnapshotEvent(scope: UrlScope, entry: unknown): unknown {
  if (typeof entry !== "object" || entry === null) return entry;
  const { type, data } = entry as { type?: unknown; data?: unknown };
  if (typeof data !== "object" || data === null) return entry;

  if (type === RRWEB_META && "href" in data) {
    return {
      ...entry,
      data: { ...data, href: templateHrefOf(scope, data.href) },
    };
  }
  const payload = (data as { payload?: unknown }).payload;
  if (
    type === RRWEB_CUSTOM &&
    typeof payload === "object" &&
    payload !== null &&
    "href" in payload
  ) {
    return {
      ...entry,
      data: {
        ...data,
        payload: { ...payload, href: templateHrefOf(scope, payload.href) },
      },
    };
  }
  return entry;
}

function sanitizeSnapshotProperty(
  scope: UrlScope,
  key: string,
  value: unknown,
): unknown {
  switch (key) {
    case "$snapshot_data":
      return Array.isArray(value)
        ? value.map((entry) => sanitizeSnapshotEvent(scope, entry))
        : undefined;
    case "$snapshot_bytes":
      return isFiniteNumber(value) ? value : undefined;
    // The hostname of the recorded page; the OGS portal's names the school.
    case "$snapshot_host":
      return typeof value === "string" ? scope.host : undefined;
    default:
      return undefined;
  }
}

// --- properties and person ---------------------------------------------------

function urlScopeOf(context: AnalyticsContext): UrlScope {
  return {
    surface: routeSurfaceOf(context),
    host: analyticsHost(context.deployment),
  };
}

function sanitizeProperties(
  context: AnalyticsContext,
  event: string,
  properties: Properties,
  person: boolean,
): Properties {
  const scope = urlScopeOf(context);
  const rules = TIER_RULES[tierOf(context)];
  const safe: Properties = {};

  for (const [key, value] of Object.entries(properties)) {
    const sanitized =
      sanitizeCommon(key, value, scope) ??
      sanitizeEventSpecific(event, key, value, scope, rules);
    if (sanitized !== undefined) safe[key] = sanitized;
  }

  // Forced after filtering so neither a caller nor a registered property can
  // override them.
  safe.deployment = scope.host;
  safe.surface = isKnownSurface(context.surface) ? context.surface : "unknown";
  safe.$geoip_disable = true;
  safe.$process_person_profile = person;
  return safe;
}

/** The only person property is the role; never a name or an address. */
function sanitizePersonProperties(value: unknown): Properties | undefined {
  if (typeof value !== "object" || value === null) return undefined;
  const { role } = value as { role?: unknown };
  return typeof role === "string" && SAFE_VALUES.role?.has(role)
    ? { role }
    : undefined;
}

function isAllowedEvent(
  context: AnalyticsContext,
  event: string,
  identity: string | null,
): boolean {
  if (event === SNAPSHOT_EVENT) return recordingOf(context) !== null;
  if (PERSON_EVENTS.has(event)) return identity !== null;
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
 * A pseudonymous ID leaves only in the context that owns it: an event that
 * carries another one, or one outside the Analyse-Freigabe, is dropped whole.
 */
export function filterAnalyticsEvent(
  context: AnalyticsContext,
  captureResult: CaptureResult | null,
): CaptureResult | null {
  if (!captureResult) return null;
  const { event } = captureResult;
  const identity = analyticsIdentity(context);
  if (!isAllowedEvent(context, event, identity)) return null;

  const distinctId: unknown = captureResult.properties.distinct_id;
  if (isPseudonym(distinctId) && distinctId !== identity) return null;
  const person = identity !== null && distinctId === identity;
  if (PERSON_EVENTS.has(event) && !person) return null;

  const filtered: CaptureResult = {
    uuid: captureResult.uuid,
    event,
    timestamp: captureResult.timestamp,
    properties: sanitizeProperties(
      context,
      event,
      captureResult.properties,
      person,
    ),
  };
  if (PERSON_EVENTS.has(event)) {
    const personProperties = sanitizePersonProperties(captureResult.$set);
    if (personProperties) filtered.$set = personProperties;
  }
  return filtered;
}

import type { PostHogConfig, Properties } from "posthog-js";
import { clientEnv } from "~/env.client";
import { createLogger } from "~/lib/logger";
import { sanitizePostHogEvent } from "~/lib/posthog-privacy";

type PostHog = (typeof import("posthog-js"))["default"];
type Operation = (posthog: PostHog) => void;

const logger = createLogger({ component: "PostHogClient" });
const pendingOperations: Array<{
  readonly name: string;
  readonly run: Operation;
}> = [];

let instance: PostHog | null = null;
let initialization: Promise<void> | null = null;
let initializationFailed = false;

const postHogOptions = {
  api_host: clientEnv.NEXT_PUBLIC_POSTHOG_HOST,
  defaults: "2026-01-30",
  autocapture: false,
  rageclick: false,
  capture_pageview: false,
  capture_pageleave: false,
  capture_performance: false,
  capture_heatmaps: false,
  capture_dead_clicks: false,
  capture_exceptions: false,
  disable_scroll_properties: true,
  disable_session_recording: true,
  persistence: "memory",
  disable_persistence: true,
  person_profiles: "never",
  save_referrer: false,
  save_campaign_params: false,
  disable_surveys: true,
  advanced_disable_flags: true,
  before_send: sanitizePostHogEvent,
} satisfies Partial<PostHogConfig>;

function execute(name: string, run: Operation): void {
  if (!clientEnv.NEXT_PUBLIC_POSTHOG_KEY || initializationFailed) return;

  if (!instance) {
    pendingOperations.push({ name, run });
    return;
  }

  try {
    run(instance);
  } catch (error) {
    logger.warn("posthog_operation_failed", {
      operation: name,
      error: error instanceof Error ? error.message : String(error),
    });
  }
}

export function capturePostHog(event: string, properties?: Properties): void {
  execute("capture", (posthog) => posthog.capture(event, properties));
}

export function resetAndCapturePostHog(
  event: string,
  properties?: Properties,
): void {
  execute("reset_and_capture", (posthog) => {
    posthog.reset();
    posthog.capture(event, properties);
  });
}

export function setPostHogContext(
  properties: Properties,
  resetFirst: boolean,
): void {
  execute("set_context", (posthog) => {
    if (resetFirst) posthog.reset();
    posthog.register(properties);
  });
}

export function clearPostHogContext(): void {
  execute("clear_context", (posthog) => {
    posthog.unregister("school_id");
    posthog.unregister("$groups");
    posthog.unregister("deployment");
    posthog.reset();
  });
}

export function initializePostHog(): Promise<void> {
  const key = clientEnv.NEXT_PUBLIC_POSTHOG_KEY;
  if (!key) return Promise.resolve();
  if (initialization) return initialization;

  initialization = import("posthog-js")
    .then(({ default: posthog }) => {
      posthog.init(key, postHogOptions);
      instance = posthog;

      for (const operation of pendingOperations.splice(0)) {
        execute(operation.name, operation.run);
      }
    })
    .catch((error: unknown) => {
      initializationFailed = true;
      pendingOperations.length = 0;
      logger.warn("posthog_initialization_failed", {
        error: error instanceof Error ? error.message : String(error),
      });
    });

  return initialization;
}

export function schedulePostHogInitialization(): void {
  if (!clientEnv.NEXT_PUBLIC_POSTHOG_KEY) return;

  const initialize = () => void initializePostHog();
  if (typeof window.requestIdleCallback === "function") {
    window.requestIdleCallback(initialize, { timeout: 2_000 });
    return;
  }
  window.setTimeout(initialize, 0);
}

import type { Properties } from "posthog-js";
import { clientEnv } from "~/env.client";
import { analyticsDeployment } from "~/lib/analytics-deployment";
import {
  analyticsInitOptions,
  analyticsSurfaceForHost,
  filterAnalyticsEvent,
  type AnalyticsContext,
  type AnalyticsSurface,
} from "~/lib/analytics-policy";
import { createLogger } from "~/lib/logger";

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

// undefined: not resolved yet; null: this host has no analytics (operator).
let hostSurface: AnalyticsSurface | null | undefined;
let context: AnalyticsContext | null = null;

function startSurface(): AnalyticsSurface | null {
  if (hostSurface === undefined) {
    hostSurface = analyticsSurfaceForHost(window.location.host, {
      operator: clientEnv.NEXT_PUBLIC_OPERATOR_HOSTNAME,
      parents: clientEnv.NEXT_PUBLIC_PARENTS_HOSTNAME,
      school: clientEnv.NEXT_PUBLIC_SCHOOL_HOSTNAME,
      tenantDomain: clientEnv.NEXT_PUBLIC_TENANT_DOMAIN,
    });
  }
  return hostSurface;
}

function startContext(): AnalyticsContext | null {
  const surface = startSurface();
  if (!surface) return null;
  return {
    deployment: analyticsDeployment(),
    surface,
    analyseFreigabe: false,
  };
}

function currentContext(): AnalyticsContext | null {
  context ??= startContext();
  return context;
}

function analyticsEnabled(): boolean {
  return (
    Boolean(clientEnv.NEXT_PUBLIC_POSTHOG_KEY) &&
    !initializationFailed &&
    typeof window !== "undefined" &&
    currentContext() !== null
  );
}

function execute(name: string, run: Operation): void {
  if (!analyticsEnabled()) return;

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

/**
 * Sets the portal of the analytics context. The filter reads the context for
 * every event, so the change applies to the next event, before and after the
 * SDK has loaded.
 */
export function setAnalyticsSurface(surface: AnalyticsSurface): void {
  const current = currentContext();
  if (!current) return;
  context = { ...current, surface };
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
  if (currentContext()) context = startContext();
  execute("clear_context", (posthog) => {
    posthog.unregister("school_id");
    posthog.unregister("role");
    posthog.reset();
  });
}

export function initializePostHog(): Promise<void> {
  const key = clientEnv.NEXT_PUBLIC_POSTHOG_KEY;
  const startingContext =
    typeof window === "undefined" ? null : currentContext();
  if (!key || !startingContext) return Promise.resolve();
  if (initialization) return initialization;

  initialization = import("posthog-js")
    .then(({ default: posthog }) => {
      posthog.init(key, {
        ...analyticsInitOptions(startingContext, window.location.hostname),
        before_send: (captureResult) => {
          const active = currentContext();
          return active ? filterAnalyticsEvent(active, captureResult) : null;
        },
      });
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
  if (!analyticsEnabled()) return;

  const initialize = () => void initializePostHog();
  if (typeof window.requestIdleCallback === "function") {
    window.requestIdleCallback(initialize, { timeout: 2_000 });
    return;
  }
  window.setTimeout(initialize, 0);
}

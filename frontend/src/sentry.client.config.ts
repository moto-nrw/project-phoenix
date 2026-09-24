import * as Sentry from "@sentry/nextjs";
import { scrubEvent } from "./sentry.shared";

const dsn = process.env.NEXT_PUBLIC_SENTRY_DSN;
const environment = process.env.NEXT_PUBLIC_SENTRY_ENVIRONMENT;

/**
 * Which portal this page belongs to, so an event can be filtered by it
 * (`portal:parent`). Every portal but the tenant one has its own host.
 */
function currentPortal(): "parent" | "school" | "operator" | "tenant" {
  const host = window.location.host;
  if (host === process.env.NEXT_PUBLIC_PARENTS_HOSTNAME) return "parent";
  if (host === process.env.NEXT_PUBLIC_SCHOOL_HOSTNAME) return "school";
  if (host === process.env.NEXT_PUBLIC_OPERATOR_HOSTNAME) return "operator";
  return "tenant";
}

if (dsn) {
  Sentry.init({
    dsn,
    environment,

    tracesSampleRate: 0,
    replaysSessionSampleRate: 0,
    replaysOnErrorSampleRate: 0,

    initialScope: { tags: { portal: currentPortal() } },

    beforeSend: scrubEvent,
  });
}

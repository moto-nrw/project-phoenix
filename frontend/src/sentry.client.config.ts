import * as Sentry from "@sentry/nextjs";
import {
  sampleBrowserTrace,
  scrubClientEvent,
  scrubSpan,
  sentryDataCollection,
} from "./sentry.shared";

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
    dataCollection: sentryDataCollection,

    // Page loads, navigations and their Web Vitals for a 5 % sample. The
    // default browserTracingIntegration measures them and sends trace
    // headers to the BFF (same origin), which passes them on to the backend.
    tracesSampler: sampleBrowserTrace,
    replaysSessionSampleRate: 0,
    replaysOnErrorSampleRate: 0,

    // Tags reach error events, attributes reach the streamed spans.
    initialScope: {
      tags: { portal: currentPortal() },
      attributes: { portal: currentPortal() },
    },

    beforeSend: scrubClientEvent,
    // Spans are streamed: the page view span carries the page URL and the
    // referrer, so scrubSpan also covers what a transaction held in v10.
    beforeSendSpan: scrubSpan,
  });
}

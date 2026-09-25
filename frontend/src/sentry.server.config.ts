import * as Sentry from "@sentry/nextjs";
import { sampleNoTrace, scrubEvent } from "./sentry.shared";

const dsn = process.env.NEXT_PUBLIC_SENTRY_DSN;
const environment = process.env.NEXT_PUBLIC_SENTRY_ENVIRONMENT;

if (dsn) {
  Sentry.init({
    dsn,
    environment,

    // Only the browser measures performance. Tracing stays enabled so trace
    // headers reach the backend and error events carry the trace ID.
    tracesSampler: sampleNoTrace,

    beforeSend: scrubEvent,
  });
}

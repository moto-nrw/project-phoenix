import type { ErrorEvent } from "@sentry/nextjs";

/**
 * Shared Sentry beforeSend handler for GDPR-compliant event scrubbing.
 * Used by client, server, and edge configs to keep scrubbing rules in sync.
 */
export function scrubEvent(event: ErrorEvent): ErrorEvent {
  // Strip auth headers and cookies
  if (event.request?.headers) {
    delete event.request.headers["Authorization"];
    delete event.request.headers["authorization"];
    delete event.request.headers["Cookie"];
    delete event.request.headers["cookie"];
  }
  if (event.request?.cookies) {
    event.request.cookies = {};
  }

  // Strip PII from auto-collected user context
  if (event.user) {
    delete event.user.ip_address;
    delete event.user.email;
    delete event.user.username;
  }

  return event;
}

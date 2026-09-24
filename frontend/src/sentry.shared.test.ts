import type { ErrorEvent } from "@sentry/nextjs";
import { describe, it, expect } from "vitest";
import { scrubEvent } from "./sentry.shared";

function makeEvent(overrides: Partial<ErrorEvent> = {}): ErrorEvent {
  return {
    event_id: "test-id",
    ...overrides,
  } as ErrorEvent;
}

describe("scrubEvent", () => {
  it("strips Authorization and Cookie headers (case-sensitive variants)", () => {
    const event = makeEvent({
      request: {
        headers: {
          Authorization: "Bearer secret-token",
          authorization: "Bearer secret-token",
          Cookie: "session=abc123",
          cookie: "session=abc123",
          "Content-Type": "application/json",
        },
      },
    });

    const result = scrubEvent(event);

    expect(result).not.toBeNull();
    if (result === null) throw new Error("expected event to be kept");
    expect(result.request?.headers).toStrictEqual({
      "Content-Type": "application/json",
    });
  });

  it("clears request cookies", () => {
    const event = makeEvent({
      request: {
        cookies: { session: "abc123", token: "xyz" },
      },
    });

    const result = scrubEvent(event);

    expect(result).not.toBeNull();
    if (result === null) throw new Error("expected event to be kept");
    expect(result.request?.cookies).toStrictEqual({});
  });

  it("strips PII from user context", () => {
    const event = makeEvent({
      user: {
        id: "42",
        ip_address: "192.168.1.1",
        email: "test@example.com",
        username: "testuser",
      },
    });

    const result = scrubEvent(event);

    expect(result).not.toBeNull();
    if (result === null) throw new Error("expected event to be kept");
    expect(result.user).toStrictEqual({ id: "42" });
  });

  it("handles events with no request or user context", () => {
    const event = makeEvent({});

    const result = scrubEvent(event);

    expect(result).not.toBeNull();
    if (result === null) throw new Error("expected event to be kept");
    expect(result.event_id).toBe("test-id");
  });

  it("handles request with no headers or cookies", () => {
    const event = makeEvent({
      request: { url: "https://example.com" },
    });

    const result = scrubEvent(event);

    expect(result).not.toBeNull();
    if (result === null) throw new Error("expected event to be kept");
    expect(result.request?.url).toBe("https://example.com");
  });

  it("returns the same event reference (mutates in place)", () => {
    const event = makeEvent({ user: { id: "1", email: "a@b.com" } });

    const result = scrubEvent(event);

    expect(result).toBe(event);
  });

  it("drops the known Next.js RSC router state parse noise", () => {
    const event = makeEvent({
      exception: {
        values: [
          {
            type: "Error",
            value: "The router state header was sent but could not be parsed.",
          },
        ],
      },
      contexts: {
        nextjs: {
          request_path: "/dashboard?_rsc=tww4p",
        },
      },
      request: {
        url: "https://altenberge.moto-app.de/dashboard",
      },
    });

    const result = scrubEvent(event);

    expect(result).toBeNull();
  });

  it("keeps router state parse errors without an RSC marker", () => {
    const event = makeEvent({
      exception: {
        values: [
          {
            type: "Error",
            value: "The router state header was sent but could not be parsed.",
          },
        ],
      },
      request: {
        url: "https://altenberge.moto-app.de/dashboard",
      },
    });

    const result = scrubEvent(event);

    expect(result).toBe(event);
  });

  it("keeps unrelated RSC errors", () => {
    const event = makeEvent({
      exception: {
        values: [{ type: "Error", value: "database unavailable" }],
      },
      contexts: {
        nextjs: {
          request_path: "/dashboard?_rsc=tww4p",
        },
      },
    });

    const result = scrubEvent(event);

    expect(result).toBe(event);
  });

  it("redacts the calendar-feed token from url, transaction, path and breadcrumbs", () => {
    const event = makeEvent({
      request: { url: "https://parents.test/api/calendar-feed/secret-token" },
      transaction: "GET /api/calendar-feed/secret-token",
      contexts: { nextjs: { request_path: "/api/calendar-feed/secret-token" } },
      breadcrumbs: [
        {
          message: "fetch /public/calendar/backend-token",
          data: {
            url: "http://server:8080/public/calendar/backend-token?x=1",
          },
        },
      ],
    });

    const result = scrubEvent(event);
    if (result === null) throw new Error("expected event to be kept");

    const serialized = JSON.stringify(result);
    expect(serialized).not.toContain("secret-token");
    expect(serialized).not.toContain("backend-token");
    expect(result.request?.url).toBe(
      "https://parents.test/api/calendar-feed/[REDACTED]",
    );
    expect(result.transaction).toBe("GET /api/calendar-feed/[REDACTED]");
    expect(result.contexts?.nextjs?.request_path).toBe(
      "/api/calendar-feed/[REDACTED]",
    );
    expect(result.breadcrumbs?.[0]?.data?.url).toBe(
      "http://server:8080/public/calendar/[REDACTED]",
    );
  });

  it("strips query strings and fragments from request and breadcrumbs, keeping paths with IDs", () => {
    const event = makeEvent({
      request: {
        url: "https://schule-a.moto-app.de/students/42?search=Mia%20Muster#details",
        query_string: "search=Mia%20Muster",
        headers: {
          Referer: "https://schule-a.moto-app.de/students?search=Mia",
        },
      },
      transaction: "/students/42?search=Mia",
      contexts: { nextjs: { request_path: "/students/42?search=Mia" } },
      breadcrumbs: [
        {
          category: "navigation",
          data: { from: "/students?search=Mia", to: "/students/42?tab=notes" },
        },
        {
          category: "fetch",
          data: {
            method: "GET",
            url: "/api/students?search=Mia%20Muster",
            status_code: 500,
          },
        },
        {
          category: "log.StudentSearch",
          message: "search failed for /api/students?search=Mia",
        },
        {
          category: "log.DemoEntry",
          message: "entry https://demo.moto-app.de/demo#token=secret-demo",
        },
      ],
    });

    const result = scrubEvent(event);
    if (result === null) throw new Error("expected event to be kept");

    const serialized = JSON.stringify(result);
    expect(serialized).not.toContain("Mia");
    expect(serialized).not.toContain("secret-demo");
    expect(result.request?.url).toBe(
      "https://schule-a.moto-app.de/students/42",
    );
    expect(result.request?.query_string).toBeUndefined();
    expect(result.request?.headers?.Referer).toBe(
      "https://schule-a.moto-app.de/students",
    );
    expect(result.transaction).toBe("/students/42");
    expect(result.contexts?.nextjs?.request_path).toBe("/students/42");
    expect(result.breadcrumbs?.[0]?.data).toEqual({
      from: "/students",
      to: "/students/42",
    });
    expect(result.breadcrumbs?.[1]?.data).toEqual({
      method: "GET",
      url: "/api/students",
      status_code: 500,
    });
    expect(result.breadcrumbs?.[2]?.message).toBe(
      "search failed for /api/students",
    );
    expect(result.breadcrumbs?.[3]?.message).toBe(
      "entry https://demo.moto-app.de/demo",
    );
  });

  it("keeps a question mark that ends a sentence in a breadcrumb", () => {
    const result = scrubEvent(
      makeEvent({ breadcrumbs: [{ message: "Speichern fehlgeschlagen?" }] }),
    );

    expect(result?.breadcrumbs?.[0]?.message).toBe("Speichern fehlgeschlagen?");
  });

  it("keeps only the account ID of the user: no name, e-mail or IP", () => {
    const result = scrubEvent(
      makeEvent({
        user: {
          id: "17",
          name: "Mia Muster",
          email: "mia@example.com",
          ip_address: "203.0.113.7",
          username: "mia",
          geo: { city: "Münster" },
        },
      }),
    );

    expect(result?.user).toStrictEqual({ id: "17" });
  });

  it("redacts request-feed tokens from frontend and backend paths", () => {
    const result = scrubEvent(
      makeEvent({
        request: {
          url: "https://school.test/api/request-feed/frontend-secret",
        },
        transaction: "GET /api/request-feed/frontend-secret",
        breadcrumbs: [
          {
            message: "fetch /public/request-feed/backend-secret",
          },
        ],
      }),
    );

    expect(result?.request?.url).toBe(
      "https://school.test/api/request-feed/[REDACTED]",
    );
    expect(result?.transaction).toBe("GET /api/request-feed/[REDACTED]");
    expect(result?.breadcrumbs?.[0]?.message).toBe(
      "fetch /public/request-feed/[REDACTED]",
    );
  });

  it("leaves non-feed URLs untouched", () => {
    const event = makeEvent({
      request: { url: "https://parents.test/api/children/5" },
    });
    const result = scrubEvent(event);
    expect(result?.request?.url).toBe("https://parents.test/api/children/5");
  });
});

import type { BrowserOptions, ErrorEvent } from "@sentry/nextjs";
import { describe, it, expect } from "vitest";
import {
  pageViewTraceSampleRate,
  sampleBrowserTrace,
  sampleNoTrace,
  scrubEvent,
  scrubSpan,
  scrubTransaction,
} from "./sentry.shared";

type SpanJSON = Parameters<NonNullable<BrowserOptions["beforeSendSpan"]>>[0];
type TransactionEvent = Parameters<
  NonNullable<BrowserOptions["beforeSendTransaction"]>
>[0];
type TracesSamplingContext = Parameters<
  NonNullable<BrowserOptions["tracesSampler"]>
>[0];

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

function makeSpan(overrides: Partial<SpanJSON> = {}): SpanJSON {
  return {
    span_id: "span-id",
    trace_id: "trace-id",
    start_timestamp: 0,
    data: {},
    ...overrides,
  };
}

describe("scrubSpan", () => {
  it("strips query strings and fragments from the span name and data, keeping paths with IDs", () => {
    const span = makeSpan({
      op: "http.client",
      description: "GET /api/students/42?search=Mia%20Muster",
      data: {
        url: "/api/students/42?search=Mia%20Muster",
        "http.url": "https://schule-a.moto-app.de/api/students/42?search=Mia",
        "url.full": "https://schule-a.moto-app.de/demo#token=secret-demo",
        "http.query": "?search=Mia",
        "http.fragment": "#token=secret-demo",
        "url.query": "search=Mia",
        "url.fragment": "token=secret-demo",
        "lcp.url": "https://schule-a.moto-app.de/_next/image?url=%2Fmia.png",
        "http.response.status_code": 200,
      },
    });

    const result = scrubSpan(span);

    const serialized = JSON.stringify(result);
    expect(serialized).not.toContain("Mia");
    expect(serialized).not.toContain("secret-demo");
    expect(result.description).toBe("GET /api/students/42");
    expect(result.data).toStrictEqual({
      url: "/api/students/42",
      "http.url": "https://schule-a.moto-app.de/api/students/42",
      "url.full": "https://schule-a.moto-app.de/demo",
      "lcp.url": "https://schule-a.moto-app.de/_next/image",
      "http.response.status_code": 200,
    });
  });

  it("redacts feed tokens from page view names and fetch URLs", () => {
    const span = makeSpan({
      op: "pageload",
      description: "/public/calendar/calendar-secret",
      data: {
        url: "https://parents.test/api/request-feed/request-secret",
        "http.url": "http://server:8080/public/request-feed/backend-secret",
        "url.full": "https://parents.test/api/calendar-feed/feed-secret.ics",
      },
    });

    const result = scrubSpan(span);

    expect(JSON.stringify(result)).not.toContain("secret");
    expect(result.description).toBe("/public/calendar/[REDACTED]");
    expect(result.data).toStrictEqual({
      url: "https://parents.test/api/request-feed/[REDACTED]",
      "http.url": "http://server:8080/public/request-feed/[REDACTED]",
      "url.full": "https://parents.test/api/calendar-feed/[REDACTED]",
    });
  });

  it("leaves spans without URLs untouched", () => {
    const span = makeSpan({
      op: "ui.interaction.click",
      description: "body > button.save",
      data: { "sentry.op": "ui.interaction.click" },
    });

    expect(scrubSpan(span)).toStrictEqual(
      makeSpan({
        op: "ui.interaction.click",
        description: "body > button.save",
        data: { "sentry.op": "ui.interaction.click" },
      }),
    );
  });
});

describe("scrubTransaction", () => {
  it("strips the query string, fragment and feed token from a page view's request data", () => {
    const event = {
      type: "transaction",
      transaction: "/public/calendar/calendar-secret",
      request: {
        url: "https://schule-a.moto-app.de/students/42?search=Mia#details",
        query_string: "search=Mia",
        headers: {
          Referer: "https://schule-a.moto-app.de/students?search=Mia",
          Cookie: "session=abc123",
        },
      },
      user: { id: "17", email: "mia@example.com" },
    } as TransactionEvent;

    const result = scrubTransaction(event);

    expect(JSON.stringify(result)).not.toContain("Mia");
    expect(result.transaction).toBe("/public/calendar/[REDACTED]");
    expect(result.request).toStrictEqual({
      url: "https://schule-a.moto-app.de/students/42",
      headers: { Referer: "https://schule-a.moto-app.de/students" },
    });
    expect(result.user).toStrictEqual({ id: "17" });
  });
});

function samplingContext(
  op: string | undefined,
  parentSampled?: boolean,
): TracesSamplingContext {
  return {
    name: "span",
    attributes: op === undefined ? {} : { "sentry.op": op },
    parentSampled,
    inheritOrSampleWith: () => {
      throw new Error("the sampler must decide on its own");
    },
  };
}

describe("sampleBrowserTrace", () => {
  it("samples 5 % of page loads and navigations", () => {
    expect(pageViewTraceSampleRate).toBe(0.05);
    expect(sampleBrowserTrace(samplingContext("pageload"))).toBe(0.05);
    expect(sampleBrowserTrace(samplingContext("navigation"))).toBe(0.05);
  });

  it("decides on page views itself, whatever the server's trace says", () => {
    expect(sampleBrowserTrace(samplingContext("pageload", false))).toBe(0.05);
    expect(sampleBrowserTrace(samplingContext("navigation", true))).toBe(0.05);
  });

  it.each([
    "http.client",
    "ui.interaction.click",
    "ui.long-animation-frame",
    "resource.script",
    "function",
  ])("samples no %s span of its own", (op) => {
    expect(sampleBrowserTrace(samplingContext(op))).toBe(0);
    expect(sampleBrowserTrace(samplingContext(op, false))).toBe(0);
  });

  it("samples no span without an operation", () => {
    expect(sampleBrowserTrace(samplingContext(undefined))).toBe(0);
  });

  it("keeps the INP Web Vital of a page view that was sampled", () => {
    expect(
      sampleBrowserTrace(samplingContext("ui.interaction.click", true)),
    ).toBe(1);
  });
});

describe("sampleNoTrace", () => {
  it("samples nothing on the server, not even inside a sampled browser trace", () => {
    expect(sampleNoTrace()).toBe(0);
  });
});

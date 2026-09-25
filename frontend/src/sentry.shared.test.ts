import type { BrowserOptions, ErrorEvent } from "@sentry/nextjs";
import { describe, it, expect } from "vitest";
import {
  pageViewTraceSampleRate,
  sampleBrowserTrace,
  sampleNoTrace,
  scrubClientEvent,
  scrubEvent,
  scrubSpan,
  sentryDataCollection,
} from "./sentry.shared";

type SpanJSON = Parameters<NonNullable<BrowserOptions["beforeSendSpan"]>>[0];
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
  it("drops a client fetch network failure but preserves a normal client exception", () => {
    const network = makeEvent({
      exception: { values: [{ type: "TypeError", value: "Failed to fetch" }] },
    });
    const other = makeEvent({
      exception: {
        values: [{ type: "TypeError", value: "Invalid response format" }],
      },
    });

    expect(scrubClientEvent(network)).toBeNull();
    expect(
      scrubClientEvent(
        makeEvent({
          exception: {
            values: [{ type: "AxiosError", value: "Network Error" }],
          },
        }),
      ),
    ).toBeNull();
    expect(scrubClientEvent(other)).toBe(other);
    expect(scrubEvent(network)).toBe(network);
  });
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
    name: "span",
    start_timestamp: 0,
    status: "ok",
    is_segment: false,
    attributes: {},
    ...overrides,
  };
}

describe("scrubSpan", () => {
  it("strips query strings and fragments from the span name and attributes, keeping paths with IDs", () => {
    const span = makeSpan({
      name: "GET /api/students/42?search=Mia%20Muster",
      attributes: {
        "sentry.op": "http.client",
        url: "/api/students/42?search=Mia%20Muster",
        "http.url": "https://schule-a.moto-app.de/api/students/42?search=Mia",
        "url.full": "https://schule-a.moto-app.de/demo#token=secret-demo",
        "http.query": "?search=Mia",
        "http.fragment": "#token=secret-demo",
        "url.query": "search=Mia",
        "url.fragment": "token=secret-demo",
        "browser.web_vital.lcp.url":
          "https://schule-a.moto-app.de/_next/image?url=%2Fmia.png",
        "http.response.status_code": 200,
      },
    });

    const result = scrubSpan(span);

    const serialized = JSON.stringify(result);
    expect(serialized).not.toContain("Mia");
    expect(serialized).not.toContain("secret-demo");
    expect(result.name).toBe("GET /api/students/42");
    expect(result.attributes).toStrictEqual({
      "sentry.op": "http.client",
      url: "/api/students/42",
      "http.url": "https://schule-a.moto-app.de/api/students/42",
      "url.full": "https://schule-a.moto-app.de/demo",
      "browser.web_vital.lcp.url": "https://schule-a.moto-app.de/_next/image",
      "http.response.status_code": 200,
    });
  });

  it("redacts feed tokens from page view names and fetch URLs", () => {
    const span = makeSpan({
      name: "/public/calendar/calendar-secret",
      is_segment: true,
      attributes: {
        "sentry.op": "pageload",
        url: "https://parents.test/api/request-feed/request-secret",
        "http.url": "http://server:8080/public/request-feed/backend-secret",
        "url.full": "https://parents.test/api/calendar-feed/feed-secret.ics",
      },
    });

    const result = scrubSpan(span);

    expect(JSON.stringify(result)).not.toContain("secret");
    expect(result.name).toBe("/public/calendar/[REDACTED]");
    expect(result.attributes).toStrictEqual({
      "sentry.op": "pageload",
      url: "https://parents.test/api/request-feed/[REDACTED]",
      "http.url": "http://server:8080/public/request-feed/[REDACTED]",
      "url.full": "https://parents.test/api/calendar-feed/[REDACTED]",
    });
  });

  // The page view span carries the page URL and the referrer, which the
  // SDK sets as a list. v10 kept both in the transaction's request data.
  it("scrubs the referrer list and the page URL of a page view", () => {
    const span = makeSpan({
      name: "/students/[id]",
      is_segment: true,
      attributes: {
        "sentry.op": "navigation",
        "url.full": "https://schule-a.moto-app.de/students/42?search=Mia",
        "http.request.header.referer": [
          "https://schule-a.moto-app.de/students?search=Mia#details",
          "https://parents.test/api/calendar-feed/feed-secret",
        ],
        "browser.web_vital.cls.source": {
          value: "https://schule-a.moto-app.de/students?search=Mia",
          unit: "none",
        },
      },
    });

    const result = scrubSpan(span);

    const serialized = JSON.stringify(result);
    expect(serialized).not.toContain("Mia");
    expect(serialized).not.toContain("secret");
    expect(result.attributes).toStrictEqual({
      "sentry.op": "navigation",
      "url.full": "https://schule-a.moto-app.de/students/42",
      "http.request.header.referer": [
        "https://schule-a.moto-app.de/students",
        "https://parents.test/api/calendar-feed/[REDACTED]",
      ],
      "browser.web_vital.cls.source": {
        value: "https://schule-a.moto-app.de/students",
        unit: "none",
      },
    });
  });

  it("keeps only the account ID of the user: no name, e-mail or IP", () => {
    const span = makeSpan({
      is_segment: true,
      attributes: {
        "user.id": "17",
        "user.email": "mia@example.com",
        "user.name": "Mia Muster",
        "user.username": "mia",
        "user.ip_address": "203.0.113.7",
        "client.address": "203.0.113.7",
        "http.client_ip": "203.0.113.7",
      },
    });

    const result = scrubSpan(span);

    expect(result.attributes).toStrictEqual({ "user.id": "17" });
  });

  it("strips auth and cookie headers, whatever their case", () => {
    const span = makeSpan({
      is_segment: true,
      attributes: {
        "http.request.header.authorization": ["Bearer secret-token"],
        "http.request.header.Cookie": ["session=abc123"],
        "http.request.header.user_agent": ["Firefox"],
      },
    });

    const result = scrubSpan(span);

    expect(result.attributes).toStrictEqual({
      "http.request.header.user_agent": ["Firefox"],
    });
  });

  it("leaves spans without URLs untouched", () => {
    const span = makeSpan({
      name: "Click",
      attributes: {
        "sentry.op": "ui.interaction.click",
        "browser.web_vital.inp.target": "body > button.save",
        "browser.web_vital.inp.value": 120,
        "sentry.sdk.integrations": ["BrowserTracing", "HttpContext"],
      },
    });

    expect(scrubSpan(span)).toStrictEqual(
      makeSpan({
        name: "Click",
        attributes: {
          "sentry.op": "ui.interaction.click",
          "browser.web_vital.inp.target": "body > button.save",
          "browser.web_vital.inp.value": 120,
          "sentry.sdk.integrations": ["BrowserTracing", "HttpContext"],
        },
      }),
    );
  });
});

describe("sentryDataCollection", () => {
  // v11 collects user data (with the IP), cookies, headers, bodies and query
  // strings unless told otherwise. The data boundary of #3636 stays.
  it("collects no user data, cookies, bodies or query strings", () => {
    expect(sentryDataCollection).toStrictEqual({
      userInfo: false,
      cookies: false,
      httpHeaders: {
        request: { allow: ["user-agent", "referer"] },
        response: false,
      },
      httpBodies: [],
      urlQueryParams: false,
      graphQL: { document: false, variables: false },
      genAI: { inputs: false, outputs: false },
      databaseQueryData: false,
      queues: false,
      stackFrameVariables: false,
    });
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

  it.each([
    "http.client",
    "ui.long-animation-frame",
    "resource.script",
    "resource.css",
    "resource.img",
    "function",
  ])("samples no %s span inside a sampled page view", (op) => {
    expect(sampleBrowserTrace(samplingContext(op, true))).toBe(0);
  });

  it("samples no span without an operation", () => {
    expect(sampleBrowserTrace(samplingContext(undefined))).toBe(0);
    expect(sampleBrowserTrace(samplingContext(undefined, true))).toBe(0);
  });

  it.each([
    "ui.interaction.click",
    "ui.interaction.keyboard",
    "ui.interaction.pointer",
    "ui.interaction.drag",
    "ui.webvital.lcp",
    "ui.webvital.cls",
  ])("keeps the %s Web Vital of a page view that was sampled", (op) => {
    expect(sampleBrowserTrace(samplingContext(op, true))).toBe(1);
  });
});

describe("sampleNoTrace", () => {
  it("samples nothing on the server, not even inside a sampled browser trace", () => {
    expect(sampleNoTrace()).toBe(0);
  });
});

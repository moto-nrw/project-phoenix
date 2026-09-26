import { beforeEach, describe, expect, it, vi } from "vitest";

const { addBreadcrumb, captureMessage, captureException } = vi.hoisted(() => ({
  addBreadcrumb: vi.fn(),
  captureMessage: vi.fn(),
  captureException: vi.fn(),
}));
vi.mock("@sentry/nextjs", () => ({
  addBreadcrumb,
  captureMessage,
  captureException,
}));

vi.unmock("~/lib/logger");

const { reportLogToSentry } = await import("./logger-sentry");
const { createLogger } = await import("./logger");

function entry(overrides: Record<string, unknown> = {}) {
  return {
    timestamp: "2026-09-23T14:15:08.000Z",
    level: "error" as const,
    msg: "parent_news_poll_answer_failed",
    component: "ParentNews",
    environment: "production" as const,
    context: "client" as const,
    ...overrides,
  };
}

describe("reportLogToSentry", () => {
  beforeEach(() => vi.clearAllMocks());

  it("records an error as a breadcrumb without creating an event", () => {
    reportLogToSentry(entry({ error: "Invalid response format" }));

    expect(addBreadcrumb).toHaveBeenCalledWith({
      category: "log.ParentNews",
      message: "parent_news_poll_answer_failed",
      level: "error",
      data: { error: "Invalid response format" },
    });
    expect(captureMessage).not.toHaveBeenCalled();
    expect(captureException).not.toHaveBeenCalled();
  });

  it("keeps debug out of breadcrumbs and records server errors only as breadcrumbs", () => {
    reportLogToSentry(entry({ level: "debug" }));
    reportLogToSentry(entry({ context: "server" }));

    expect(addBreadcrumb).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({ level: "error" }),
    );
    expect(captureMessage).not.toHaveBeenCalled();
    expect(captureException).not.toHaveBeenCalled();
  });

  it("treats a client network failure as a warning breadcrumb, not an event", () => {
    createLogger({ component: "Probe" }).error("request_failed", {
      error: "TypeError: Failed to fetch",
    });

    expect(addBreadcrumb).toHaveBeenCalledWith(
      expect.objectContaining({
        category: "log.Probe",
        level: "warning",
        data: expect.objectContaining({
          expected_failure: "network",
        }) as unknown,
      }),
    );
    expect(captureMessage).not.toHaveBeenCalled();
    expect(captureException).not.toHaveBeenCalled();
  });
});

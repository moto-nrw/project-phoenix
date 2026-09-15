import { describe, it, expect, vi, beforeEach } from "vitest";

const mocks = vi.hoisted(() => ({
  capture: vi.fn(),
  resetAndCapture: vi.fn(),
}));
const mockEnv = vi.hoisted(() => ({
  NEXT_PUBLIC_TENANT_DOMAIN: "localhost",
}));

vi.mock("~/lib/posthog-client", () => ({
  capturePostHog: mocks.capture,
  resetAndCapturePostHog: mocks.resetAndCapture,
}));

vi.mock("~/env.client", () => ({ clientEnv: mockEnv }));

import { trackEvent, trackPageView, trackTenantEvent } from "./analytics";

describe("trackEvent", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("forwards event name and props to the PostHog client", () => {
    trackEvent("data_exported", { format: "xlsx" });

    expect(mocks.capture).toHaveBeenCalledWith("data_exported", {
      format: "xlsx",
    });
  });

  it("captures allowlisted page views without an account identifier", () => {
    trackPageView("/students/:id", "42");

    expect(mocks.capture).toHaveBeenCalledWith("page_viewed", {
      view_id: "/students/:id",
      portal: "tenant",
      deployment: "localhost",
      school_id: "42",
      $groups: { school: "42" },
      $geoip_disable: true,
      $process_person_profile: false,
    });
  });

  it("attaches trusted school context directly to pre-session events", () => {
    trackTenantEvent("login_failed", "42", {
      reason: "invalid_credentials",
    });

    expect(mocks.capture).toHaveBeenCalledWith("login_failed", {
      reason: "invalid_credentials",
      deployment: "localhost",
      school_id: "42",
      $groups: { school: "42" },
    });
  });

  it("rejects tenant events without a numeric school ID", () => {
    trackTenantEvent("login_success", "school-a");

    expect(mocks.capture).not.toHaveBeenCalled();
  });

  it("resets identity before capturing a completed tenant switch", () => {
    trackTenantEvent("tenant_switched", "42");

    expect(mocks.resetAndCapture).toHaveBeenCalledWith("tenant_switched", {
      deployment: "localhost",
      school_id: "42",
      $groups: { school: "42" },
    });
    expect(mocks.capture).not.toHaveBeenCalled();
  });
});

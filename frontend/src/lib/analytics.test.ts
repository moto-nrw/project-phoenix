import { describe, it, expect, vi, beforeEach } from "vitest";

const mocks = vi.hoisted(() => ({
  capture: vi.fn(),
  resetAndCapture: vi.fn(),
  setAnalyticsContext: vi.fn(),
  setContext: vi.fn(),
  clearContext: vi.fn(),
}));
const mockEnv = vi.hoisted(() => ({
  NEXT_PUBLIC_TENANT_DOMAIN: "localhost",
}));

vi.mock("~/lib/posthog-client", () => ({
  capturePostHog: mocks.capture,
  resetAndCapturePostHog: mocks.resetAndCapture,
  setAnalyticsContext: mocks.setAnalyticsContext,
  setPostHogContext: mocks.setContext,
  clearPostHogContext: mocks.clearContext,
}));

vi.mock("~/env.client", () => ({ clientEnv: mockEnv }));

import {
  clearPortalSession,
  ogsAnalyticsRole,
  registerPortalSession,
  trackEvent,
  trackTenantEvent,
} from "./analytics";

describe("trackEvent", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("forwards event name and props to the PostHog client", () => {
    trackEvent("login_failed", { reason: "error" });

    expect(mocks.capture).toHaveBeenCalledWith("login_failed", {
      reason: "error",
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
    });
  });

  it("rejects tenant events without a numeric school ID", () => {
    trackTenantEvent("login_failed", "school-a");

    expect(mocks.capture).not.toHaveBeenCalled();
  });

  it("resets identity before capturing a completed tenant switch", () => {
    trackTenantEvent("tenant_switched", "42");

    expect(mocks.resetAndCapture).toHaveBeenCalledWith("tenant_switched", {
      deployment: "localhost",
      school_id: "42",
    });
    expect(mocks.capture).not.toHaveBeenCalled();
  });
});

describe("portal sessions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("registers surface, school, and role, never an account", () => {
    registerPortalSession(
      { surface: "school", schoolId: "42", role: "lehrkraft" },
      false,
    );

    // Outside the OGS portal no Freigabe and no person reach the context.
    expect(mocks.setAnalyticsContext).toHaveBeenCalledWith({
      surface: "school",
      role: "lehrkraft",
      analyseFreigabe: false,
      recordingSamplePercent: 0,
      person: null,
    });
    expect(mocks.setContext).toHaveBeenCalledWith(
      { school_id: "42", role: "lehrkraft" },
      false,
    );
  });

  it("hands the OGS portal's Analyse-Freigabe and person to the context", () => {
    registerPortalSession(
      {
        surface: "ogs",
        schoolId: "42",
        role: "staff",
        analyseFreigabe: true,
        recordingSamplePercent: 50,
        person: "pseudo_4b9630678fc1afce76ce690721aaf949",
      },
      false,
    );

    expect(mocks.setAnalyticsContext).toHaveBeenCalledWith({
      surface: "ogs",
      role: "staff",
      analyseFreigabe: true,
      recordingSamplePercent: 50,
      person: "pseudo_4b9630678fc1afce76ce690721aaf949",
    });
    // The registered properties stay school and role; the person is the
    // distinct ID, never a property.
    expect(mocks.setContext).toHaveBeenCalledWith(
      { school_id: "42", role: "staff" },
      false,
    );
  });

  it("registers a parents session without a school", () => {
    registerPortalSession(
      { surface: "parents", schoolId: null, role: "guardian" },
      true,
    );

    expect(mocks.setContext).toHaveBeenCalledWith({ role: "guardian" }, true);
  });

  it("clears the context at logout", () => {
    clearPortalSession();

    expect(mocks.clearContext).toHaveBeenCalledOnce();
  });

  it.each([
    [{}, "staff"],
    [{ isAdmin: true }, "admin"],
    [{ isAdmin: false, isPreview: true }, "admin"],
  ])("maps the OGS user %o to the role %s", (user, role) => {
    expect(ogsAnalyticsRole(user)).toBe(role);
  });
});

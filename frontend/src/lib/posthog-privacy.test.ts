import { describe, expect, it } from "vitest";
import { sanitizePostHogEvent } from "./posthog-privacy";

describe("sanitizePostHogEvent", () => {
  it("removes browser, URL, and person properties from allowed events", () => {
    const result = sanitizePostHogEvent({
      uuid: "018f47ac-10b5-7c3d-9d3c-0123456789ab",
      event: "page_viewed",
      properties: {
        token: "phc_test",
        distinct_id: "anonymous-runtime-id",
        view_id: "/students/:id",
        portal: "tenant",
        deployment: "moto-app.de",
        school_id: "42",
        $groups: { school: "42" },
        $current_url: "https://school.example/students/123?token=secret",
        $pathname: "/students/123",
        $referrer: "https://school.example/dashboard",
        $raw_user_agent: "browser fingerprint",
        email: "person@example.com",
      },
      $set: { email: "person@example.com" },
    });

    expect(result).toEqual({
      uuid: "018f47ac-10b5-7c3d-9d3c-0123456789ab",
      event: "page_viewed",
      timestamp: undefined,
      properties: {
        token: "phc_test",
        distinct_id: "anonymous-runtime-id",
        view_id: "/students/:id",
        portal: "tenant",
        deployment: "moto-app.de",
        school_id: "42",
        $groups: { school: "42" },
        $geoip_disable: true,
        $process_person_profile: false,
      },
    });
  });

  it("drops SDK-generated and unknown events", () => {
    expect(
      sanitizePostHogEvent({
        uuid: "018f47ac-10b5-7c3d-9d3c-0123456789ab",
        event: "$pageview",
        properties: { token: "phc_test", distinct_id: "id" },
      }),
    ).toBeNull();
  });

  it("drops unrecognized values even for allowlisted property names", () => {
    const result = sanitizePostHogEvent({
      uuid: "018f47ac-10b5-7c3d-9d3c-0123456789ab",
      event: "login_failed",
      properties: {
        token: "phc_test",
        distinct_id: "id",
        reason: "person@example.com",
      },
    });

    expect(result?.properties).not.toHaveProperty("reason");
  });
});

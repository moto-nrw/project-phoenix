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

  it("keeps the public demo's events with the demo access as identity", () => {
    for (const event of [
      "demo_entered",
      "demo_role_switched",
      "demo_restarted",
      "demo_start_clicked",
    ]) {
      const result = sanitizePostHogEvent({
        uuid: "018f47ac-10b5-7c3d-9d3c-0123456789ab",
        event,
        properties: {
          token: "phc_test",
          distinct_id: "4711",
          deployment: "demo",
          src: "messe",
          demo_role: "lead",
          email: "kim@ogs-beispiel.de",
          person_name: "Kim Beispiel",
        },
      });

      expect(result?.properties).toEqual({
        token: "phc_test",
        distinct_id: "4711",
        deployment: "demo",
        src: "messe",
        demo_role: "lead",
        $geoip_disable: true,
        $process_person_profile: false,
      });
    }
  });

  // The parents app of the demo (#3468) reports its role like the others.
  it("keeps the demo role parent", () => {
    const result = sanitizePostHogEvent({
      uuid: "018f47ac-10b5-7c3d-9d3c-0123456789ab",
      event: "demo_role_switched",
      properties: {
        token: "phc_test",
        distinct_id: "4711",
        demo_role: "parent",
        school_name: "OGS Beispiel",
      },
    });

    expect(result?.properties).toMatchObject({ demo_role: "parent" });
    expect(result?.properties).not.toHaveProperty("school_name");
  });

  it("drops a demo source or role that is not a plain label", () => {
    const result = sanitizePostHogEvent({
      uuid: "018f47ac-10b5-7c3d-9d3c-0123456789ab",
      event: "demo_entered",
      properties: {
        token: "phc_test",
        distinct_id: "4711",
        src: "kim@ogs-beispiel.de",
        demo_role: "operator",
      },
    });

    expect(result?.properties).not.toHaveProperty("src");
    expect(result?.properties).not.toHaveProperty("demo_role");
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

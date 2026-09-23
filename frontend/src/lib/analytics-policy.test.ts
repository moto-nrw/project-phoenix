import type { CaptureResult, Properties } from "posthog-js";
import { describe, expect, it } from "vitest";
import {
  analyticsInitOptions,
  analyticsSurfaceForHost,
  filterAnalyticsEvent,
  type AnalyticsContext,
} from "./analytics-policy";

const UUID = "018f47ac-10b5-7c3d-9d3c-0123456789ab";
const SESSION_ID = "0199aa2b-1c2d-7e3f-8a9b-0123456789ab";
const WINDOW_ID = "0199aa2b-1c2d-7e3f-8a9b-ba9876543210";

function capture(event: string, properties: Properties): CaptureResult {
  return {
    uuid: UUID,
    event,
    properties: {
      token: "phc_test",
      distinct_id: "0199aa2b-anon",
      $session_id: SESSION_ID,
      $window_id: WINDOW_ID,
      ...properties,
    },
  };
}

// Every context the analytics module knows, plus an unknown surface.
const CONTEXTS: ReadonlyArray<{
  readonly name: string;
  readonly context: AnalyticsContext;
  readonly elementText: boolean;
  readonly surface: string;
  readonly origin: string;
  readonly page: string;
  readonly template: string;
}> = [
  {
    name: "demo",
    context: { deployment: "demo", surface: "ogs", analyseFreigabe: false },
    elementText: true,
    surface: "ogs",
    origin: "https://ogs-demo.demo.moto-app.de",
    page: "/students/4711/room-history",
    template: "/students/:id/room-history",
  },
  {
    name: "OGS portal",
    context: {
      deployment: "moto-app.de",
      surface: "ogs",
      analyseFreigabe: false,
    },
    elementText: false,
    surface: "ogs",
    origin: "https://school-a.moto-app.de",
    page: "/students/4711",
    template: "/students/:id",
  },
  {
    name: "OGS portal with Analyse-Freigabe",
    context: {
      deployment: "moto-app.de",
      surface: "ogs",
      analyseFreigabe: true,
    },
    elementText: false,
    surface: "ogs",
    origin: "https://school-a.moto-app.de",
    page: "/messages/88",
    template: "/messages/:threadId",
  },
  {
    name: "parents portal",
    context: {
      deployment: "moto-app.de",
      surface: "parents",
      analyseFreigabe: true,
    },
    elementText: false,
    surface: "parents",
    origin: "https://eltern.moto-app.de",
    page: "/children/4711",
    template: "/children/:id",
  },
  {
    name: "school portal",
    context: {
      deployment: "moto-app.de",
      surface: "school",
      analyseFreigabe: true,
    },
    elementText: false,
    surface: "school",
    origin: "https://schule.moto-app.de",
    page: "/nachrichten/88",
    template: "/nachrichten/:threadID",
  },
  {
    name: "unknown surface",
    context: {
      deployment: "demo",
      surface: "kiosk",
      analyseFreigabe: true,
    },
    elementText: false,
    surface: "unknown",
    origin: "https://school-a.moto-app.de",
    page: "/students/4711",
    template: "/students/:id",
  },
];

describe.each(CONTEXTS)(
  "analytics rules: $name",
  ({ context, elementText, surface, origin, page, template }) => {
    // Sent URLs name the deployment; the origin of the OGS portal carries the
    // school slug and must never leave the browser.
    const sentOrigin = `https://${context.deployment}`;

    it("configures anonymous capture without recording or persistence", () => {
      const options = analyticsInitOptions(context, "school-a.moto-app.de");

      expect(options).toMatchObject({
        api_host: "/ingest",
        ui_host: "https://eu.posthog.com",
        persistence: "memory",
        disable_persistence: true,
        person_profiles: "never",
        advanced_disable_feature_flags: true,
        capture_pageview: "history_change",
        capture_pageleave: true,
        autocapture: true,
        mask_all_text: !elementText,
        mask_all_element_attributes: true,
        capture_heatmaps: true,
        capture_dead_clicks: true,
        rageclick: true,
        capture_exceptions: false,
        disable_session_recording: true,
        disable_surveys: true,
        tracing_headers: ["school-a.moto-app.de"],
      });
      expect(options).not.toHaveProperty("advanced_disable_flags");
    });

    it("drops every session recording snapshot", () => {
      expect(
        filterAnalyticsEvent(
          context,
          capture("$snapshot", {
            $snapshot_data: [{ type: 2, data: { href: `${origin}${page}` } }],
          }),
        ),
      ).toBeNull();
    });

    it("rewrites page URLs to the route template and forces person flags", () => {
      const result = filterAnalyticsEvent(
        context,
        capture("$pageview", {
          $current_url: `${origin}${page}?student=Kim#notes`,
          $pathname: page,
          $host: new URL(origin).host,
          $referrer: `${origin}${page}?from=search`,
          $prev_pageview_pathname: page,
          $prev_pageview_duration: 12.5,
          $prev_pageview_max_scroll_percentage: 0.8,
          title: "Kim Beispiel – Kinderakte",
          $raw_user_agent: "browser fingerprint",
          $initial_current_url: `${origin}${page}`,
          $process_person_profile: true,
          $set: { email: "kim@example.org" },
        }),
      );

      expect(result?.properties).toEqual({
        token: "phc_test",
        distinct_id: "0199aa2b-anon",
        $session_id: SESSION_ID,
        $window_id: WINDOW_ID,
        $current_url: `${sentOrigin}${template}`,
        $pathname: template,
        $host: context.deployment,
        $referrer: `${sentOrigin}${template}`,
        $prev_pageview_pathname: template,
        $prev_pageview_duration: 12.5,
        $prev_pageview_max_scroll_percentage: 0.8,
        deployment: context.deployment,
        surface,
        $geoip_disable: true,
        $process_person_profile: false,
      });
      expect(JSON.stringify(result?.properties)).not.toContain(
        new URL(origin).host,
      );
    });

    it("sends a page without a template as /unknown", () => {
      const result = filterAnalyticsEvent(
        context,
        capture("$pageleave", {
          $current_url: `${origin}/unreviewed/4711`,
          $pathname: "/unreviewed/4711",
        }),
      );

      expect(result?.properties).toMatchObject({
        $current_url: `${sentOrigin}/unknown`,
        $pathname: "/unknown",
      });
    });

    it("keeps clicks without element text, attributes, or link targets", () => {
      const result = filterAnalyticsEvent(
        context,
        capture("$autocapture", {
          $event_type: "click",
          $ce_version: 1,
          $el_text: "Kim Beispiel",
          $external_click_url: "https://example.org/kim",
          $elements_chain:
            'a.flex.hover:bg-gray-50:attr__href="/students/4711"href="/students/4711"nth-child="2"nth-of-type="1"text="Kim Beispiel";li:nth-child="3"nth-of-type="3";ul.divide-y:attr__aria-label="Kinder von Frau Muster"nth-child="1"nth-of-type="1"',
          $element_selectors: ["#student-4711"],
          student_id: "4711",
        }),
      );

      const chain = result?.properties.$elements_chain as string;
      expect(chain).toBe(
        elementText
          ? 'a.flex.hover:bg-gray-50:nth-child="2"nth-of-type="1"text="Kim Beispiel";li:nth-child="3"nth-of-type="3";ul.divide-y:nth-child="1"nth-of-type="1"'
          : 'a.flex.hover:bg-gray-50:nth-child="2"nth-of-type="1";li:nth-child="3"nth-of-type="3";ul.divide-y:nth-child="1"nth-of-type="1"',
      );
      expect(chain).not.toContain("4711");
      expect(chain).not.toContain("Muster");
      expect(result?.properties).toMatchObject({
        $event_type: "click",
        $ce_version: 1,
      });
      expect(result?.properties).not.toHaveProperty("$external_click_url");
      expect(result?.properties).not.toHaveProperty("$element_selectors");
      expect(result?.properties).not.toHaveProperty("student_id");
      if (elementText) {
        expect(result?.properties.$el_text).toBe("Kim Beispiel");
      } else {
        expect(result?.properties).not.toHaveProperty("$el_text");
      }
    });

    it("drops an element chain that does not parse", () => {
      const result = filterAnalyticsEvent(
        context,
        capture("$rageclick", {
          $elements_chain: 'a:href="/students/4711\\"nth-child="1"',
        }),
      );

      expect(result).not.toBeNull();
      expect(result?.properties).not.toHaveProperty("$elements_chain");
    });

    it("keeps dead click timings", () => {
      const result = filterAnalyticsEvent(
        context,
        capture("$dead_click", {
          $dead_click_scroll_delay_ms: 2500,
          $dead_click_absolute_timeout: true,
          $dead_click_selection_changed_delay_ms: "Kim",
        }),
      );

      expect(result?.properties).toMatchObject({
        $dead_click_scroll_delay_ms: 2500,
        $dead_click_absolute_timeout: true,
      });
      expect(result?.properties).not.toHaveProperty(
        "$dead_click_selection_changed_delay_ms",
      );
    });

    it("rewrites heatmap URLs to the route template", () => {
      const result = filterAnalyticsEvent(
        context,
        capture("$$heatmap", {
          $heatmap_data: {
            [`${origin}${page}?tab=akte`]: [
              { x: 10, y: 20, target_fixed: false, type: "click" },
              { x: "Kim", y: 20, type: "click" },
            ],
            [`${origin}${page}`]: [
              { x: 30, y: 40, target_fixed: true, type: "rageclick" },
            ],
          },
        }),
      );

      expect(result?.properties.$heatmap_data).toEqual({
        [`${sentOrigin}${template}`]: [
          { x: 10, y: 20, target_fixed: false, type: "click" },
          { x: 30, y: 40, target_fixed: true, type: "rageclick" },
        ],
      });
    });
  },
);

describe("filterAnalyticsEvent", () => {
  const ogs: AnalyticsContext = {
    deployment: "moto-app.de",
    surface: "ogs",
    analyseFreigabe: false,
  };

  it("drops unknown SDK and custom events", () => {
    for (const event of [
      "$identify",
      "$exception",
      "$web_vitals",
      "$feature_flag_called",
      "survey shown",
      "page_viewed",
      "student_opened",
    ]) {
      expect(filterAnalyticsEvent(ogs, capture(event, {}))).toBeNull();
    }
  });

  it("keeps school and role, drops groups and person properties", () => {
    const result = filterAnalyticsEvent(
      ogs,
      capture("group_created", {
        school_id: "42",
        role: "admin",
        $groups: { school: "42" },
        email: "person@example.com",
        deployment: "attacker.example",
        surface: "operator",
      }),
    );

    expect(result).toEqual({
      uuid: UUID,
      event: "group_created",
      timestamp: undefined,
      properties: {
        token: "phc_test",
        distinct_id: "0199aa2b-anon",
        $session_id: SESSION_ID,
        $window_id: WINDOW_ID,
        school_id: "42",
        role: "admin",
        deployment: "moto-app.de",
        surface: "ogs",
        $geoip_disable: true,
        $process_person_profile: false,
      },
    });
  });

  it("drops a free-text role or a non-numeric school", () => {
    const result = filterAnalyticsEvent(
      ogs,
      capture("login_success", { role: "OGS-Leitung", school_id: "school-a" }),
    );

    expect(result?.properties).not.toHaveProperty("role");
    expect(result?.properties).not.toHaveProperty("school_id");
  });

  it("keeps the public demo's events with the demo access as identity", () => {
    const demo: AnalyticsContext = {
      deployment: "demo",
      surface: "public",
      analyseFreigabe: false,
    };
    for (const event of [
      "demo_entered",
      "demo_role_switched",
      "demo_restarted",
      "demo_start_clicked",
    ]) {
      const result = filterAnalyticsEvent(demo, {
        uuid: UUID,
        event,
        properties: {
          token: "phc_test",
          distinct_id: "4711",
          deployment: "demo",
          src: "messe",
          demo_role: "parent",
          email: "kim@ogs-beispiel.de",
          person_name: "Kim Beispiel",
        },
      });

      expect(result?.properties).toEqual({
        token: "phc_test",
        distinct_id: "4711",
        deployment: "demo",
        surface: "public",
        src: "messe",
        demo_role: "parent",
        $geoip_disable: true,
        $process_person_profile: false,
      });
    }
  });

  it("drops a demo source or role that is not a plain label", () => {
    const result = filterAnalyticsEvent(
      { deployment: "demo", surface: "public", analyseFreigabe: false },
      capture("demo_entered", {
        src: "kim@ogs-beispiel.de",
        demo_role: "operator",
      }),
    );

    expect(result?.properties).not.toHaveProperty("src");
    expect(result?.properties).not.toHaveProperty("demo_role");
  });

  it("drops unrecognized values even for allowlisted property names", () => {
    const result = filterAnalyticsEvent(
      ogs,
      capture("login_failed", {
        reason: "person@example.com",
        $session_id: "Kim Beispiel <kim@example.org>",
      }),
    );

    expect(result?.properties).not.toHaveProperty("reason");
    expect(result?.properties).not.toHaveProperty("$session_id");
  });

  it("keeps the direct referrer marker and drops non-HTTP URLs", () => {
    const result = filterAnalyticsEvent(
      ogs,
      capture("$pageview", {
        $referrer: "$direct",
        $current_url: "javascript:alert(1)",
      }),
    );

    expect(result?.properties.$referrer).toBe("$direct");
    expect(result?.properties).not.toHaveProperty("$current_url");
  });

  it("drops a null event", () => {
    expect(filterAnalyticsEvent(ogs, null)).toBeNull();
  });
});

describe("analyticsSurfaceForHost", () => {
  const hosts = {
    operator: "operator.moto-app.de",
    parents: "eltern.moto-app.de",
    school: "schule.moto-app.de",
    tenantDomain: "moto-app.de",
  };

  it.each([
    ["school-a.moto-app.de", "ogs"],
    ["eltern.moto-app.de", "parents"],
    ["schule.moto-app.de", "school"],
    ["moto-app.de", "public"],
    ["operator.moto-app.de", null],
  ])("maps %s to %s", (host, surface) => {
    expect(analyticsSurfaceForHost(host, hosts)).toBe(surface);
  });
});

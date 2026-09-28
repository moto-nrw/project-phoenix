import type { CaptureResult, Properties } from "posthog-js";
import { describe, expect, it } from "vitest";
import {
  ANALYTICS_BLOCK_ATTRIBUTE,
  analyticsIdentity,
  analyticsInitOptions,
  analyticsRuntimeOptions,
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

// Every context the analytics module knows, plus an unknown surface. Only the
// demo and the OGS portal with Analyse-Freigabe record (#3603); a person
// exists only in the latter.
const PERSON = "pseudo_4b9630678fc1afce76ce690721aaf949";
const OTHER_PERSON = "pseudo_0123456789abcdef0123456789abcdef";

const CONTEXTS: ReadonlyArray<{
  readonly name: string;
  readonly context: AnalyticsContext;
  readonly elementText: boolean;
  readonly recording: "demo" | "school" | null;
  readonly person: boolean;
  readonly personProfiles: "never" | "identified_only";
  readonly surface: string;
  readonly origin: string;
  readonly page: string;
  readonly template: string;
}> = [
  {
    name: "demo",
    context: { deployment: "demo", surface: "ogs", analyseFreigabe: false },
    elementText: true,
    recording: "demo",
    person: false,
    personProfiles: "never",
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
      recordingSamplePercent: 100,
      person: PERSON,
    },
    elementText: false,
    recording: null,
    person: false,
    personProfiles: "identified_only",
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
      recordingSamplePercent: 100,
      person: PERSON,
      role: "staff",
    },
    elementText: false,
    recording: "school",
    person: true,
    personProfiles: "identified_only",
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
      recordingSamplePercent: 100,
      person: PERSON,
    },
    elementText: false,
    recording: null,
    person: false,
    personProfiles: "never",
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
      recordingSamplePercent: 100,
      person: PERSON,
    },
    elementText: false,
    recording: null,
    person: false,
    personProfiles: "never",
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
      recordingSamplePercent: 100,
      person: PERSON,
    },
    elementText: false,
    recording: null,
    person: false,
    personProfiles: "never",
    surface: "unknown",
    origin: "https://school-a.moto-app.de",
    page: "/students/4711",
    template: "/students/:id",
  },
];

describe.each(CONTEXTS)(
  "analytics rules: $name",
  ({
    context,
    elementText,
    recording,
    person,
    personProfiles,
    surface,
    origin,
    page,
    template,
  }) => {
    // Sent URLs name the deployment; the origin of the OGS portal carries the
    // school slug and must never leave the browser.
    const sentOrigin = `https://${context.deployment}`;

    it("configures capture, recording, and persistence for the context", () => {
      const options = analyticsInitOptions(context, "school-a.moto-app.de");

      expect(options).toMatchObject({
        api_host: "/ingest",
        ui_host: "https://eu.posthog.com",
        persistence: "memory",
        disable_persistence: true,
        person_profiles: personProfiles,
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
        capture_performance: false,
        disable_session_recording: recording === null,
        enable_recording_console_log: false,
        disable_surveys: true,
        tracing_headers: ["school-a.moto-app.de"],
      });
      expect(options).not.toHaveProperty("advanced_disable_flags");
      expect(analyticsRuntimeOptions(context)).toMatchObject({
        disable_session_recording: recording === null,
      });
    });

    it(
      recording
        ? "keeps its recording snapshots with template URLs"
        : "drops every session recording snapshot",
      () => {
        const result = filterAnalyticsEvent(
          context,
          capture("$snapshot", {
            $snapshot_data: [
              {
                type: 4,
                data: { href: `${origin}${page}?tab=akte`, width: 1 },
              },
              { type: 2, data: { node: { id: 1 } }, cv: "2024-10" },
              {
                type: 5,
                data: {
                  tag: "$pageview",
                  payload: { href: `${origin}${page}` },
                },
              },
            ],
            $snapshot_bytes: 1200,
            $snapshot_host: new URL(origin).hostname,
            $lib: "web",
          }),
        );

        if (!recording) {
          expect(result).toBeNull();
          return;
        }
        expect(result?.properties).toMatchObject({
          $snapshot_data: [
            { type: 4, data: { href: `${sentOrigin}${template}`, width: 1 } },
            { type: 2, data: { node: { id: 1 } }, cv: "2024-10" },
            {
              type: 5,
              data: {
                tag: "$pageview",
                payload: { href: `${sentOrigin}${template}` },
              },
            },
          ],
          $snapshot_bytes: 1200,
          $snapshot_host: context.deployment,
          $session_id: SESSION_ID,
          $window_id: WINDOW_ID,
        });
        expect(JSON.stringify(result)).not.toContain(new URL(origin).host);
      },
    );

    it(
      person
        ? "identifies the pseudonymous person with the role only"
        : "never sends a person",
      () => {
        const identify = filterAnalyticsEvent(context, {
          ...capture("$identify", {
            distinct_id: PERSON,
            $anon_distinct_id: "0199aa2b-anon",
          }),
          $set: { role: "staff", email: "kim@example.org" },
          $set_once: { $initial_referrer: "https://example.org" },
        });
        const pageview = filterAnalyticsEvent(
          context,
          capture("$pageview", { distinct_id: PERSON }),
        );

        if (!person) {
          expect(identify).toBeNull();
          expect(pageview).toBeNull();
          return;
        }
        expect(identify?.$set).toEqual({ role: "staff" });
        expect(identify).not.toHaveProperty("$set_once");
        expect(identify?.properties).toMatchObject({
          distinct_id: PERSON,
          $anon_distinct_id: "0199aa2b-anon",
          $process_person_profile: true,
        });
        expect(pageview?.properties).toMatchObject({
          distinct_id: PERSON,
          $process_person_profile: true,
        });
      },
    );

    it("drops an event of another pseudonymous person", () => {
      expect(
        filterAnalyticsEvent(
          context,
          capture("$pageview", { distinct_id: OTHER_PERSON }),
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

  // The backend sends these after the write succeeded (#3602); a browser
  // copy would count every action twice.
  it("drops core actions that the backend sends", () => {
    for (const event of [
      "login_success",
      "group_created",
      "group_updated",
      "user_invited",
      "data_exported",
    ]) {
      expect(filterAnalyticsEvent(ogs, capture(event, {}))).toBeNull();
    }
  });

  it("keeps school and role, drops groups and person properties", () => {
    const result = filterAnalyticsEvent(
      ogs,
      capture("tenant_switched", {
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
      event: "tenant_switched",
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
      capture("login_failed", { role: "OGS-Leitung", school_id: "school-a" }),
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

describe("session recording", () => {
  const freigabe: AnalyticsContext = {
    deployment: "moto-app.de",
    surface: "ogs",
    analyseFreigabe: true,
    recordingSamplePercent: 30,
    person: PERSON,
  };
  const demo: AnalyticsContext = {
    deployment: "demo",
    surface: "public",
    analyseFreigabe: false,
  };
  const recordingOf = (context: AnalyticsContext) =>
    analyticsRuntimeOptions(context).session_recording ?? {};

  it("masks every text, input, image, and content attribute with the Freigabe", () => {
    const options = recordingOf(freigabe);

    expect(options).toMatchObject({
      maskAllInputs: true,
      maskTextSelector: "*",
      recordHeaders: false,
      recordBody: false,
      captureCanvas: { recordCanvas: false },
      sampleRate: 0.3,
    });
    for (const blocked of ["img", "video", "canvas", "svg image", "iframe"]) {
      expect(options.blockSelector).toContain(blocked);
    }
    const maskAttribute = options.maskAttributeFn;
    expect(maskAttribute?.("alt", "Kim Beispiel")).toBe("*");
    expect(maskAttribute?.("title", "Kim Beispiel")).toBe("*");
    expect(maskAttribute?.("aria-label", "Kim Beispiel")).toBe("*");
    expect(maskAttribute?.("src", "/api/students/4711/photo")).toBe("*");
    expect(maskAttribute?.("class", "flex gap-2")).toBe("flex gap-2");
    expect(maskAttribute?.("href", "/students/4711?tab=akte")).toBe(
      "https://moto-app.de/students/:id",
    );
  });

  it("masks inputs and blocks the /start form in the demo", () => {
    const options = recordingOf(demo);

    expect(options).toMatchObject({
      maskAllInputs: true,
      maskTextSelector: null,
      captureCanvas: { recordCanvas: false },
      sampleRate: 1,
    });
    expect(options.blockSelector).toContain(`[${ANALYTICS_BLOCK_ATTRIBUTE}]`);
    expect(options.maskAttributeFn?.("alt", "Demo-Kind")).toBe("Demo-Kind");
    expect(options.maskAttributeFn?.("href", "/students/12")).toBe(
      "https://demo/students/:id",
    );
  });

  it("rewrites the recording's page and network URLs without headers or bodies", () => {
    for (const context of [freigabe, demo]) {
      const masked = recordingOf(context).maskCapturedNetworkRequestFn?.({
        name: "https://school-a.moto-app.de/students/4711?tab=akte",
        entryType: "resource",
        startTime: 10,
        duration: 5,
        requestHeaders: { authorization: "Bearer x" },
        responseBody: "Kim Beispiel",
      });

      expect(masked).toEqual({
        name: `https://${context.deployment}/students/:id`,
        entryType: "resource",
        startTime: 10,
        duration: 5,
        requestHeaders: undefined,
        responseHeaders: undefined,
        requestBody: undefined,
        responseBody: undefined,
      });
    }
  });

  it("records nothing with the Freigabe but no sample", () => {
    const context = { ...freigabe, recordingSamplePercent: 0 };

    expect(analyticsRuntimeOptions(context)).toMatchObject({
      disable_session_recording: true,
    });
    expect(
      filterAnalyticsEvent(
        context,
        capture("$snapshot", { $snapshot_data: [] }),
      ),
    ).toBeNull();
  });

  it("names a person only for a pseudonymous ID with the Freigabe", () => {
    expect(analyticsIdentity(freigabe)).toBe(PERSON);
    expect(analyticsIdentity({ ...freigabe, person: "4711" })).toBeNull();
    expect(analyticsIdentity({ ...freigabe, person: null })).toBeNull();
    expect(
      analyticsIdentity({ ...freigabe, analyseFreigabe: false }),
    ).toBeNull();
    expect(analyticsIdentity({ ...freigabe, deployment: "demo" })).toBeNull();
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

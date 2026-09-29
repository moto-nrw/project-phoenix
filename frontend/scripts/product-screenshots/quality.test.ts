import { describe, expect, it } from "vitest";

import {
  BrokenShotError,
  findBrokenReasons,
  isIgnoredResponse,
  type ShotObservation,
} from "./quality";

const HEALTHY: ShotObservation = {
  requestedPath: "/dashboard",
  finalPath: "/dashboard",
  documentStatus: 200,
  failedResponses: [],
  consoleErrors: [],
  pageErrors: [],
  bodyText: "Anwesenheit heute: 12 Kinder",
  loginFormVisible: false,
  devErrorOverlayPresent: false,
  loadingIndicators: 0,
  unexpectedDialogs: 0,
};

describe("findBrokenReasons", () => {
  it("lässt einen gesunden Shot durch", () => {
    expect(findBrokenReasons(HEALTHY)).toEqual([]);
  });

  it.each([
    ["HTTP-Fehler der Zielseite", { documentStatus: 500 }, /HTTP 500/],
    ["fehlende Antwort", { documentStatus: null }, /nicht geantwortet/],
    [
      "HTTP-Fehler einer Datenabfrage",
      {
        failedResponses: [
          {
            method: "GET",
            url: "http://localhost:8080/api/students",
            status: 403,
          },
        ],
      },
      /HTTP 403 bei GET http:\/\/localhost:8080\/api\/students/,
    ],
    [
      "Fehler-Boundary",
      { bodyText: "Ein unerwarteter Fehler ist aufgetreten" },
      /Fehlerseite sichtbar/,
    ],
    [
      "fehlende Route",
      { documentStatus: 200, bodyText: "Seite nicht gefunden" },
      /Seite nicht gefunden/,
    ],
    ["Dev-Fehler-Overlay", { devErrorOverlayPresent: true }, /Fehler-Overlay/],
    [
      "eine stille Umleitung (fehlende Route)",
      { requestedPath: "/gibt-es-nicht", finalPath: "/" },
      /angefordert \/gibt-es-nicht, angekommen \//,
    ],
    [
      "Umleitung auf den Login",
      { loginFormVisible: true },
      /Umleitung auf den Login/,
    ],
    [
      "Ladezustand nach Timeout",
      { loadingIndicators: 3 },
      /3 Ladeanzeige\(n\)/,
    ],
    [
      "einen unerwarteten Dialog",
      { unexpectedDialogs: 1 },
      /unerwarteter Dialog/,
    ],
    [
      "Konsolenfehler",
      { consoleErrors: ["Warning: hydration mismatch"] },
      /Konsolenfehler: Warning: hydration mismatch/,
    ],
    [
      "unbehandelter Seitenfehler",
      { pageErrors: ["Cannot read properties of undefined"] },
      /Unbehandelter Fehler/,
    ],
  ] satisfies [string, Partial<ShotObservation>, RegExp][])(
    "meldet %s",
    (_name, patch, expected) => {
      const reasons = findBrokenReasons({ ...HEALTHY, ...patch });
      expect(reasons).toHaveLength(1);
      expect(reasons[0]).toMatch(expected);
    },
  );

  it("ignoriert einen abschließenden Schrägstrich beim Pfadvergleich", () => {
    expect(findBrokenReasons({ ...HEALTHY, finalPath: "/dashboard/" })).toEqual(
      [],
    );
  });

  it("sammelt mehrere Gründe", () => {
    const reasons = findBrokenReasons({
      ...HEALTHY,
      documentStatus: 404,
      bodyText: "Seite nicht gefunden",
    });
    expect(reasons).toHaveLength(2);
  });
});

describe("BrokenShotError", () => {
  it("nennt Shot, Gerät und alle Gründe", () => {
    const error = new BrokenShotError("kinder-zuhause", "iphone", [
      "HTTP 500",
      "Konsolenfehler: x",
    ]);
    expect(error.message).toContain("kinder-zuhause");
    expect(error.message).toContain("iphone");
    expect(error.message).toContain("- HTTP 500");
    expect(error.message).toContain("- Konsolenfehler: x");
  });
});

describe("isIgnoredResponse", () => {
  const pushKey = {
    method: "GET",
    url: "http://marketing.localhost:3036/api/notifications/push/public-key",
    status: 404,
  };

  it("überhört nur die Web-Push-Schlüssel, die lokal nicht konfiguriert sind", () => {
    expect(isIgnoredResponse(pushKey)).toBe(true);
    expect(
      isIgnoredResponse({
        ...pushKey,
        url: "http://eltern.localhost:3036/api/parent/me/push/public-key",
      }),
    ).toBe(true);
  });

  it("überhört keinen anderen Fehler", () => {
    expect(isIgnoredResponse({ ...pushKey, status: 500 })).toBe(false);
    expect(isIgnoredResponse({ ...pushKey, method: "POST" })).toBe(false);
    expect(
      isIgnoredResponse({
        ...pushKey,
        url: "http://marketing.localhost:3036/api/students",
      }),
    ).toBe(false);
  });
});

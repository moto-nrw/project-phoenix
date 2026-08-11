import { describe, expect, it } from "vitest";
import {
  faviconManifest,
  faviconMetadata,
  resolveFaviconVariant,
} from "./favicon-variants";

const config = {
  operatorHostname: "operator.moto-app.de",
  parentsHostname: "eltern.moto-app.de",
  schoolHostname: "schule.moto-app.de",
  tenantDomain: "moto-app.de",
};

describe("resolveFaviconVariant", () => {
  it.each([
    ["moto-app.de", "normal"],
    ["school-a.moto-app.de", "normal"],
    ["operator.moto-app.de", "operator"],
    ["eltern.moto-app.de", "normal"],
    ["parents.moto-app.de", "normal"],
    ["schule.moto-app.de", "schule"],
    ["staging.moto-app.de", "normal-staging"],
    ["school-a.staging.moto-app.de", "normal-staging"],
    ["operator.staging.moto-app.de", "operator-staging"],
    ["eltern.staging.moto-app.de", "eltern-staging"],
    ["parents.staging.moto-app.de", "eltern-staging"],
    ["schule.staging.moto-app.de", "schule-staging"],
  ] as const)("maps %s to %s", (host, expected) => {
    const hostConfig =
      host.endsWith(".staging.moto-app.de") || host === "staging.moto-app.de"
        ? {
            ...config,
            operatorHostname: "operator.staging.moto-app.de",
            parentsHostname: "eltern.staging.moto-app.de",
            schoolHostname: "schule.staging.moto-app.de",
          }
        : config;

    expect(resolveFaviconVariant(host, hostConfig)).toBe(expected);
  });

  it("ignores ports and forwarded-host lists", () => {
    expect(
      resolveFaviconVariant(
        "operator.moto-app.de:3000, proxy.internal",
        config,
      ),
    ).toBe("operator");
  });

  it("detects staging when TENANT_DOMAIN is the staging domain", () => {
    const stagingConfig = {
      operatorHostname: "operator.staging.moto-app.de",
      parentsHostname: "eltern.staging.moto-app.de",
      schoolHostname: "schule.staging.moto-app.de",
      tenantDomain: "staging.moto-app.de",
    };

    expect(
      resolveFaviconVariant("school-a.staging.moto-app.de", stagingConfig),
    ).toBe("normal-staging");
    expect(
      resolveFaviconVariant("operator.staging.moto-app.de", stagingConfig),
    ).toBe("operator-staging");
    expect(
      resolveFaviconVariant("parents.staging.moto-app.de", stagingConfig),
    ).toBe("eltern-staging");
  });
});

describe("faviconMetadata", () => {
  it("returns stable asset paths for a variant", () => {
    expect(faviconMetadata("operator-staging")).toEqual({
      icons: [
        {
          rel: "icon",
          url: "/favicons/operator-staging/favicon.ico",
          type: "image/x-icon",
        },
        {
          rel: "icon",
          url: "/favicons/operator-staging/icon-32.png",
          type: "image/png",
          sizes: "32x32",
        },
        {
          rel: "apple-touch-icon",
          url: "/favicons/operator-staging/apple-touch-icon.png",
          sizes: "180x180",
        },
      ],
      manifest: "/manifest.webmanifest",
    });
  });
});

describe("faviconManifest", () => {
  it("gives the school portal its own PWA identity", () => {
    expect(faviconManifest("schule").name).toBe("moto schule");
    expect(faviconManifest("schule-staging").short_name).toBe("moto schule");
    expect(faviconManifest("normal").name).toBe("MOTO");
    expect(faviconManifest("eltern").name).toBe("MOTO");
  });

  it("points install icons at the resolved variant", () => {
    expect(faviconManifest("eltern").icons).toEqual([
      {
        src: "/favicons/eltern/icon-192.png",
        sizes: "192x192",
        type: "image/png",
        purpose: "any",
      },
      {
        src: "/favicons/eltern/icon-512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "any",
      },
      {
        src: "/favicons/eltern/icon-maskable-192.png",
        sizes: "192x192",
        type: "image/png",
        purpose: "maskable",
      },
      {
        src: "/favicons/eltern/icon-maskable-512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
    ]);
  });
});

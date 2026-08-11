import type { Metadata, MetadataRoute } from "next";

export type FaviconVariant =
  | "normal"
  | "normal-staging"
  | "operator"
  | "operator-staging"
  | "eltern"
  | "eltern-staging"
  | "schule"
  | "schule-staging";

export type FaviconConfig = {
  readonly operatorHostname: string;
  readonly parentsHostname: string;
  readonly schoolHostname: string;
  readonly tenantDomain: string;
};

const ICON_BASE_PATH = "/favicons";

function normalizeHost(host: string): string {
  return host.split(",")[0]?.trim().toLowerCase().split(":")[0] ?? "";
}

function isStagingHost(host: string, tenantDomain: string): boolean {
  const stagingDomain = tenantDomain.startsWith("staging.")
    ? tenantDomain
    : `staging.${tenantDomain}`;
  return host === stagingDomain || host.endsWith(`.${stagingDomain}`);
}

function legacyParentsHost(
  parentsHostname: string,
  tenantDomain: string,
): string {
  const parentsHost = normalizeHost(parentsHostname);
  const tenantHost = normalizeHost(tenantDomain);

  if (!parentsHost || !tenantHost || !parentsHost.startsWith("eltern.")) {
    return "";
  }

  return `parents.${parentsHost.slice("eltern.".length)}`;
}

export function resolveFaviconVariant(
  rawHost: string,
  config: FaviconConfig,
): FaviconVariant {
  const host = normalizeHost(rawHost);
  const operatorHost = normalizeHost(config.operatorHostname);
  const parentsHost = normalizeHost(config.parentsHostname);
  const schoolHost = normalizeHost(config.schoolHostname);
  const tenantDomain = normalizeHost(config.tenantDomain);
  const legacyParents = legacyParentsHost(parentsHost, tenantDomain);
  const staging = isStagingHost(host, tenantDomain);

  if (host === operatorHost) {
    return staging ? "operator-staging" : "operator";
  }

  if (host === parentsHost || (legacyParents && host === legacyParents)) {
    return staging ? "eltern-staging" : "normal";
  }

  if (host === schoolHost) {
    return staging ? "schule-staging" : "schule";
  }

  return staging ? "normal-staging" : "normal";
}

export function faviconMetadata(
  variant: FaviconVariant,
): Pick<Metadata, "icons" | "manifest"> {
  const base = `${ICON_BASE_PATH}/${variant}`;

  return {
    icons: [
      { rel: "icon", url: `${base}/favicon.ico`, type: "image/x-icon" },
      {
        rel: "icon",
        url: `${base}/icon-32.png`,
        type: "image/png",
        sizes: "32x32",
      },
      {
        rel: "apple-touch-icon",
        url: `${base}/apple-touch-icon.png`,
        sizes: "180x180",
      },
    ],
    manifest: `/manifest.webmanifest`,
  };
}

export function faviconManifest(
  variant: FaviconVariant,
): MetadataRoute.Manifest {
  const base = `${ICON_BASE_PATH}/${variant}`;
  // The school portal installs as its own PWA identity ("moto schule",
  // #2207); every other surface keeps the shared MOTO identity.
  const isSchule = variant === "schule" || variant === "schule-staging";
  const appName = isSchule ? "moto schule" : "MOTO";

  return {
    name: appName,
    short_name: appName,
    icons: [
      {
        src: `${base}/icon-192.png`,
        sizes: "192x192",
        type: "image/png",
        purpose: "any",
      },
      {
        src: `${base}/icon-512.png`,
        sizes: "512x512",
        type: "image/png",
        purpose: "any",
      },
      {
        src: `${base}/icon-maskable-192.png`,
        sizes: "192x192",
        type: "image/png",
        purpose: "maskable",
      },
      {
        src: `${base}/icon-maskable-512.png`,
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
    ],
    theme_color: "#ffffff",
    background_color: "#ffffff",
    display: "standalone",
    start_url: "/",
    scope: "/",
  };
}

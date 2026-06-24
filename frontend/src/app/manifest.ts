import type { MetadataRoute } from "next";
import { headers } from "next/headers";
import { faviconManifest, resolveFaviconVariant } from "~/lib/favicon-variants";

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is not set.`);
  return value;
}

export default async function manifest(): Promise<MetadataRoute.Manifest> {
  const requestHeaders = await headers();
  const host =
    requestHeaders.get("x-forwarded-host") ?? requestHeaders.get("host") ?? "";
  const variant = resolveFaviconVariant(host, {
    operatorHostname: requiredEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME"),
    parentsHostname: requiredEnv("NEXT_PUBLIC_PARENTS_HOSTNAME"),
    tenantDomain: requiredEnv("TENANT_DOMAIN"),
  });

  return faviconManifest(variant);
}

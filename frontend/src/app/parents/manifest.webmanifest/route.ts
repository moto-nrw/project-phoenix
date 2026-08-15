import { headers } from "next/headers";
import { resolveFaviconVariant } from "~/lib/favicon-variants";
import parentsManifest from "../manifest";

/**
 * Liefert das Manifest der Eltern-App unter /parents/manifest.webmanifest.
 *
 * Next.js erkennt die Dateikonvention manifest.ts nur direkt in app/, nicht in
 * einem Unterverzeichnis (nachgeprueft: app/parents/manifest.ts allein
 * antwortet mit 404). Der Route Handler ist deshalb der Weg, ein zweites,
 * eltern-eigenes Manifest neben dem Wurzel-Manifest auszuliefern.
 *
 * Der Inhaltstyp muss application/manifest+json sein, sonst verweigern Browser
 * die Installation.
 */

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is not set.`);
  return value;
}

export async function GET(): Promise<Response> {
  const requestHeaders = await headers();
  const host =
    requestHeaders.get("x-forwarded-host") ?? requestHeaders.get("host") ?? "";
  // Staging soll ein anderes Symbol auf dem Home-Bildschirm tragen als die
  // Produktion. Die Staging-Erkennung liegt bereits in resolveFaviconVariant,
  // sie wird hier nur auf den Eltern-Symbolsatz abgebildet.
  const staging = resolveFaviconVariant(host, {
    operatorHostname: requiredEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME"),
    parentsHostname: requiredEnv("NEXT_PUBLIC_PARENTS_HOSTNAME"),
    tenantDomain: requiredEnv("TENANT_DOMAIN"),
  }).endsWith("-staging");

  return new Response(
    JSON.stringify(parentsManifest(staging ? "eltern-staging" : "eltern")),
    {
      headers: {
        "content-type": "application/manifest+json",
        "cache-control": "public, max-age=0, must-revalidate",
      },
    },
  );
}

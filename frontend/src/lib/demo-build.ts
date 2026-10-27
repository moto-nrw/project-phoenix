// The public demo build (#3462) and what it keeps from the visitor.

/**
 * True in the image built for the public demo only. Next.js inlines the
 * value at build time, so no other build ever shows the demo banner.
 */
export function isDemoBuild(): boolean {
  return process.env.NEXT_PUBLIC_APP_ENV === "demo";
}

// The addresses the seed gives every account and contact of a demo school,
// e.g. sabine.schneider@demo-ogs-nord.moto-ogs.de.
const DEMO_SEED_ADDRESS = /@demo-[a-z0-9-]+\.moto-ogs\.de$/i;

/**
 * The address shown for a person. The public demo hides its seed addresses,
 * as the header does (#3891): they are technical, and the visitor's own
 * caregiver and parent keep the address of the seed person whose name they
 * took (#5033). An address someone typed in the demo stays visible.
 */
export function displayEmail(email: string | null | undefined): string {
  if (!email) return "";
  return isDemoBuild() && DEMO_SEED_ADDRESS.test(email.trim()) ? "" : email;
}

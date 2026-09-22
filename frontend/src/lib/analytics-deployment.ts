import { clientEnv } from "~/env.client";
import { isDemoBuild } from "~/lib/demo-access";

/**
 * The `deployment` property of every analytics event: the tenant domain, or
 * `demo` in the public demo (#3467), whose events must not mix with a school's.
 */
export function analyticsDeployment(): string {
  return isDemoBuild() ? "demo" : clientEnv.NEXT_PUBLIC_TENANT_DOMAIN;
}

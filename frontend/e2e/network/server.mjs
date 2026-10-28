import { createServer } from "node:http";
import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";

const sharedTenantCookie = process.env.TENANT_SWITCH_EVIDENCE === "1";
// Local runs of parallel checkouts may move the shared-cookie ports aside.
const frontendPort = sharedTenantCookie
  ? Number(process.env.TENANT_SWITCH_PORT ?? 13845)
  : 13843;
const apiPort = sharedTenantCookie
  ? Number(process.env.TENANT_SWITCH_API_PORT ?? 18445)
  : 18443;
let switchRequests = 0;
let logoutRequests = 0;
const schools = ["school-a", "school-b"].map((subdomain, index) => ({
  tenant_id: index + 1,
  slug: subdomain,
  subdomain,
  name: index === 0 ? "School A" : "School B",
  organization_id: 1,
  organization_name: "Fixture",
}));

// Only synthetic fixtures. This is not a backend/authentication test.
const envelope = (data) => ({ status: "success", success: true, data });
const claims = (scope = "") => ({
  id: 1,
  sub: "1",
  scope,
  tenant_id: scope === "parent" ? undefined : 1,
  org_id: 1,
  first_name: "Test",
  last_name: "Fixture",
  email: "fixture@example.invalid",
  roles: [
    scope === "school" ? "lehrkraft" : scope === "parent" ? "guardian" : "user",
  ],
  permissions: [],
  exp: Math.floor(Date.now() / 1000) + 3600,
});
const profile = {
  id: 1,
  first_name: "Test",
  last_name: "Fixture",
  email: "fixture@example.invalid",
  settings: {},
};
const api = createServer(async (req, res) => {
  const url = new URL(req.url, `http://localhost:${apiPort}`);
  const path = url.pathname;
  let data;
  if (path === "/health") data = {};
  else if (path === "/auth/tenant/resolve") {
    const slug = url.searchParams.get("slug") ?? "proof-off";
    data = envelope({
      tenant_id: sharedTenantCookie && slug === "school-b" ? 2 : 1,
      slug,
      subdomain: slug,
      name: sharedTenantCookie
        ? slug === "school-b"
          ? "School B"
          : "School A"
        : "Netzwerktest",
      organization_id: 1,
      organization_name: "Fixture",
      settings: {},
      grade_level_max: 4,
      analytics_enabled: slug === "proof-on",
      analytics_freigabe: false,
      student_photos_enabled: true,
      timetable_enabled: false,
    });
  } else if (sharedTenantCookie && path === "/__fixture/switch-state") {
    data = { switchRequests, logoutRequests };
  } else if (sharedTenantCookie && path === "/auth/switch-tenant") {
    switchRequests++;
    res.writeHead(401, { "Content-Type": "application/problem+json" });
    res.end(
      JSON.stringify({ status: 401, code: "identity.mfa_session_required" }),
    );
    return;
  } else if (sharedTenantCookie && path === "/auth/logout") {
    logoutRequests++;
    data = {};
  } else if (
    ["/auth/login", "/school/auth/login", "/parent/auth/login"].includes(path)
  ) {
    const scope = path.startsWith("/school")
      ? "school"
      : path.startsWith("/parent")
        ? "parent"
        : "";
    const token = `fixture.${Buffer.from(JSON.stringify(claims(scope))).toString("base64url")}.fixture`;
    const pair = { access_token: token, refresh_token: token };
    data = scope === "parent" ? envelope(pair) : pair;
  } else if (path === "/auth/session/validate") {
    let body = "";
    for await (const chunk of req) body += chunk;
    const portal = JSON.parse(body).portal;
    data = claims(portal === "tenant" ? "" : portal);
  } else if (path === "/api/me/profile") data = envelope(profile);
  else if (path === "/api/me/navigation")
    data = envelope({
      educational_groups: [],
      supervised_groups: [],
      current_staff: null,
      incomplete: false,
      unavailable_sections: [],
    });
  else if (path === "/auth/account/tenants")
    data = envelope(sharedTenantCookie ? schools : []);
  else {
    res.writeHead(404, { "Content-Type": "application/json" });
    res.end(
      JSON.stringify({ status: "error", code: "fixture.not_configured" }),
    );
    return;
  }
  res.writeHead(200, { "Content-Type": "application/json" });
  res.end(JSON.stringify(data));
});
api.listen(apiPort, "127.0.0.1");

// Do not inherit user-specific service configuration or credentials. Explicit
// empty values also take precedence over Next's automatic .env.local loading.
const env = {
  PATH: process.env.PATH,
  HOME: process.env.HOME,
  TMPDIR: process.env.TMPDIR,
  NODE_ENV: "development",
  NEXT_TELEMETRY_DISABLED: "1",
  AUTH_TRUST_HOST: "true",
  API_URL: `http://127.0.0.1:${apiPort}`,
  NEXT_PUBLIC_API_URL: `http://127.0.0.1:${apiPort}`,
  NEXTAUTH_URL: `http://localhost:${frontendPort}`,
  NEXTAUTH_SECRET: randomBytes(32).toString("hex"),
  AUTH_JWT_EXPIRY: "15m",
  AUTH_JWT_REFRESH_EXPIRY: "12h",
  TENANT_DOMAIN: sharedTenantCookie ? "mfa.localhost" : "localhost",
  METRICS_BEARER_TOKEN: randomBytes(32).toString("hex"),
  NEXT_PUBLIC_APP_ENV: "development",
  NEXT_PUBLIC_TENANT_DOMAIN: sharedTenantCookie ? "mfa.localhost" : "localhost",
  NEXT_PUBLIC_OPERATOR_HOSTNAME: `operator.localhost:${frontendPort}`,
  NEXT_PUBLIC_PARENTS_HOSTNAME: `eltern.localhost:${frontendPort}`,
  NEXT_PUBLIC_SCHOOL_HOSTNAME: `${sharedTenantCookie ? "schule.mfa.localhost" : "schule.localhost"}:${frontendPort}`,
  NEXT_PUBLIC_CATERER_HOSTNAME: `caterer.localhost:${frontendPort}`,
  NEXT_PUBLIC_TRAEGER_HOSTNAME: `traeger.localhost:${frontendPort}`,
  NEXT_PUBLIC_POSTHOG_KEY: "network-evidence-placeholder",
  // SaaS-shaped, nonfunctional placeholder so the real SDK uses /monitoring.
  NEXT_PUBLIC_SENTRY_DSN: "https://network_evidence@o0.ingest.sentry.io/1",
  NEXT_PUBLIC_SENTRY_ENVIRONMENT: "network-evidence",
  SENTRY_DSN: "",
  SENTRY_AUTH_TOKEN: "",
  NEXT_PUBLIC_LOG_LEVEL: "error",
  NODE_OPTIONS: "--require ./e2e/network/local-only.cjs",
};
const next = spawn(
  process.execPath,
  ["node_modules/next/dist/bin/next", "dev", "--port", String(frontendPort)],
  { env, stdio: "ignore" },
);
const stop = () => {
  next.kill("SIGTERM");
  api.close();
  api.closeAllConnections();
};
process.on("SIGTERM", stop);
process.on("SIGINT", stop);
next.on("exit", (code) => {
  api.close();
  api.closeAllConnections();
  process.exitCode = code ?? 1;
});

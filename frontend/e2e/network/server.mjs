import { createServer } from "node:http";
import { connect } from "node:net";
import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { fileURLToPath } from "node:url";
import { join } from "node:path";
import { createAppWorkspace } from "./app-workspace.mjs";
import { networkPorts } from "./ports.mjs";

const sharedTenantCookie = process.env.TENANT_SWITCH_EVIDENCE === "1";
const ports = networkPorts(sharedTenantCookie ? "tenantSwitch" : "network");
const frontendPort = ports.frontend;
const apiPort = ports.api;
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
// Shared-cookie mode models the server-side session family (#5194): login
// opens a family, logout revokes it, and session routes refuse a token whose
// family is gone although it has not expired.
let familySequence = 0;
const liveFamilies = new Set();
const fixtureToken = (payload) =>
  `fixture.${Buffer.from(JSON.stringify(payload)).toString("base64url")}.fixture`;
const tokenClaims = (token) => {
  try {
    return JSON.parse(
      Buffer.from(String(token).split(".")[1] ?? "", "base64url").toString(),
    );
  } catch {
    return null;
  }
};
const bearerClaims = (req) =>
  tokenClaims((req.headers.authorization ?? "").replace(/^Bearer /, ""));
const familyRevoked = (req) => {
  const family = bearerClaims(req)?.family_id;
  return (
    sharedTenantCookie && family !== undefined && !liveFamilies.has(family)
  );
};
// Every session route of every portal refuses a revoked family; the auth
// routes answer for themselves.
const familyRoutes = (path) =>
  !path.startsWith("/auth/") &&
  !path.startsWith("/operator/auth/") &&
  !path.startsWith("/__fixture/") &&
  path !== "/health";
// One protected read per other portal, behind its real BFF route.
const portalReads = new Set([
  "/parent/me/calendar",
  "/school/supervisions/",
  "/operator/organizations",
  "/caterer/schools",
]);
const refuse = (res, body) => {
  res.writeHead(401, { "Content-Type": "application/problem+json" });
  res.end(JSON.stringify(body));
};
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
  if (familyRoutes(path) && familyRevoked(req)) {
    refuse(res, {
      status: 401,
      detail: "session has been revoked",
      code: "general.permission",
    });
    return;
  }
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
  } else if (sharedTenantCookie && portalReads.has(path)) {
    data = envelope([]);
  } else if (sharedTenantCookie && path === "/operator/auth/logout") {
    logoutRequests++;
    liveFamilies.delete(bearerClaims(req)?.family_id);
    res.writeHead(204);
    res.end();
    return;
  } else if (sharedTenantCookie && path === "/operator/auth/refresh") {
    const presented = bearerClaims(req);
    if (!liveFamilies.has(presented?.family_id)) {
      refuse(res, { status: 401, code: "general.permission" });
      return;
    }
    const token = fixtureToken({
      ...claims("platform"),
      family_id: presented.family_id,
    });
    data = envelope({ access_token: token, refresh_token: token });
  } else if (sharedTenantCookie && path === "/__fixture/switch-state") {
    data = { switchRequests, logoutRequests, liveFamilies: liveFamilies.size };
  } else if (sharedTenantCookie && path === "/auth/switch-tenant") {
    switchRequests++;
    res.writeHead(401, { "Content-Type": "application/problem+json" });
    res.end(
      JSON.stringify({ status: 401, code: "identity.mfa_session_required" }),
    );
    return;
  } else if (sharedTenantCookie && path === "/auth/logout") {
    // Like the backend: 204 only after the family is revoked.
    logoutRequests++;
    liveFamilies.delete(bearerClaims(req)?.family_id);
    res.writeHead(204);
    res.end();
    return;
  } else if (sharedTenantCookie && path === "/auth/refresh") {
    const presented = bearerClaims(req);
    if (!liveFamilies.has(presented?.family_id)) {
      refuse(res, { status: 401, code: "general.permission" });
      return;
    }
    const token = fixtureToken({
      ...claims(presented.scope ?? ""),
      family_id: presented.family_id,
    });
    data = { access_token: token, refresh_token: token };
  } else if (
    [
      "/auth/login",
      "/school/auth/login",
      "/parent/auth/login",
      "/operator/auth/login",
      "/caterer/auth/login",
    ].includes(path)
  ) {
    const scope = path.startsWith("/school")
      ? "school"
      : path.startsWith("/parent")
        ? "parent"
        : path.startsWith("/operator")
          ? "platform"
          : path.startsWith("/caterer")
            ? "caterer"
            : "";
    const family = sharedTenantCookie
      ? `family-${++familySequence}`
      : undefined;
    if (family) liveFamilies.add(family);
    const token = fixtureToken({ ...claims(scope), family_id: family });
    const pair = { access_token: token, refresh_token: token };
    data = ["parent", "platform", "caterer"].includes(scope)
      ? envelope(pair)
      : scope === "school"
        ? { status: "authenticated", ...pair }
        : pair;
  } else if (path === "/auth/session/validate") {
    let body = "";
    for await (const chunk of req) body += chunk;
    const { portal, access_token: access } = JSON.parse(body);
    data = {
      ...claims(portal === "tenant" ? "" : portal),
      family_id: tokenClaims(access)?.family_id,
    };
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

const fail = (message) => {
  process.stderr.write(`Network evidence: ${message}\n`);
  process.exit(1);
};
// macOS lets a wildcard and a loopback listener share a port, so a bind
// alone does not prove that loopback traffic reaches this run. Any answer on
// either loopback address means another process owns the port: stop instead
// of testing against it, and never touch that process.
const answers = (port, host) =>
  new Promise((resolve) => {
    const socket = connect({ port, host });
    socket.once("connect", () => {
      socket.destroy();
      resolve(true);
    });
    socket.once("error", () => resolve(false));
  });
for (const [service, port, name] of [
  ["fixture API", apiPort, ports.names.api],
  ["frontend", frontendPort, ports.names.frontend],
]) {
  for (const host of ["127.0.0.1", "::1"]) {
    if (await answers(port, host)) {
      fail(
        `${service} port ${port} is already in use on ${host}. Stop your own earlier run or choose another port with ${name}.`,
      );
    }
  }
}
await new Promise((resolve) => {
  api.once("error", (error) =>
    fail(
      `fixture API cannot listen on 127.0.0.1:${apiPort} (${error.code ?? "error"}). Choose another port with ${ports.names.api}.`,
    ),
  );
  api.listen(apiPort, "127.0.0.1", resolve);
});

// Do not inherit user-specific service configuration or credentials. Explicit
// empty values also take precedence over Next's automatic .env.local loading.
const frontendDirectory = fileURLToPath(new URL("../../", import.meta.url));
const workspace = createAppWorkspace(frontendDirectory);
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
  NODE_OPTIONS: `--require ${JSON.stringify(fileURLToPath(new URL("./local-only.cjs", import.meta.url)))}`,
};
const next = spawn(
  process.execPath,
  [
    join(frontendDirectory, "node_modules/next/dist/bin/next"),
    "dev",
    "--port",
    String(frontendPort),
  ],
  { cwd: workspace.directory, env, stdio: "ignore" },
);
// Only this run's own children: the Next child and the fixture listener.
let stopping = false;
const stop = () => {
  stopping = true;
  next.kill("SIGTERM");
  api.close();
  api.closeAllConnections();
};
process.on("SIGTERM", stop);
process.on("SIGINT", stop);
process.on("exit", () => next.kill("SIGTERM"));
// Playwright holds stdin open; it closes when the runner ends, even when the
// runner is killed without tearing the server down.
if (process.argv.includes("--stop-with-stdin")) {
  process.stdin.on("end", stop).resume();
}
next.on("error", (error) => {
  api.close();
  workspace.cleanup();
  fail(`frontend could not start (${error.message})`);
});
next.on("close", (code, signal) => {
  workspace.cleanup();
  if (!stopping && code !== 0 && signal === null) {
    process.stderr.write(
      `Network evidence: frontend on port ${frontendPort} exited with code ${code}. Check the frontend configuration or choose another port with ${ports.names.frontend}.\n`,
    );
  }
  api.close();
  api.closeAllConnections();
  // The open stdin pipe would otherwise keep this process alive.
  process.stdin.destroy();
  process.exitCode = stopping ? 0 : (code ?? 1);
});

import { createTenantLoginRoute } from "~/lib/tenant-login-route.server";

export const POST = createTenantLoginRoute("/auth/login");

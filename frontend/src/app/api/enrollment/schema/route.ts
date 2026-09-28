import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

export const GET = createTenantJsonProxy({
  method: "GET",
  path: "/api/enrollment/schema",
  contentTypeOnGet: false,
  unauthorizedError: "Unauthenticated",
});

export const POST = createTenantJsonProxy({
  method: "POST",
  path: "/api/enrollment/schema",
  unauthorizedError: "Unauthenticated",
  mapJsonBody: (value) => {
    const body = value as Record<string, unknown>;
    return {
      name: typeof body.name === "string" ? body.name : "",
      fields: body.fields ?? [],
      // Absent means backend defaults; present persists the admin's toggle.
      ...(body.core_requirements === undefined
        ? {}
        : { core_requirements: body.core_requirements }),
      ...(body.legal_blocks === undefined
        ? {}
        : { legal_blocks: body.legal_blocks }),
    };
  },
});

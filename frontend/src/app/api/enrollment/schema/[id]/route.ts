import { NextResponse } from "next/server";
import { createTenantJsonProxy } from "~/lib/backend-proxy-route.server";

const path = (
  _request: Request,
  params: Record<string, string | string[] | undefined>,
) => {
  const id = params.id;
  if (typeof id !== "string" || !id) {
    return NextResponse.json({ error: "Invalid id" }, { status: 400 });
  }
  return `/api/enrollment/schema/${encodeURIComponent(id)}`;
};

export const GET = createTenantJsonProxy({
  method: "GET",
  path,
  cache: "no-store",
  contentTypeOnGet: false,
  unauthorizedError: "Unauthenticated",
});
export const PUT = createTenantJsonProxy({
  method: "PUT",
  path,
  unauthorizedError: "Unauthenticated",
  mapJsonBody: (value) => {
    const body = value as Record<string, unknown>;
    return {
      ...(body.name === undefined ? {} : { name: body.name }),
      fields: body.fields ?? [],
      ...(body.core_requirements === undefined
        ? {}
        : { core_requirements: body.core_requirements }),
      ...(body.legal_blocks === undefined
        ? {}
        : { legal_blocks: body.legal_blocks }),
    };
  },
});
export const PATCH = createTenantJsonProxy({
  method: "PATCH",
  path,
  unauthorizedError: "Unauthenticated",
  mapJsonBody: (value) => {
    const body = value as Record<string, unknown>;
    return { name: body.name };
  },
});
export const DELETE = createTenantJsonProxy({
  method: "DELETE",
  path,
  contentTypeOnDelete: false,
  unauthorizedError: "Unauthenticated",
});

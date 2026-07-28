import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AccountTenantAccessAPI" });

/** One role an account holds at one school. */
interface AccountTenantRole {
  id: string;
  name: string;
}

/** One school an account has (or had) access to. */
export interface AccountTenantAccess {
  tenantId: string;
  schoolName: string;
  schoolSlug: string;
  schoolActive: boolean;
  organizationId: string;
  organizationName: string;
  status: "active" | "pending" | "inactive";
  activatedAt: string | null;
  deactivatedAt: string | null;
  hasPerson: boolean;
  hasStaff: boolean;
  roles: AccountTenantRole[];
}

interface BackendAccountTenantAccess {
  tenant_id: number;
  school_name: string;
  school_slug: string;
  school_active: boolean;
  organization_id: number;
  organization_name: string;
  status: string;
  activated_at?: string | null;
  deactivated_at?: string | null;
  has_person: boolean;
  has_staff: boolean;
  roles?: { id: number; name: string }[] | null;
}

interface GrantAccountTenantAccessRequest {
  schoolId: string;
  roleId: string;
  firstName?: string;
  lastName?: string;
  position?: string;
}

export class AccountTenantAccessApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "AccountTenantAccessApiError";
    this.status = status;
  }
}

function mapAccess(entry: BackendAccountTenantAccess): AccountTenantAccess {
  const status =
    entry.status === "active" || entry.status === "pending"
      ? entry.status
      : "inactive";
  return {
    tenantId: entry.tenant_id.toString(),
    schoolName: entry.school_name,
    schoolSlug: entry.school_slug,
    schoolActive: entry.school_active,
    organizationId: entry.organization_id.toString(),
    organizationName: entry.organization_name,
    status,
    activatedAt: entry.activated_at ?? null,
    deactivatedAt: entry.deactivated_at ?? null,
    hasPerson: entry.has_person,
    hasStaff: entry.has_staff,
    roles: (entry.roles ?? []).map((role) => ({
      id: role.id.toString(),
      name: role.name,
    })),
  };
}

async function throwApiError(response: Response): Promise<never> {
  let message = `Die Anfrage ist fehlgeschlagen (${response.status}).`;
  try {
    const payload = (await response.json()) as {
      message?: string;
      error?: string;
    };
    message = payload.message ?? payload.error ?? message;
  } catch (error) {
    logger.warn("failed to parse school access error response", {
      error: error instanceof Error ? error.message : String(error),
      status: response.status,
    });
  }
  throw new AccountTenantAccessApiError(message, response.status);
}

async function request(
  endpoint: string,
  options: RequestInit = {},
): Promise<AccountTenantAccess[]> {
  const response = await fetch(endpoint, {
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(options.headers ?? {}),
    },
    ...options,
  });

  if (!response.ok) {
    await throwApiError(response);
  }

  const payload = (await response.json()) as {
    data?: BackendAccountTenantAccess[] | null;
  };
  return (payload.data ?? []).map(mapAccess);
}

async function requestRoles(
  endpoint: string,
): Promise<{ id: string; name: string }[]> {
  const response = await fetch(endpoint, {
    credentials: "include",
    headers: { "Content-Type": "application/json" },
  });
  if (!response.ok) {
    await throwApiError(response);
  }
  const payload = (await response.json()) as {
    data?: { id: number; name: string }[] | null;
  };
  return (payload.data ?? []).map((role) => ({
    id: role.id.toString(),
    name: role.name,
  }));
}

function basePath(accountId: string): string {
  return `/api/operator/accounts/${encodeURIComponent(accountId)}/tenants`;
}

class AccountTenantAccessService {
  list(accountId: string) {
    return request(basePath(accountId));
  }

  listAssignableRoles(accountId: string, tenantId: string) {
    return requestRoles(
      `${basePath(accountId)}/${encodeURIComponent(tenantId)}/roles`,
    );
  }

  grant(accountId: string, body: GrantAccountTenantAccessRequest) {
    return request(basePath(accountId), {
      method: "POST",
      body: JSON.stringify({
        school_id: Number(body.schoolId),
        role_id: Number(body.roleId),
        first_name: body.firstName,
        last_name: body.lastName,
        position: body.position,
      }),
    });
  }

  updateRole(accountId: string, tenantId: string, roleId: string) {
    return request(`${basePath(accountId)}/${encodeURIComponent(tenantId)}`, {
      method: "PUT",
      body: JSON.stringify({ role_id: Number(roleId) }),
    });
  }

  revoke(accountId: string, tenantId: string) {
    return request(`${basePath(accountId)}/${encodeURIComponent(tenantId)}`, {
      method: "DELETE",
    });
  }
}

export const accountTenantAccessService = new AccountTenantAccessService();

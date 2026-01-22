/**
 * Admin API client for SaaS organization management.
 */

export interface Organization {
  id: string;
  name: string;
  slug: string;
  status: "pending" | "active" | "rejected" | "suspended";
  createdAt: string;
  ownerEmail: string | null;
  ownerName: string | null;
}

export interface OrganizationsResponse {
  organizations: Organization[];
}

export interface OrgActionResponse {
  success: boolean;
  organization: Organization;
}

interface ApiErrorResponse {
  error?: string;
}

function getErrorMessage(error: unknown): string {
  if (
    error &&
    typeof error === "object" &&
    "error" in error &&
    typeof (error as ApiErrorResponse).error === "string"
  ) {
    return (error as ApiErrorResponse).error ?? "Unknown error";
  }
  return "Unknown error";
}

export interface CreateOrganizationRequest {
  name: string;
  slug?: string;
}

/**
 * Create a new organization with active status (for SaaS admin console).
 * The organization is created without an owner - invited admins will manage it.
 */
export async function createOrganization(
  data: CreateOrganizationRequest,
): Promise<Organization> {
  const response = await fetch("/api/admin/organizations", {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(data),
  });

  if (!response.ok) {
    const errorData: unknown = await response
      .json()
      .catch(() => ({ error: "Unknown error" }));
    throw new Error(getErrorMessage(errorData));
  }

  const result = (await response.json()) as OrgActionResponse;
  return result.organization;
}

/**
 * Fetch all organizations with optional status filter.
 */
export async function fetchOrganizations(
  status?: Organization["status"],
): Promise<Organization[]> {
  const url = new URL("/api/admin/organizations", window.location.origin);
  if (status) {
    url.searchParams.set("status", status);
  }

  const response = await fetch(url.toString(), {
    method: "GET",
    credentials: "include",
  });

  if (!response.ok) {
    const errorData: unknown = await response
      .json()
      .catch(() => ({ error: "Unknown error" }));
    throw new Error(getErrorMessage(errorData));
  }

  const data = (await response.json()) as OrganizationsResponse;
  return data.organizations;
}

/**
 * Approve a pending organization.
 */
export async function approveOrganization(
  orgId: string,
): Promise<Organization> {
  const response = await fetch(`/api/admin/organizations/${orgId}/approve`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
  });

  if (!response.ok) {
    const errorData: unknown = await response
      .json()
      .catch(() => ({ error: "Unknown error" }));
    throw new Error(getErrorMessage(errorData));
  }

  const data = (await response.json()) as OrgActionResponse;
  return data.organization;
}

/**
 * Reject a pending organization.
 */
export async function rejectOrganization(
  orgId: string,
  reason?: string,
): Promise<Organization> {
  const response = await fetch(`/api/admin/organizations/${orgId}/reject`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ reason }),
  });

  if (!response.ok) {
    const errorData: unknown = await response
      .json()
      .catch(() => ({ error: "Unknown error" }));
    throw new Error(getErrorMessage(errorData));
  }

  const data = (await response.json()) as OrgActionResponse;
  return data.organization;
}

/**
 * Suspend an active organization.
 */
export async function suspendOrganization(
  orgId: string,
): Promise<Organization> {
  const response = await fetch(`/api/admin/organizations/${orgId}/suspend`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
  });

  if (!response.ok) {
    const errorData: unknown = await response
      .json()
      .catch(() => ({ error: "Unknown error" }));
    throw new Error(getErrorMessage(errorData));
  }

  const data = (await response.json()) as OrgActionResponse;
  return data.organization;
}

/**
 * Reactivate a suspended organization.
 */
export async function reactivateOrganization(
  orgId: string,
): Promise<Organization> {
  const response = await fetch(`/api/admin/organizations/${orgId}/reactivate`, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
    },
  });

  if (!response.ok) {
    const errorData: unknown = await response
      .json()
      .catch(() => ({ error: "Unknown error" }));
    throw new Error(getErrorMessage(errorData));
  }

  const data = (await response.json()) as OrgActionResponse;
  return data.organization;
}

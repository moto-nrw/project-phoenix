import { operatorFetch } from "./api-helpers";
import type {
  BackendOrganization,
  BackendOrganizationSummary,
  BackendProvisioningStats,
  BackendSchool,
  BackendSchoolSummary,
  BackendInvitation,
  BackendSchoolAccount,
  BackendOrgAccount,
  BackendOperatorDevice,
  BackendOperatorPerson,
  BackendUnregisteredTagScan,
  Organization,
  OrganizationSummary,
  ProvisioningStats,
  School,
  SchoolSummary,
  Invitation,
  SchoolAccount,
  OrgAccount,
  OperatorDevice,
  OperatorPerson,
  UnregisteredTagScan,
  CreateOrganizationRequest,
  CreateSchoolRequest,
  CreateDeviceRequest,
  InviteAdminRequest,
  CreateAccountRequest,
  UpdateOrganizationRequest,
  UpdateSchoolRequest,
  BackendDeviceTransferStatus,
  DeviceTransferStatus,
} from "./provisioning-helpers";
import {
  mapOrganization,
  mapOrganizationSummary,
  mapProvisioningStats,
  mapSchool,
  mapSchoolSummary,
  mapInvitation,
  mapSchoolAccount,
  mapOrgAccount,
  mapOperatorDevice,
  mapOperatorPerson,
  mapUnregisteredTagScan,
  mapDeviceTransferStatus,
} from "./provisioning-helpers";

class OperatorProvisioningService {
  async getStats(): Promise<ProvisioningStats> {
    const data = await operatorFetch<BackendProvisioningStats>(
      "/api/operator/provisioning/stats",
    );
    return mapProvisioningStats(data);
  }

  async listOrganizationSummaries(): Promise<OrganizationSummary[]> {
    const data = await operatorFetch<BackendOrganizationSummary[]>(
      "/api/operator/provisioning/organizations/summaries",
    );
    return data.map(mapOrganizationSummary);
  }

  async listSchoolSummaries(): Promise<SchoolSummary[]> {
    const data = await operatorFetch<BackendSchoolSummary[]>(
      "/api/operator/provisioning/schools/summaries",
    );
    return data.map(mapSchoolSummary);
  }

  async listOrganizationSchools(
    organizationId: string,
  ): Promise<SchoolSummary[]> {
    const data = await operatorFetch<BackendSchoolSummary[]>(
      `/api/operator/provisioning/organizations/${encodeURIComponent(organizationId)}/schools`,
    );
    return data.map(mapSchoolSummary);
  }

  async listOrganizations(): Promise<Organization[]> {
    const data = await operatorFetch<BackendOrganization[]>(
      "/api/operator/provisioning/organizations",
    );
    return data.map(mapOrganization);
  }

  async createOrganization(
    data: CreateOrganizationRequest,
  ): Promise<Organization> {
    const result = await operatorFetch<BackendOrganization>(
      "/api/operator/provisioning/organizations",
      { method: "POST", body: data },
    );
    return mapOrganization(result);
  }

  async listSchools(): Promise<School[]> {
    const data = await operatorFetch<BackendSchool[]>(
      "/api/operator/provisioning/schools",
    );
    return data.map(mapSchool);
  }

  async createSchool(data: CreateSchoolRequest): Promise<School> {
    const result = await operatorFetch<BackendSchool>(
      "/api/operator/provisioning/schools",
      { method: "POST", body: data },
    );
    return mapSchool(result);
  }

  async updateOrganization(
    id: string,
    data: UpdateOrganizationRequest,
  ): Promise<Organization> {
    const result = await operatorFetch<BackendOrganization>(
      `/api/operator/provisioning/organizations/${encodeURIComponent(id)}`,
      { method: "PUT", body: data },
    );
    return mapOrganization(result);
  }

  async updateSchool(id: string, data: UpdateSchoolRequest): Promise<School> {
    const result = await operatorFetch<BackendSchool>(
      `/api/operator/provisioning/schools/${encodeURIComponent(id)}`,
      { method: "PUT", body: data },
    );
    return mapSchool(result);
  }

  async listSchoolAccounts(schoolId: string): Promise<SchoolAccount[]> {
    const data = await operatorFetch<BackendSchoolAccount[]>(
      `/api/operator/provisioning/schools/${encodeURIComponent(schoolId)}/accounts`,
    );
    return data.map(mapSchoolAccount);
  }

  async listOrganizationAccounts(orgId: string): Promise<OrgAccount[]> {
    const data = await operatorFetch<BackendOrgAccount[]>(
      `/api/operator/provisioning/organizations/${encodeURIComponent(orgId)}/accounts`,
    );
    return data.map(mapOrgAccount);
  }

  async listAllAccounts(): Promise<OrgAccount[]> {
    const data = await operatorFetch<BackendOrgAccount[]>(
      "/api/operator/provisioning/accounts",
    );
    return data.map(mapOrgAccount);
  }

  async listAllDevices(): Promise<OperatorDevice[]> {
    const data = await operatorFetch<BackendOperatorDevice[]>(
      "/api/operator/provisioning/devices",
    );
    return data.map(mapOperatorDevice);
  }

  async listSchoolDevices(schoolId: string): Promise<OperatorDevice[]> {
    const data = await operatorFetch<BackendOperatorDevice[]>(
      `/api/operator/provisioning/schools/${encodeURIComponent(schoolId)}/devices`,
    );
    return data.map(mapOperatorDevice);
  }

  async listOrganizationDevices(orgId: string): Promise<OperatorDevice[]> {
    const data = await operatorFetch<BackendOperatorDevice[]>(
      `/api/operator/provisioning/organizations/${encodeURIComponent(orgId)}/devices`,
    );
    return data.map(mapOperatorDevice);
  }

  async createDevice(data: CreateDeviceRequest): Promise<OperatorDevice> {
    const result = await operatorFetch<BackendOperatorDevice>(
      "/api/operator/provisioning/devices",
      { method: "POST", body: data },
    );
    return mapOperatorDevice(result);
  }

  async deleteDevice(deviceId: string): Promise<void> {
    await operatorFetch<null>(
      `/api/operator/provisioning/devices/${encodeURIComponent(deviceId)}`,
      { method: "DELETE" },
    );
  }

  async setDeviceAPIKey(
    deviceId: string,
    apiKey?: string,
  ): Promise<OperatorDevice> {
    const result = await operatorFetch<BackendOperatorDevice>(
      `/api/operator/provisioning/devices/${encodeURIComponent(deviceId)}/set-api-key`,
      { method: "POST", body: apiKey ? { api_key: apiKey } : {} },
    );
    return mapOperatorDevice(result);
  }

  async getDeviceTransferStatus(
    deviceId: string,
  ): Promise<DeviceTransferStatus> {
    const result = await operatorFetch<BackendDeviceTransferStatus>(
      `/api/operator/provisioning/devices/${encodeURIComponent(deviceId)}/transfer-status`,
    );
    return mapDeviceTransferStatus(result);
  }

  async transferDevice(
    deviceId: string,
    targetSchoolId: string,
  ): Promise<OperatorDevice> {
    const result = await operatorFetch<BackendOperatorDevice>(
      `/api/operator/provisioning/devices/${encodeURIComponent(deviceId)}/transfer`,
      {
        method: "POST",
        body: { target_school_id: Number(targetSchoolId) },
      },
    );
    return mapOperatorDevice(result);
  }

  async inviteSchoolAdmin(
    schoolId: string,
    data: InviteAdminRequest,
  ): Promise<Invitation> {
    const result = await operatorFetch<BackendInvitation>(
      `/api/operator/provisioning/schools/${encodeURIComponent(schoolId)}/invite-admin`,
      { method: "POST", body: data },
    );
    return mapInvitation(result);
  }

  async createSchoolAccount(
    schoolId: string,
    data: CreateAccountRequest,
  ): Promise<{ id: string; email: string }> {
    const result = await operatorFetch<{ id: number; email: string }>(
      `/api/operator/provisioning/schools/${encodeURIComponent(schoolId)}/create-account`,
      { method: "POST", body: data },
    );
    return { id: result.id.toString(), email: result.email };
  }

  async listSchoolPersons(schoolId: string): Promise<OperatorPerson[]> {
    const data = await operatorFetch<BackendOperatorPerson[]>(
      `/api/operator/provisioning/schools/${encodeURIComponent(schoolId)}/persons`,
    );
    return data.map(mapOperatorPerson);
  }

  async listOrganizationPersons(
    organizationId: string,
  ): Promise<OperatorPerson[]> {
    const data = await operatorFetch<BackendOperatorPerson[]>(
      `/api/operator/provisioning/organizations/${encodeURIComponent(organizationId)}/persons`,
    );
    return data.map(mapOperatorPerson);
  }

  async softDeletePerson(personId: string): Promise<void> {
    await operatorFetch<null>(
      `/api/operator/provisioning/persons/${encodeURIComponent(personId)}`,
      { method: "DELETE" },
    );
  }

  async listUnregisteredTagScans(options: {
    organizationId?: string;
    schoolId?: string;
    resolved?: "unresolved" | "all";
  }): Promise<UnregisteredTagScan[]> {
    const params = new URLSearchParams();
    if (options.organizationId) {
      params.set("organization_id", options.organizationId);
    }
    if (options.schoolId) {
      params.set("school_id", options.schoolId);
    }
    if (options.resolved === "all") {
      params.set("resolved", "all");
    }
    const suffix = params.toString() ? `?${params.toString()}` : "";
    const data = await operatorFetch<BackendUnregisteredTagScan[]>(
      `/api/operator/provisioning/unregistered-tag-scans${suffix}`,
    );
    return data.map(mapUnregisteredTagScan);
  }

  async resolveUnregisteredTagScan(id: string, note?: string): Promise<void> {
    const body = note ? { note } : {};
    await operatorFetch<BackendUnregisteredTagScan>(
      `/api/operator/provisioning/unregistered-tag-scans/${encodeURIComponent(id)}/resolve`,
      { method: "POST", body },
    );
  }

  async softDeleteOrganization(id: string): Promise<void> {
    await operatorFetch(
      `/api/operator/provisioning/organizations/${encodeURIComponent(id)}`,
      { method: "DELETE" },
    );
  }

  async restoreOrganization(id: string): Promise<void> {
    await operatorFetch(
      `/api/operator/provisioning/organizations/${encodeURIComponent(id)}/restore`,
      { method: "POST" },
    );
  }

  async softDeleteSchool(id: string): Promise<void> {
    await operatorFetch(
      `/api/operator/provisioning/schools/${encodeURIComponent(id)}`,
      { method: "DELETE" },
    );
  }

  async restoreSchool(id: string): Promise<void> {
    await operatorFetch(
      `/api/operator/provisioning/schools/${encodeURIComponent(id)}/restore`,
      { method: "POST" },
    );
  }

  async listSystemRoles(): Promise<
    { id: string; name: string; isSystem: boolean }[]
  > {
    const result = await operatorFetch<
      { id: number; name: string; is_system: boolean }[]
    >(`/api/operator/provisioning/roles`);
    return result.map((r) => ({
      id: r.id.toString(),
      name: r.name,
      isSystem: r.is_system,
    }));
  }
}

export const operatorProvisioningService = new OperatorProvisioningService();

/**
 * Purges the Next.js ISR cache for the given tenant subdomain slugs so stale
 * tenant-resolution does not linger (~5 min) after a slug change, delete,
 * restore, or active-flag toggle. Fire-and-forget: on failure the cache
 * self-heals within its TTL, so callers must never block success flow.
 */
export async function revalidateTenantCache(slugs: string[]): Promise<void> {
  if (slugs.length === 0) return;
  try {
    await fetch("/api/operator/provisioning/revalidate-tenant", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ slugs }),
    });
  } catch {
    // Cache self-heals in ≤5 min; don't block the success flow.
  }
}

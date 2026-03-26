import { operatorFetch } from "./api-helpers";
import type {
  BackendOrganization,
  BackendSchool,
  BackendInvitation,
  BackendSchoolAccount,
  BackendOrgAccount,
  BackendOperatorDevice,
  Organization,
  School,
  Invitation,
  SchoolAccount,
  OrgAccount,
  OperatorDevice,
  CreateOrganizationRequest,
  CreateSchoolRequest,
  CreateDeviceRequest,
  InviteAdminRequest,
  CreateAccountRequest,
  UpdateOrganizationRequest,
  UpdateSchoolRequest,
} from "./provisioning-helpers";
import {
  mapOrganization,
  mapSchool,
  mapInvitation,
  mapSchoolAccount,
  mapOrgAccount,
  mapOperatorDevice,
} from "./provisioning-helpers";

class OperatorProvisioningService {
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

  async softDeleteSchool(id: string): Promise<void> {
    await operatorFetch<null>(
      `/api/operator/provisioning/schools/${encodeURIComponent(id)}`,
      { method: "DELETE" },
    );
  }

  async restoreSchool(id: string): Promise<void> {
    await operatorFetch<null>(
      `/api/operator/provisioning/schools/${encodeURIComponent(id)}/restore`,
      { method: "POST" },
    );
  }

  async purgeSchool(id: string): Promise<Record<string, unknown>> {
    return operatorFetch<Record<string, unknown>>(
      `/api/operator/provisioning/schools/${encodeURIComponent(id)}/purge`,
      { method: "POST", body: { confirm: "PURGE" } },
    );
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

import { operatorFetch } from "./api-helpers";
import type {
  BackendOrganization,
  BackendSchool,
  BackendInvitation,
  BackendSchoolAccount,
  BackendOrgAccount,
  BackendOperatorDevice,
  BackendAssignableRole,
  BackendProvisionedAccount,
  Organization,
  School,
  Invitation,
  SchoolAccount,
  OrgAccount,
  OperatorDevice,
  AssignableRole,
  ProvisionedAccount,
  CreateOrganizationRequest,
  CreateSchoolRequest,
  CreateDeviceRequest,
  InviteAdminRequest,
  CreateSchoolAccountRequest,
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
  mapAssignableRole,
  mapProvisionedAccount,
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

  async listAssignableRoles(): Promise<AssignableRole[]> {
    const data = await operatorFetch<BackendAssignableRole[]>(
      "/api/operator/provisioning/accounts/roles",
    );
    return data.map(mapAssignableRole);
  }

  async createSchoolAccount(
    schoolId: string,
    data: CreateSchoolAccountRequest,
  ): Promise<ProvisionedAccount> {
    const result = await operatorFetch<BackendProvisionedAccount>(
      `/api/operator/provisioning/schools/${encodeURIComponent(schoolId)}/accounts`,
      { method: "POST", body: data },
    );
    return mapProvisionedAccount(result);
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
}

export const operatorProvisioningService = new OperatorProvisioningService();

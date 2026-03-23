import { operatorFetch } from "./api-helpers";
import type {
  BackendOrganization,
  BackendSchool,
  BackendInvitation,
  BackendSchoolAccount,
  Organization,
  School,
  Invitation,
  SchoolAccount,
  CreateOrganizationRequest,
  CreateSchoolRequest,
  InviteAdminRequest,
  UpdateOrganizationRequest,
  UpdateSchoolRequest,
} from "./provisioning-helpers";
import {
  mapOrganization,
  mapSchool,
  mapInvitation,
  mapSchoolAccount,
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

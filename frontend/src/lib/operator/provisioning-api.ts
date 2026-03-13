import { operatorFetch } from "./api-helpers";
import type {
  BackendOrganization,
  BackendSchool,
  BackendInvitation,
  Organization,
  School,
  Invitation,
  CreateOrganizationRequest,
  CreateSchoolRequest,
  InviteAdminRequest,
} from "./provisioning-helpers";
import {
  mapOrganization,
  mapSchool,
  mapInvitation,
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

  async inviteSchoolAdmin(
    schoolId: string,
    data: InviteAdminRequest,
  ): Promise<Invitation> {
    const result = await operatorFetch<BackendInvitation>(
      `/api/operator/provisioning/schools/${schoolId}/invite-admin`,
      { method: "POST", body: data },
    );
    return mapInvitation(result);
  }
}

export const operatorProvisioningService = new OperatorProvisioningService();

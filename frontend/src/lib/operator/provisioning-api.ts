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
  async fetchOrganizations(): Promise<Organization[]> {
    const data = await operatorFetch<BackendOrganization[]>(
      "/api/operator/organizations",
    );
    return data.map(mapOrganization);
  }

  async createOrganization(
    req: CreateOrganizationRequest,
  ): Promise<Organization> {
    const data = await operatorFetch<BackendOrganization>(
      "/api/operator/organizations",
      { method: "POST", body: req },
    );
    return mapOrganization(data);
  }

  async fetchSchools(): Promise<School[]> {
    const data = await operatorFetch<BackendSchool[]>("/api/operator/schools");
    return data.map(mapSchool);
  }

  async createSchool(req: CreateSchoolRequest): Promise<School> {
    const data = await operatorFetch<BackendSchool>("/api/operator/schools", {
      method: "POST",
      body: req,
    });
    return mapSchool(data);
  }

  async inviteSchoolAdmin(
    schoolId: string,
    req: InviteAdminRequest,
  ): Promise<Invitation> {
    const data = await operatorFetch<BackendInvitation>(
      `/api/operator/schools/${schoolId}/invite-admin`,
      { method: "POST", body: req },
    );
    return mapInvitation(data);
  }
}

export const operatorProvisioningService = new OperatorProvisioningService();

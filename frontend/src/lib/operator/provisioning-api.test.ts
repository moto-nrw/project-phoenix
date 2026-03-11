import { describe, it, expect, vi, beforeEach } from "vitest";

const { mockOperatorFetch } = vi.hoisted(() => ({
  mockOperatorFetch: vi.fn(),
}));

vi.mock("./api-helpers", () => ({
  operatorFetch: mockOperatorFetch,
}));

vi.mock("./provisioning-helpers", () => ({
  mapOrganization: vi.fn((data: { id: number }) => ({
    ...data,
    id: String(data.id),
  })),
  mapSchool: vi.fn((data: { id: number }) => ({
    ...data,
    id: String(data.id),
  })),
  mapInvitation: vi.fn((data: { id: number }) => ({
    ...data,
    id: String(data.id),
  })),
}));

import { operatorProvisioningService } from "./provisioning-api";
import {
  mapOrganization,
  mapSchool,
  mapInvitation,
} from "./provisioning-helpers";

describe("OperatorProvisioningService", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("fetchOrganizations", () => {
    it("fetches organizations and maps them", async () => {
      const backendOrgs = [
        { id: 1, name: "Org A" },
        { id: 2, name: "Org B" },
      ];
      mockOperatorFetch.mockResolvedValue(backendOrgs);

      const result = await operatorProvisioningService.fetchOrganizations();

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/organizations",
      );
      expect(mapOrganization).toHaveBeenCalledTimes(2);
      expect(result).toHaveLength(2);
    });
  });

  describe("createOrganization", () => {
    it("posts to organizations endpoint and maps result", async () => {
      const backendOrg = { id: 1, name: "New Org", slug: "new-org" };
      mockOperatorFetch.mockResolvedValue(backendOrg);

      const req = { name: "New Org", slug: "new-org" };
      const result = await operatorProvisioningService.createOrganization(req);

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/organizations",
        { method: "POST", body: req },
      );
      expect(mapOrganization).toHaveBeenCalledWith(backendOrg);
      expect(result.id).toBe("1");
    });
  });

  describe("fetchSchools", () => {
    it("fetches schools and maps them", async () => {
      const backendSchools = [{ id: 1, name: "School A" }];
      mockOperatorFetch.mockResolvedValue(backendSchools);

      const result = await operatorProvisioningService.fetchSchools();

      expect(mockOperatorFetch).toHaveBeenCalledWith("/api/operator/schools");
      expect(mapSchool).toHaveBeenCalledTimes(1);
      expect(result).toHaveLength(1);
    });
  });

  describe("createSchool", () => {
    it("posts to schools endpoint and maps result", async () => {
      const backendSchool = { id: 5, name: "Test School" };
      mockOperatorFetch.mockResolvedValue(backendSchool);

      const req = {
        organization_id: 1,
        name: "Test School",
        slug: "test-school",
        subdomain: "test-school",
      };
      const result = await operatorProvisioningService.createSchool(req);

      expect(mockOperatorFetch).toHaveBeenCalledWith("/api/operator/schools", {
        method: "POST",
        body: req,
      });
      expect(mapSchool).toHaveBeenCalledWith(backendSchool);
      expect(result.id).toBe("5");
    });
  });

  describe("inviteSchoolAdmin", () => {
    it("posts to invite-admin endpoint and maps result", async () => {
      const backendInvitation = {
        id: 10,
        email: "admin@test.de",
        delivery_status: "sent",
      };
      mockOperatorFetch.mockResolvedValue(backendInvitation);

      const req = { email: "admin@test.de" };
      const result = await operatorProvisioningService.inviteSchoolAdmin(
        "5",
        req,
      );

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/schools/5/invite-admin",
        { method: "POST", body: req },
      );
      expect(mapInvitation).toHaveBeenCalledWith(backendInvitation);
      expect(result.id).toBe("10");
    });
  });
});

import { describe, it, expect, vi, beforeEach } from "vitest";

const { mockOperatorFetch } = vi.hoisted(() => ({
  mockOperatorFetch: vi.fn(),
}));

vi.mock("./api-helpers", () => ({
  operatorFetch: mockOperatorFetch,
}));

import { operatorProvisioningService } from "./provisioning-api";
import type {
  BackendOrganization,
  BackendSchool,
} from "./provisioning-helpers";

const NOW = "2026-03-15T10:00:00Z";

const mockBackendOrg: BackendOrganization = {
  id: 5,
  name: "Stadt Köln",
  slug: "stadt-koeln",
  active: true,
  settings: null,
  created_at: NOW,
  updated_at: NOW,
};

const mockBackendSchool: BackendSchool = {
  id: 10,
  organization_id: 5,
  name: "GGS Europaschule",
  slug: "ggs-europa",
  subdomain: "ggs-europa",
  address: "Hauptstr. 1",
  city: "Köln",
  zip: "50667",
  phone: "0221123456",
  email: "info@ggs-europa.de",
  active: true,
  settings: null,
  created_at: NOW,
  updated_at: NOW,
};

describe("OperatorProvisioningService", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe("updateOrganization", () => {
    it("calls correct endpoint with PUT method", async () => {
      mockOperatorFetch.mockResolvedValue(mockBackendOrg);

      const updateData = { name: "Updated", slug: "updated", active: true };
      await operatorProvisioningService.updateOrganization("5", updateData);

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/organizations/5",
        { method: "PUT", body: updateData },
      );
    });

    it("maps response data correctly", async () => {
      mockOperatorFetch.mockResolvedValue(mockBackendOrg);

      const result = await operatorProvisioningService.updateOrganization("5", {
        name: "Stadt Köln",
        slug: "stadt-koeln",
        active: true,
      });

      expect(result).toEqual({
        id: "5",
        name: "Stadt Köln",
        slug: "stadt-koeln",
        active: true,
        createdAt: NOW,
        updatedAt: NOW,
      });
    });
  });

  describe("updateSchool", () => {
    it("calls correct endpoint with PUT method", async () => {
      mockOperatorFetch.mockResolvedValue(mockBackendSchool);

      const updateData = {
        organization_id: 5,
        name: "Updated School",
        slug: "updated-school",
        subdomain: "updated-school",
        address: "Street 1",
        city: "Berlin",
        zip: "10115",
        phone: "030123456",
        email: "school@example.com",
        active: true,
      };
      await operatorProvisioningService.updateSchool("10", updateData);

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/10",
        { method: "PUT", body: updateData },
      );
    });

    it("maps response data correctly", async () => {
      mockOperatorFetch.mockResolvedValue(mockBackendSchool);

      const result = await operatorProvisioningService.updateSchool("10", {
        organization_id: 5,
        name: "GGS Europaschule",
        slug: "ggs-europa",
        subdomain: "ggs-europa",
        address: "Hauptstr. 1",
        city: "Köln",
        zip: "50667",
        phone: "0221123456",
        email: "info@ggs-europa.de",
        active: true,
      });

      expect(result).toEqual({
        id: "10",
        organizationId: "5",
        name: "GGS Europaschule",
        slug: "ggs-europa",
        subdomain: "ggs-europa",
        address: "Hauptstr. 1",
        city: "Köln",
        zip: "50667",
        phone: "0221123456",
        email: "info@ggs-europa.de",
        active: true,
        createdAt: NOW,
        updatedAt: NOW,
        organization: undefined,
      });
    });
  });

  describe("listOrganizations", () => {
    it("calls correct endpoint", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendOrg]);

      const result = await operatorProvisioningService.listOrganizations();

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/organizations",
      );
      expect(result).toHaveLength(1);
      expect(result[0]!.id).toBe("5");
    });
  });

  describe("listSchools", () => {
    it("calls correct endpoint", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendSchool]);

      const result = await operatorProvisioningService.listSchools();

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools",
      );
      expect(result).toHaveLength(1);
      expect(result[0]!.id).toBe("10");
    });
  });

  describe("createOrganization", () => {
    it("calls correct endpoint with POST method", async () => {
      mockOperatorFetch.mockResolvedValue(mockBackendOrg);

      await operatorProvisioningService.createOrganization({
        name: "Stadt Köln",
        slug: "stadt-koeln",
      });

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/organizations",
        {
          method: "POST",
          body: { name: "Stadt Köln", slug: "stadt-koeln" },
        },
      );
    });
  });

  describe("createSchool", () => {
    it("calls correct endpoint with POST method", async () => {
      mockOperatorFetch.mockResolvedValue(mockBackendSchool);

      await operatorProvisioningService.createSchool({
        organization_id: 5,
        name: "GGS Europaschule",
        slug: "ggs-europa",
        subdomain: "ggs-europa",
      });

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools",
        {
          method: "POST",
          body: {
            organization_id: 5,
            name: "GGS Europaschule",
            slug: "ggs-europa",
            subdomain: "ggs-europa",
          },
        },
      );
    });
  });

  describe("inviteSchoolAdmin", () => {
    it("calls correct endpoint with POST method", async () => {
      mockOperatorFetch.mockResolvedValue({
        id: 1,
        email: "admin@example.com",
        role_id: 4,
        role_name: "admin",
        expires_at: NOW,
        created_by: 0,
        delivery_status: "pending",
        email_retry_count: 0,
      });

      await operatorProvisioningService.inviteSchoolAdmin("10", {
        email: "admin@example.com",
        first_name: "Ada",
        last_name: "Lovelace",
      });

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/10/invite-admin",
        {
          method: "POST",
          body: {
            email: "admin@example.com",
            first_name: "Ada",
            last_name: "Lovelace",
          },
        },
      );
    });
  });
});

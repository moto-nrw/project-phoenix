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
  BackendSchoolAccount,
  BackendOrgAccount,
  BackendOperatorDevice,
  BackendInvitation,
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
  hidden: false,
  deleted_at: null,
  settings: null,
  created_at: NOW,
  updated_at: NOW,
};

const mockBackendSchoolAccount: BackendSchoolAccount = {
  account_id: 1,
  email: "teacher@school.de",
  active: true,
  first_name: "Maria",
  last_name: "Schmidt",
  role_name: "teacher",
  pedagogic_role: "Erzieher",
  status: "active",
};

const mockBackendOrgAccount: BackendOrgAccount = {
  ...mockBackendSchoolAccount,
  school_id: 10,
  school_name: "GGS Europaschule",
};

const mockBackendDevice: BackendOperatorDevice = {
  id: 1,
  device_id: "dev-001",
  device_type: "terminal",
  name: "Eingang Hauptgebäude",
  status: "active",
  api_key: undefined,
  masked_api_key: "dk_...abc",
  last_seen: NOW,
  is_online: true,
  school_id: 10,
  school_name: "GGS Europaschule",
  organization_id: 5,
  organization_name: "Stadt Köln",
  created_at: NOW,
  updated_at: NOW,
};

const mockBackendInvitation: BackendInvitation = {
  id: 1,
  email: "admin@example.com",
  role_id: 4,
  role_name: "admin",
  expires_at: NOW,
  first_name: "Ada",
  last_name: "Lovelace",
  created_by: 0,
  delivery_status: "pending",
  email_retry_count: 0,
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
        hidden: false,
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
        hidden: false,
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
        hidden: false,
        deletedAt: null,
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

  describe("listSchoolAccounts", () => {
    it("calls correct endpoint with school id", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendSchoolAccount]);

      await operatorProvisioningService.listSchoolAccounts("10");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/10/accounts",
      );
    });

    it("encodes school id in URL", async () => {
      mockOperatorFetch.mockResolvedValue([]);

      await operatorProvisioningService.listSchoolAccounts("10/evil");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/10%2Fevil/accounts",
      );
    });

    it("maps response data correctly", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendSchoolAccount]);

      const result = await operatorProvisioningService.listSchoolAccounts("10");

      expect(result).toHaveLength(1);
      expect(result[0]).toEqual({
        accountId: "1",
        email: "teacher@school.de",
        active: true,
        firstName: "Maria",
        lastName: "Schmidt",
        roleName: "teacher",
        pedagogicRole: "Erzieher",
        status: "active",
      });
    });
  });

  describe("listOrganizationAccounts", () => {
    it("calls correct endpoint with org id", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendOrgAccount]);

      await operatorProvisioningService.listOrganizationAccounts("5");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/organizations/5/accounts",
      );
    });

    it("maps response data correctly", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendOrgAccount]);

      const result =
        await operatorProvisioningService.listOrganizationAccounts("5");

      expect(result).toHaveLength(1);
      expect(result[0]).toEqual({
        accountId: "1",
        email: "teacher@school.de",
        active: true,
        firstName: "Maria",
        lastName: "Schmidt",
        roleName: "teacher",
        pedagogicRole: "Erzieher",
        status: "active",
        schoolId: "10",
        schoolName: "GGS Europaschule",
      });
    });
  });

  describe("listAllAccounts", () => {
    it("calls correct endpoint", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendOrgAccount]);

      await operatorProvisioningService.listAllAccounts();

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/accounts",
      );
    });

    it("maps response data correctly", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendOrgAccount]);

      const result = await operatorProvisioningService.listAllAccounts();

      expect(result).toHaveLength(1);
      expect(result[0]!.schoolId).toBe("10");
      expect(result[0]!.schoolName).toBe("GGS Europaschule");
    });
  });

  describe("listAllDevices", () => {
    it("calls correct endpoint", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendDevice]);

      await operatorProvisioningService.listAllDevices();

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/devices",
      );
    });

    it("maps response data correctly", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendDevice]);

      const result = await operatorProvisioningService.listAllDevices();

      expect(result).toHaveLength(1);
      expect(result[0]).toEqual({
        id: "1",
        deviceId: "dev-001",
        deviceType: "terminal",
        name: "Eingang Hauptgebäude",
        status: "active",
        apiKey: "",
        maskedApiKey: "dk_...abc",
        lastSeen: NOW,
        isOnline: true,
        schoolId: "10",
        schoolName: "GGS Europaschule",
        organizationId: "5",
        organizationName: "Stadt Köln",
        createdAt: NOW,
        updatedAt: NOW,
      });
    });
  });

  describe("listSchoolDevices", () => {
    it("calls correct endpoint with school id", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendDevice]);

      await operatorProvisioningService.listSchoolDevices("10");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/10/devices",
      );
    });

    it("encodes school id in URL", async () => {
      mockOperatorFetch.mockResolvedValue([]);

      await operatorProvisioningService.listSchoolDevices("10/evil");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/10%2Fevil/devices",
      );
    });

    it("maps response data correctly", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendDevice]);

      const result = await operatorProvisioningService.listSchoolDevices("10");

      expect(result).toHaveLength(1);
      expect(result[0]!.deviceId).toBe("dev-001");
    });
  });

  describe("listOrganizationDevices", () => {
    it("calls correct endpoint with org id", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendDevice]);

      await operatorProvisioningService.listOrganizationDevices("5");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/organizations/5/devices",
      );
    });

    it("maps response data correctly", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendDevice]);

      const result =
        await operatorProvisioningService.listOrganizationDevices("5");

      expect(result).toHaveLength(1);
      expect(result[0]!.organizationId).toBe("5");
    });
  });

  describe("inviteSchoolAdmin", () => {
    it("calls correct endpoint with POST method", async () => {
      mockOperatorFetch.mockResolvedValue(mockBackendInvitation);

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

    it("maps invitation response correctly", async () => {
      mockOperatorFetch.mockResolvedValue(mockBackendInvitation);

      const result = await operatorProvisioningService.inviteSchoolAdmin("10", {
        email: "admin@example.com",
        first_name: "Ada",
        last_name: "Lovelace",
      });

      expect(result).toEqual({
        id: "1",
        email: "admin@example.com",
        roleId: "4",
        roleName: "admin",
        expiresAt: NOW,
        firstName: "Ada",
        lastName: "Lovelace",
        position: null,
        createdBy: "0",
        creator: "",
        deliveryStatus: "pending",
        emailSentAt: null,
        emailError: null,
        emailRetryCount: 0,
      });
    });
  });

  describe("createDevice", () => {
    it("calls correct endpoint with POST method", async () => {
      mockOperatorFetch.mockResolvedValue(mockBackendDevice);

      const createData = {
        school_id: 10,
        device_id: "DEV-001",
        device_type: "terminal",
        name: "Eingang Hauptgebäude",
        api_key: "manual-key",
      };
      await operatorProvisioningService.createDevice(createData);

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/devices",
        { method: "POST", body: createData },
      );
    });

    it("maps response data correctly", async () => {
      mockOperatorFetch.mockResolvedValue({
        ...mockBackendDevice,
        api_key: "manual-key",
      });

      const result = await operatorProvisioningService.createDevice({
        school_id: 10,
        device_id: "DEV-001",
        device_type: "terminal",
      });

      expect(result).toEqual({
        id: "1",
        deviceId: "dev-001",
        deviceType: "terminal",
        name: "Eingang Hauptgebäude",
        status: "active",
        apiKey: "manual-key",
        maskedApiKey: "dk_...abc",
        lastSeen: NOW,
        isOnline: true,
        schoolId: "10",
        schoolName: "GGS Europaschule",
        organizationId: "5",
        organizationName: "Stadt Köln",
        createdAt: NOW,
        updatedAt: NOW,
      });
    });
  });

  describe("setDeviceAPIKey", () => {
    it("encodes the device id and sends manual api key", async () => {
      mockOperatorFetch.mockResolvedValue(mockBackendDevice);

      await operatorProvisioningService.setDeviceAPIKey(
        "1/unsafe",
        "manual-key",
      );

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/devices/1%2Funsafe/set-api-key",
        { method: "POST", body: { api_key: "manual-key" } },
      );
    });

    it("sends an empty body when auto-generating a key", async () => {
      mockOperatorFetch.mockResolvedValue({
        ...mockBackendDevice,
        api_key: "generated-key",
      });

      const result = await operatorProvisioningService.setDeviceAPIKey("1");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/devices/1/set-api-key",
        { method: "POST", body: {} },
      );
      expect(result.apiKey).toBe("generated-key");
    });
  });

  describe("createSchoolAccount", () => {
    const mockAccountData = {
      email: "new@school.de",
      first_name: "Max",
      last_name: "Mustermann",
      password: "SecurePass123!",
      confirm_password: "SecurePass123!",
    };

    it("calls correct endpoint with POST method", async () => {
      mockOperatorFetch.mockResolvedValue({ id: 99, email: "new@school.de" });

      await operatorProvisioningService.createSchoolAccount(
        "10",
        mockAccountData,
      );

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/10/create-account",
        { method: "POST", body: mockAccountData },
      );
    });

    it("encodes school id in URL", async () => {
      mockOperatorFetch.mockResolvedValue({ id: 99, email: "new@school.de" });

      await operatorProvisioningService.createSchoolAccount(
        "10/evil",
        mockAccountData,
      );

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/10%2Fevil/create-account",
        expect.any(Object),
      );
    });

    it("maps response data correctly", async () => {
      mockOperatorFetch.mockResolvedValue({ id: 99, email: "new@school.de" });

      const result = await operatorProvisioningService.createSchoolAccount(
        "10",
        mockAccountData,
      );

      expect(result).toEqual({
        id: "99",
        email: "new@school.de",
      });
    });
  });

  describe("listSystemRoles", () => {
    it("calls correct endpoint", async () => {
      mockOperatorFetch.mockResolvedValue([
        { id: 1, name: "admin", is_system: true },
      ]);

      await operatorProvisioningService.listSystemRoles();

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/roles",
      );
    });

    it("maps response data correctly", async () => {
      mockOperatorFetch.mockResolvedValue([
        { id: 1, name: "admin", is_system: true },
        { id: 2, name: "teacher", is_system: true },
        { id: 3, name: "custom_role", is_system: false },
      ]);

      const result = await operatorProvisioningService.listSystemRoles();

      expect(result).toHaveLength(3);
      expect(result[0]).toEqual({
        id: "1",
        name: "admin",
        isSystem: true,
      });
      expect(result[1]).toEqual({
        id: "2",
        name: "teacher",
        isSystem: true,
      });
      expect(result[2]).toEqual({
        id: "3",
        name: "custom_role",
        isSystem: false,
      });
    });
  });
});

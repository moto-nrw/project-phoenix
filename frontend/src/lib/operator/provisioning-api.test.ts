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
  BackendOperatorPerson,
  BackendUnregisteredTagScan,
} from "./provisioning-helpers";

const NOW = "2026-03-15T10:00:00Z";

const mockBackendOrg: BackendOrganization = {
  id: 5,
  name: "Stadt Köln",
  slug: "stadt-koeln",
  active: true,
  deleted_at: null,
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

const mockBackendUnregisteredTagScan: BackendUnregisteredTagScan = {
  id: 300,
  tenant_id: 10,
  tag_uid: "04AABBCCDD",
  device_id: 1,
  scanned_at: NOW,
  resolved_at: null,
  resolved_by_operator_id: null,
  resolution_note: null,
  created_at: NOW,
  updated_at: NOW,
  school_id: 10,
  school_name: "GGS Europaschule",
  organization_id: 5,
  organization_name: "Stadt Köln",
  device_identifier: "dev-001",
  device_name: "Eingang Hauptgebäude",
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
        deletedAt: null,
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
        hasAdminRole: false,
        hasUserRole: false,
        hasCaregiverProfile: false,
        isActiveCaregiver: false,
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
        hasAdminRole: false,
        hasUserRole: false,
        hasCaregiverProfile: false,
        isActiveCaregiver: false,
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
        caregiver_enabled: true,
      });

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/10/invite-admin",
        {
          method: "POST",
          body: {
            email: "admin@example.com",
            first_name: "Ada",
            last_name: "Lovelace",
            caregiver_enabled: true,
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
        caregiverEnabled: false,
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

  describe("device transfer", () => {
    it("loads and maps transfer blockers", async () => {
      mockOperatorFetch.mockResolvedValue({
        can_transfer: false,
        is_online: true,
        last_seen: NOW,
        active_session: {
          id: 77,
          started_at: NOW,
          activity_name: "Mensa",
          room_name: "Speisesaal",
        },
      });

      const result =
        await operatorProvisioningService.getDeviceTransferStatus("1/unsafe");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/devices/1%2Funsafe/transfer-status",
      );
      expect(result).toEqual({
        canTransfer: false,
        isOnline: true,
        lastSeen: NOW,
        activeSession: {
          id: "77",
          startedAt: NOW,
          activityName: "Mensa",
          roomName: "Speisesaal",
        },
      });
    });

    it("posts the numeric target school and maps the transferred device", async () => {
      mockOperatorFetch.mockResolvedValue({
        ...mockBackendDevice,
        school_id: 20,
        school_name: "Walbach",
      });

      const result = await operatorProvisioningService.transferDevice(
        "1/unsafe",
        "20",
      );

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/devices/1%2Funsafe/transfer",
        { method: "POST", body: { target_school_id: 20 } },
      );
      expect(result.schoolId).toBe("20");
      expect(result.schoolName).toBe("Walbach");
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

  describe("listSchoolPersons", () => {
    const mockBackendPerson: BackendOperatorPerson = {
      id: 42,
      first_name: "Max",
      last_name: "Mustermann",
      has_account: true,
      account_email: "max@test.de",
      has_rfid_card: true,
      is_staff: true,
      is_student: false,
      school_id: 10,
      school_name: "GGS Europaschule",
      organization_id: 5,
      organization_name: "Stadt Köln",
      created_at: NOW,
    };

    it("calls correct endpoint with school id", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendPerson]);

      await operatorProvisioningService.listSchoolPersons("10");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/10/persons",
      );
    });

    it("encodes school id in URL", async () => {
      mockOperatorFetch.mockResolvedValue([]);

      await operatorProvisioningService.listSchoolPersons("10/evil");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/10%2Fevil/persons",
      );
    });

    it("maps response data correctly", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendPerson]);

      const result = await operatorProvisioningService.listSchoolPersons("10");

      expect(result).toHaveLength(1);
      expect(result[0]).toEqual({
        id: "42",
        firstName: "Max",
        lastName: "Mustermann",
        fullName: "Max Mustermann",
        hasAccount: true,
        accountEmail: "max@test.de",
        hasRfidCard: true,
        isStaff: true,
        isStudent: false,
        schoolId: "10",
        schoolName: "GGS Europaschule",
        organizationId: "5",
        organizationName: "Stadt Köln",
        createdAt: NOW,
      });
    });

    it("returns empty array when no persons", async () => {
      mockOperatorFetch.mockResolvedValue([]);

      const result = await operatorProvisioningService.listSchoolPersons("10");

      expect(result).toEqual([]);
    });
  });

  describe("softDeletePerson", () => {
    it("calls correct endpoint with DELETE method", async () => {
      mockOperatorFetch.mockResolvedValue(null);

      await operatorProvisioningService.softDeletePerson("42");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/persons/42",
        { method: "DELETE" },
      );
    });

    it("encodes person id in URL", async () => {
      mockOperatorFetch.mockResolvedValue(null);

      await operatorProvisioningService.softDeletePerson("42/evil");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/persons/42%2Fevil",
        { method: "DELETE" },
      );
    });

    it("returns void on success", async () => {
      mockOperatorFetch.mockResolvedValue(null);

      const result = await operatorProvisioningService.softDeletePerson("42");

      expect(result).toBeUndefined();
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

  describe("softDeleteSchool", () => {
    it("calls correct endpoint with DELETE method", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorProvisioningService.softDeleteSchool("10");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/10",
        { method: "DELETE" },
      );
    });

    it("encodes school id in URL", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorProvisioningService.softDeleteSchool("10/evil");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/10%2Fevil",
        { method: "DELETE" },
      );
    });

    it("resolves without returning data", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      const result = await operatorProvisioningService.softDeleteSchool("10");

      expect(result).toBeUndefined();
    });
  });

  describe("restoreSchool", () => {
    it("calls correct endpoint with POST method", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorProvisioningService.restoreSchool("10");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/10/restore",
        { method: "POST" },
      );
    });

    it("encodes school id in URL", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorProvisioningService.restoreSchool("10/evil");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/10%2Fevil/restore",
        { method: "POST" },
      );
    });

    it("resolves without returning data", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      const result = await operatorProvisioningService.restoreSchool("10");

      expect(result).toBeUndefined();
    });
  });

  describe("softDeleteOrganization", () => {
    it("calls correct endpoint with DELETE method", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorProvisioningService.softDeleteOrganization("77");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/organizations/77",
        { method: "DELETE" },
      );
    });

    it("encodes organization id in URL", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorProvisioningService.softDeleteOrganization("77/evil");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/organizations/77%2Fevil",
        { method: "DELETE" },
      );
    });
  });

  describe("restoreOrganization", () => {
    it("calls correct endpoint with POST method", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorProvisioningService.restoreOrganization("77");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/organizations/77/restore",
        { method: "POST" },
      );
    });

    it("encodes organization id in URL", async () => {
      mockOperatorFetch.mockResolvedValue(undefined);

      await operatorProvisioningService.restoreOrganization("77/evil");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/organizations/77%2Fevil/restore",
        { method: "POST" },
      );
    });
  });

  describe("getStats", () => {
    it("hits the stats endpoint and maps snake_case to camelCase", async () => {
      mockOperatorFetch.mockResolvedValue({
        traeger_count: 3,
        schulen_count: 12,
        konten_count: 250,
        geraete_count: 64,
      });

      const stats = await operatorProvisioningService.getStats();

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/stats",
      );
      expect(stats).toEqual({
        traegerCount: 3,
        schulenCount: 12,
        kontenCount: 250,
        geraeteCount: 64,
      });
    });

    it("propagates errors from operatorFetch", async () => {
      mockOperatorFetch.mockRejectedValue(new Error("network down"));

      await expect(operatorProvisioningService.getStats()).rejects.toThrow(
        "network down",
      );
    });
  });

  describe("listOrganizationSummaries", () => {
    it("calls correct endpoint and maps each row", async () => {
      mockOperatorFetch.mockResolvedValue([
        {
          ...mockBackendOrg,
          schulen_count: 3,
          konten_count: 24,
          geraete_count: 7,
          personen_count: 100,
        },
      ]);

      const summaries =
        await operatorProvisioningService.listOrganizationSummaries();

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/organizations/summaries",
      );
      expect(summaries).toHaveLength(1);
      expect(summaries[0]).toMatchObject({
        id: mockBackendOrg.id.toString(),
        slug: mockBackendOrg.slug,
        name: mockBackendOrg.name,
        schulenCount: 3,
        kontenCount: 24,
        geraeteCount: 7,
        personenCount: 100,
      });
    });
  });

  describe("listSchoolSummaries", () => {
    it("calls correct endpoint and maps each row", async () => {
      mockOperatorFetch.mockResolvedValue([
        {
          ...mockBackendSchool,
          konten_count: 12,
          geraete_count: 4,
          personen_count: 80,
        },
      ]);

      const summaries = await operatorProvisioningService.listSchoolSummaries();

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/schools/summaries",
      );
      expect(summaries).toHaveLength(1);
      expect(summaries[0]).toMatchObject({
        id: mockBackendSchool.id.toString(),
        organizationId: mockBackendSchool.organization_id.toString(),
        name: mockBackendSchool.name,
        kontenCount: 12,
        geraeteCount: 4,
        personenCount: 80,
      });
    });
  });

  describe("listOrganizationSchools", () => {
    it("calls the org-scoped schools endpoint with the encoded id", async () => {
      mockOperatorFetch.mockResolvedValue([]);

      await operatorProvisioningService.listOrganizationSchools("42");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/organizations/42/schools",
      );
    });

    it("URL-encodes the org id", async () => {
      mockOperatorFetch.mockResolvedValue([]);

      await operatorProvisioningService.listOrganizationSchools("a/b c");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/organizations/a%2Fb%20c/schools",
      );
    });

    it("maps each returned school summary", async () => {
      mockOperatorFetch.mockResolvedValue([
        {
          ...mockBackendSchool,
          konten_count: 5,
          geraete_count: 2,
          personen_count: 40,
        },
      ]);

      const result =
        await operatorProvisioningService.listOrganizationSchools("42");

      expect(result).toHaveLength(1);
      expect(result[0]).toMatchObject({
        id: mockBackendSchool.id.toString(),
        kontenCount: 5,
        geraeteCount: 2,
        personenCount: 40,
      });
    });
  });

  describe("listOrganizationPersons", () => {
    it("calls the org-scoped persons endpoint with the encoded id", async () => {
      mockOperatorFetch.mockResolvedValue([]);

      await operatorProvisioningService.listOrganizationPersons("42");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/organizations/42/persons",
      );
    });

    it("URL-encodes the org id", async () => {
      mockOperatorFetch.mockResolvedValue([]);

      await operatorProvisioningService.listOrganizationPersons("a/b c");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/organizations/a%2Fb%20c/persons",
      );
    });

    it("maps each returned person", async () => {
      const backendPerson: BackendOperatorPerson = {
        id: 5,
        first_name: "Anna",
        last_name: "Beispiel",
        has_account: true,
        account_email: "anna@example.com",
        has_rfid_card: false,
        is_staff: true,
        is_student: false,
        school_id: 10,
        school_name: "Schule A",
        organization_id: 42,
        organization_name: "Träger A",
        created_at: NOW,
      };
      mockOperatorFetch.mockResolvedValue([backendPerson]);

      const result =
        await operatorProvisioningService.listOrganizationPersons("42");

      expect(result).toHaveLength(1);
      expect(result[0]).toMatchObject({
        id: "5",
        firstName: "Anna",
        lastName: "Beispiel",
        fullName: "Anna Beispiel",
        isStaff: true,
        isStudent: false,
        hasAccount: true,
        accountEmail: "anna@example.com",
        schoolId: "10",
        organizationId: "42",
      });
    });
  });

  describe("listUnregisteredTagScans", () => {
    it("calls the base endpoint without filters and maps rows", async () => {
      mockOperatorFetch.mockResolvedValue([mockBackendUnregisteredTagScan]);

      const result = await operatorProvisioningService.listUnregisteredTagScans(
        {},
      );

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/unregistered-tag-scans",
      );
      expect(result).toEqual([
        {
          id: "300",
          tenantId: "10",
          tagUid: "04AABBCCDD",
          deviceId: "1",
          scannedAt: NOW,
          resolvedAt: null,
          resolvedByOperatorId: null,
          resolutionNote: null,
          createdAt: NOW,
          updatedAt: NOW,
          schoolId: "10",
          schoolName: "GGS Europaschule",
          organizationId: "5",
          organizationName: "Stadt Köln",
          deviceIdentifier: "dev-001",
          deviceName: "Eingang Hauptgebäude",
        },
      ]);
    });

    it("adds organization, school, and all-resolved filters", async () => {
      mockOperatorFetch.mockResolvedValue([]);

      await operatorProvisioningService.listUnregisteredTagScans({
        organizationId: "5",
        schoolId: "10",
        resolved: "all",
      });

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/unregistered-tag-scans?organization_id=5&school_id=10&resolved=all",
      );
    });

    it("omits resolved query param for unresolved-only mode", async () => {
      mockOperatorFetch.mockResolvedValue([]);

      await operatorProvisioningService.listUnregisteredTagScans({
        organizationId: "5",
        resolved: "unresolved",
      });

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/unregistered-tag-scans?organization_id=5",
      );
    });
  });

  describe("resolveUnregisteredTagScan", () => {
    it("posts an empty body when no note is supplied", async () => {
      mockOperatorFetch.mockResolvedValue(mockBackendUnregisteredTagScan);

      await operatorProvisioningService.resolveUnregisteredTagScan("300");

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/unregistered-tag-scans/300/resolve",
        { method: "POST", body: {} },
      );
    });

    it("posts the note and URL-encodes the scan id", async () => {
      mockOperatorFetch.mockResolvedValue({
        ...mockBackendUnregisteredTagScan,
        resolved_at: NOW,
      });

      await operatorProvisioningService.resolveUnregisteredTagScan(
        "300/unsafe",
        "Assigned to new card",
      );

      expect(mockOperatorFetch).toHaveBeenCalledWith(
        "/api/operator/provisioning/unregistered-tag-scans/300%2Funsafe/resolve",
        { method: "POST", body: { note: "Assigned to new card" } },
      );
    });
  });
});

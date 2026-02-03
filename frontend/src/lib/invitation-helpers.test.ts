import { describe, it, expect } from "vitest";
import type {
  BackendInvitationValidation,
  BackendInvitation,
} from "./invitation-helpers";
import {
  mapInvitationValidationResponse,
  mapPendingInvitationResponse,
} from "./invitation-helpers";

describe("invitation-helpers", () => {
  describe("mapInvitationValidationResponse", () => {
    it("should map all fields from backend to frontend format", () => {
      const backendData: BackendInvitationValidation = {
        email: "test@example.com",
        role_name: "Teacher",
        first_name: "John",
        last_name: "Doe",
        position: "Math Teacher",
        expires_at: "2024-12-31T23:59:59Z",
      };

      const result = mapInvitationValidationResponse(backendData);

      expect(result).toEqual({
        email: "test@example.com",
        roleName: "Teacher",
        firstName: "John",
        lastName: "Doe",
        position: "Math Teacher",
        expiresAt: "2024-12-31T23:59:59Z",
      });
    });

    it("should handle optional fields as null", () => {
      const backendData: BackendInvitationValidation = {
        email: "test@example.com",
        role_name: "Teacher",
        first_name: null,
        last_name: null,
        position: null,
        expires_at: "2024-12-31T23:59:59Z",
      };

      const result = mapInvitationValidationResponse(backendData);

      expect(result).toEqual({
        email: "test@example.com",
        roleName: "Teacher",
        firstName: null,
        lastName: null,
        position: null,
        expiresAt: "2024-12-31T23:59:59Z",
      });
    });

    it("should handle optional fields as undefined", () => {
      const backendData: BackendInvitationValidation = {
        email: "test@example.com",
        role_name: "Teacher",
        first_name: undefined,
        last_name: undefined,
        position: undefined,
        expires_at: "2024-12-31T23:59:59Z",
      };

      const result = mapInvitationValidationResponse(backendData);

      expect(result).toEqual({
        email: "test@example.com",
        roleName: "Teacher",
        firstName: undefined,
        lastName: undefined,
        position: undefined,
        expiresAt: "2024-12-31T23:59:59Z",
      });
    });

    it("should handle mix of present and null optional fields", () => {
      const backendData: BackendInvitationValidation = {
        email: "test@example.com",
        role_name: "Admin",
        first_name: "Jane",
        last_name: null,
        position: "System Administrator",
        expires_at: "2024-06-30T12:00:00Z",
      };

      const result = mapInvitationValidationResponse(backendData);

      expect(result).toEqual({
        email: "test@example.com",
        roleName: "Admin",
        firstName: "Jane",
        lastName: null,
        position: "System Administrator",
        expiresAt: "2024-06-30T12:00:00Z",
      });
    });

    it("should preserve special characters in fields", () => {
      const backendData: BackendInvitationValidation = {
        email: "test+special@example.com",
        role_name: "Teacher (Primary)",
        first_name: "María",
        last_name: "O'Brien",
        position: "Math & Science",
        expires_at: "2024-12-31T23:59:59Z",
      };

      const result = mapInvitationValidationResponse(backendData);

      expect(result.email).toBe("test+special@example.com");
      expect(result.roleName).toBe("Teacher (Primary)");
      expect(result.firstName).toBe("María");
      expect(result.lastName).toBe("O'Brien");
      expect(result.position).toBe("Math & Science");
    });
  });

  describe("mapPendingInvitationResponse", () => {
    it("should map all fields from backend to frontend format", () => {
      const backendData: BackendInvitation = {
        id: 123,
        email: "teacher@school.com",
        role_id: 5,
        role_name: "Teacher",
        token: "abc123xyz",
        expires_at: "2024-12-31T23:59:59Z",
        created_by: 1,
        first_name: "John",
        last_name: "Smith",
        position: "Science Teacher",
        creator: "admin@school.com",
      };

      const result = mapPendingInvitationResponse(backendData);

      expect(result).toEqual({
        id: 123,
        email: "teacher@school.com",
        roleId: 5,
        roleName: "Teacher",
        createdBy: 1,
        creatorEmail: "admin@school.com",
        expiresAt: "2024-12-31T23:59:59Z",
        token: "abc123xyz",
        firstName: "John",
        lastName: "Smith",
        position: "Science Teacher",
      });
    });

    it("should handle missing role_name by using empty string", () => {
      const backendData: BackendInvitation = {
        id: 456,
        email: "teacher@school.com",
        role_id: 5,
        role_name: undefined,
        expires_at: "2024-12-31T23:59:59Z",
        created_by: 1,
      };

      const result = mapPendingInvitationResponse(backendData);

      expect(result.roleName).toBe("");
    });

    it("should handle all optional fields as null", () => {
      const backendData: BackendInvitation = {
        id: 789,
        email: "teacher@school.com",
        role_id: 5,
        role_name: "Teacher",
        token: undefined,
        expires_at: "2024-12-31T23:59:59Z",
        created_by: 1,
        first_name: null,
        last_name: null,
        position: null,
        creator: undefined,
      };

      const result = mapPendingInvitationResponse(backendData);

      expect(result).toEqual({
        id: 789,
        email: "teacher@school.com",
        roleId: 5,
        roleName: "Teacher",
        createdBy: 1,
        creatorEmail: undefined,
        expiresAt: "2024-12-31T23:59:59Z",
        token: undefined,
        firstName: null,
        lastName: null,
        position: null,
      });
    });

    it("should handle all optional fields as undefined", () => {
      const backendData: BackendInvitation = {
        id: 101,
        email: "admin@school.com",
        role_id: 1,
        role_name: "Admin",
        token: undefined,
        expires_at: "2024-06-30T12:00:00Z",
        created_by: 1,
        first_name: undefined,
        last_name: undefined,
        position: undefined,
        creator: undefined,
      };

      const result = mapPendingInvitationResponse(backendData);

      expect(result).toEqual({
        id: 101,
        email: "admin@school.com",
        roleId: 1,
        roleName: "Admin",
        createdBy: 1,
        creatorEmail: undefined,
        expiresAt: "2024-06-30T12:00:00Z",
        token: undefined,
        firstName: undefined,
        lastName: undefined,
        position: undefined,
      });
    });

    it("should handle mix of present and missing optional fields", () => {
      const backendData: BackendInvitation = {
        id: 202,
        email: "staff@school.com",
        role_id: 3,
        role_name: "Staff",
        token: "token123",
        expires_at: "2024-08-15T10:00:00Z",
        created_by: 2,
        first_name: "Alice",
        last_name: null,
        position: undefined,
        creator: "supervisor@school.com",
      };

      const result = mapPendingInvitationResponse(backendData);

      expect(result).toEqual({
        id: 202,
        email: "staff@school.com",
        roleId: 3,
        roleName: "Staff",
        createdBy: 2,
        creatorEmail: "supervisor@school.com",
        expiresAt: "2024-08-15T10:00:00Z",
        token: "token123",
        firstName: "Alice",
        lastName: null,
        position: undefined,
      });
    });

    it("should preserve special characters in fields", () => {
      const backendData: BackendInvitation = {
        id: 303,
        email: "teacher+special@example.com",
        role_id: 5,
        role_name: "Teacher (Substitute)",
        token: "abc-123-xyz",
        expires_at: "2024-12-31T23:59:59Z",
        created_by: 1,
        first_name: "José",
        last_name: "O'Connor",
        position: "PE & Sports",
        creator: "admin+main@example.com",
      };

      const result = mapPendingInvitationResponse(backendData);

      expect(result.email).toBe("teacher+special@example.com");
      expect(result.roleName).toBe("Teacher (Substitute)");
      expect(result.firstName).toBe("José");
      expect(result.lastName).toBe("O'Connor");
      expect(result.position).toBe("PE & Sports");
      expect(result.creatorEmail).toBe("admin+main@example.com");
    });

    it("should handle numeric IDs correctly", () => {
      const backendData: BackendInvitation = {
        id: 999999,
        email: "test@example.com",
        role_id: 888888,
        role_name: "Role",
        expires_at: "2024-12-31T23:59:59Z",
        created_by: 777777,
      };

      const result = mapPendingInvitationResponse(backendData);

      expect(result.id).toBe(999999);
      expect(result.roleId).toBe(888888);
      expect(result.createdBy).toBe(777777);
    });
  });
});

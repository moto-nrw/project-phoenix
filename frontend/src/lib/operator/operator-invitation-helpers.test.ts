import { describe, it, expect } from "vitest";
import {
  mapPendingInvitation,
  mapOperatorInfo,
  mapInvitationValidation,
  type BackendInvitationResponse,
  type BackendOperatorListItem,
  type BackendInvitationValidation,
} from "./operator-invitation-helpers";

const sampleInvitation: BackendInvitationResponse = {
  id: 42,
  email: "new-op@example.com",
  display_name: "Max Mustermann",
  created_by: 1,
  creator_name: "Admin",
  expires_at: "2026-04-06T12:00:00Z",
  email_sent_at: "2026-04-04T12:00:00Z",
  email_error: null,
  email_retry_count: 0,
  created_at: "2026-04-04T12:00:00Z",
};

const sampleOperator: BackendOperatorListItem = {
  id: 7,
  email: "operator@example.com",
  display_name: "Operator One",
  active: true,
  last_login: "2026-04-03T10:00:00Z",
  created_at: "2026-01-01T00:00:00Z",
};

const sampleValidation: BackendInvitationValidation = {
  email: "invited@example.com",
  display_name: "Invited User",
  expires_at: "2026-04-06T12:00:00Z",
};

describe("mapPendingInvitation", () => {
  it("maps all fields with snake_case to camelCase", () => {
    const result = mapPendingInvitation(sampleInvitation);

    expect(result).toEqual({
      id: "42",
      email: "new-op@example.com",
      displayName: "Max Mustermann",
      createdBy: "1",
      creatorName: "Admin",
      expiresAt: "2026-04-06T12:00:00Z",
      emailSentAt: "2026-04-04T12:00:00Z",
      emailError: undefined,
      emailRetryCount: 0,
      createdAt: "2026-04-04T12:00:00Z",
    });
  });

  it("converts null display_name to undefined", () => {
    const result = mapPendingInvitation({
      ...sampleInvitation,
      display_name: null,
    });
    expect(result.displayName).toBeUndefined();
  });

  it("converts null email_sent_at to undefined", () => {
    const result = mapPendingInvitation({
      ...sampleInvitation,
      email_sent_at: null,
    });
    expect(result.emailSentAt).toBeUndefined();
  });

  it("converts null email_error to undefined", () => {
    const result = mapPendingInvitation({
      ...sampleInvitation,
      email_error: null,
    });
    expect(result.emailError).toBeUndefined();
  });

  it("converts numeric IDs to strings", () => {
    const result = mapPendingInvitation(sampleInvitation);
    expect(result.id).toBe("42");
    expect(result.createdBy).toBe("1");
  });
});

describe("mapOperatorInfo", () => {
  it("maps all fields correctly", () => {
    const result = mapOperatorInfo(sampleOperator);

    expect(result).toEqual({
      id: "7",
      email: "operator@example.com",
      displayName: "Operator One",
      active: true,
      lastLogin: "2026-04-03T10:00:00Z",
      createdAt: "2026-01-01T00:00:00Z",
    });
  });

  it("converts null last_login to undefined", () => {
    const result = mapOperatorInfo({ ...sampleOperator, last_login: null });
    expect(result.lastLogin).toBeUndefined();
  });

  it("preserves active=false", () => {
    const result = mapOperatorInfo({ ...sampleOperator, active: false });
    expect(result.active).toBe(false);
  });
});

describe("mapInvitationValidation", () => {
  it("maps all fields correctly", () => {
    const result = mapInvitationValidation(sampleValidation);

    expect(result).toEqual({
      email: "invited@example.com",
      displayName: "Invited User",
      expiresAt: "2026-04-06T12:00:00Z",
    });
  });

  it("converts null display_name to undefined", () => {
    const result = mapInvitationValidation({
      ...sampleValidation,
      display_name: null,
    });
    expect(result.displayName).toBeUndefined();
  });

  it("handles missing display_name", () => {
    const result = mapInvitationValidation({
      email: "test@test.com",
      expires_at: "2026-04-06T12:00:00Z",
    });
    expect(result.displayName).toBeUndefined();
  });
});

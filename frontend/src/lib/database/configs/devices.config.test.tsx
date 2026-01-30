/**
 * Tests for Devices Configuration
 * Tests device config structure and helper functions
 */
import { describe, it, expect } from "vitest";
import { devicesConfig } from "./devices.config";
import type { Device } from "@/lib/iot-helpers";

describe("devicesConfig", () => {
  it("exports a valid entity config", () => {
    expect(devicesConfig).toBeDefined();
    expect(devicesConfig.name).toEqual({
      singular: "Gerät",
      plural: "Geräte",
    });
  });

  it("has correct API configuration", () => {
    expect(devicesConfig.api.basePath).toBe("/api/iot");
  });

  it("has form sections configured", () => {
    expect(devicesConfig.form.sections).toHaveLength(1);
    expect(devicesConfig.form.sections[0]?.title).toBe("Geräteinformationen");
  });

  it("has required form fields", () => {
    const fields = devicesConfig.form.sections[0]?.fields ?? [];
    const fieldNames = fields.map((f) => f.name);

    expect(fieldNames).toContain("device_id");
    expect(fieldNames).toContain("name");
    expect(fieldNames).toContain("status");
  });

  it("has default values", () => {
    expect(devicesConfig.form.defaultValues?.status).toBe("active");
    expect(devicesConfig.form.defaultValues?.device_type).toBe("rfid_reader");
  });

  it("transforms data before submit with auto-generated name", () => {
    const data: Partial<Device> = {
      device_id: "RFID-001",
      device_type: "rfid_reader",
      status: "active",
    };

    const transformed = devicesConfig.form.transformBeforeSubmit?.(data);
    expect(transformed?.name).toBeDefined();
  });

  it("preserves provided name in transform", () => {
    const data: Partial<Device> = {
      device_id: "RFID-001",
      name: "Custom Name",
      device_type: "rfid_reader",
      status: "active",
    };

    const transformed = devicesConfig.form.transformBeforeSubmit?.(data);
    expect(transformed?.name).toBe("Custom Name");
  });

  it("has detail header configuration", () => {
    const mockDevice: Device = {
      id: "1",
      device_id: "RFID-001",
      name: "Main Entrance",
      device_type: "rfid_reader",
      status: "active",
      is_online: false,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };

    expect(devicesConfig.detail.header?.title(mockDevice)).toBe(
      "Main Entrance",
    );
  });

  it("falls back to device_id when no name", () => {
    const mockDevice: Device = {
      id: "1",
      device_id: "RFID-001",
      device_type: "rfid_reader",
      status: "active",
      is_online: false,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };

    expect(devicesConfig.detail.header?.title(mockDevice)).toBe("RFID-001");
  });

  it("has list configuration", () => {
    expect(devicesConfig.list.title).toBe("Gerät auswählen");
    expect(devicesConfig.list.searchStrategy).toBe("frontend");
  });

  it("has filters configured", () => {
    const filters = devicesConfig.list.filters ?? [];
    const filterIds = filters.map((f) => f.id);

    expect(filterIds).toContain("device_type");
    expect(filterIds).toContain("status");
    expect(filterIds).toContain("is_online");
  });

  it("displays device in list", () => {
    const mockDevice: Device = {
      id: "1",
      device_id: "RFID-001",
      name: "Main Entrance",
      device_type: "rfid_reader",
      status: "active",
      is_online: false,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };

    const title = devicesConfig.list.item.title(mockDevice);
    expect(title).toBe("Main Entrance");
  });

  it("has custom labels", () => {
    expect(devicesConfig.labels?.createButton).toBe("Neues Gerät registrieren");
    expect(devicesConfig.labels?.deleteConfirmation).toContain("löschen");
  });
});

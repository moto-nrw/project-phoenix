import { describe, it, expect, vi } from "vitest";

import { buildGroupOverflowItems } from "./group-overflow-items";

describe("buildGroupOverflowItems", () => {
  it("returns an empty array while supervising via substitution", () => {
    const items = buildGroupOverflowItems({
      viaSubstitution: true,
      canTransfer: true,
      activeTransfersCount: 0,
      onOpenTransfer: vi.fn(),
    });
    expect(items).toEqual([]);
  });

  it("returns the transfer item without a badge when no active transfers", () => {
    const onOpenTransfer = vi.fn();
    const items = buildGroupOverflowItems({
      viaSubstitution: false,
      canTransfer: true,
      activeTransfersCount: 0,
      onOpenTransfer,
    });

    expect(items).toHaveLength(1);
    expect(items[0]?.label).toBe("Gruppe übergeben");
    expect(items[0]?.badge).toBeUndefined();
    expect(items[0]?.onClick).toBe(onOpenTransfer);
  });

  it("adds a count badge when active transfers exist", () => {
    const items = buildGroupOverflowItems({
      viaSubstitution: false,
      canTransfer: true,
      activeTransfersCount: 3,
      onOpenTransfer: vi.fn(),
    });

    expect(items[0]?.badge).toBe(3);
  });

  it("substitution takes precedence over an active-transfer count", () => {
    // A substitute can never have outgoing transfers, but defensively
    // assert the function still suppresses the item.
    const items = buildGroupOverflowItems({
      viaSubstitution: true,
      canTransfer: true,
      activeTransfersCount: 5,
      onOpenTransfer: vi.fn(),
    });
    expect(items).toEqual([]);
  });

  it("does not offer a handover for an additional visible group", () => {
    const items = buildGroupOverflowItems({
      viaSubstitution: false,
      canTransfer: false,
      activeTransfersCount: 0,
      onOpenTransfer: vi.fn(),
    });

    expect(items).toEqual([]);
  });
});

import { describe, expect, it } from "vitest";
import { importChildQuotaNotice } from "./child-quota-import";

const quota = (requested: number, free: number, fits: boolean) => ({
  booked_places: 100,
  occupied_places: 100 - free,
  requested_places: requested,
  free_places: free,
  fits,
});

describe("importChildQuotaNotice", () => {
  it("names the new children and the free Kinderkontingent when they fit", () => {
    expect(importChildQuotaNotice(quota(2, 12, true))).toEqual({
      type: "info",
      message:
        "Der Import würde 2 Kinder hinzufügen. Im Kinderkontingent sind noch 12 frei.",
    });
  });

  it("blocks an import the Kinderkontingent cannot hold and names the way out", () => {
    expect(importChildQuotaNotice(quota(80, 12, false))).toEqual({
      type: "error",
      message:
        "Der Import würde 80 Kinder hinzufügen. Im Kinderkontingent sind nur noch 12 frei. Der Import startet darum nicht. Nehmen Sie Kinder aus der Datei heraus oder melden Sie sich beim moto-Team.",
    });
  });

  it("uses the singular for one child and a full Kinderkontingent", () => {
    expect(importChildQuotaNotice(quota(1, 0, false))?.message).toBe(
      "Der Import würde 1 Kind hinzufügen. Das Kinderkontingent ist voll. Der Import startet darum nicht. Nehmen Sie Kinder aus der Datei heraus oder melden Sie sich beim moto-Team.",
    );
    expect(importChildQuotaNotice(quota(1, 1, true))?.message).toBe(
      "Der Import würde 1 Kind hinzufügen. Im Kinderkontingent ist noch 1 frei.",
    );
  });

  it("stays silent when the import adds no child or the school has no Kinderkontingent", () => {
    expect(importChildQuotaNotice(quota(0, 0, true))).toBeNull();
    expect(importChildQuotaNotice(undefined)).toBeNull();
  });
});

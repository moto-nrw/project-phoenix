import { renderHook, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { useTenantSafe } from "~/lib/tenant-context";
import type { TenantInfo } from "~/lib/tenant-api";
import {
  CareFacilityTermScope,
  normalizeCareFacilityTerm,
  useCareFacilityTerm,
  useCareTermTranslations,
  useProvideCareFacilityTerm,
} from "./care-facility-term";

function tenantWithTerm(careFacilityTerm: string | undefined) {
  vi.mocked(useTenantSafe).mockReturnValue({
    tenantSlug: "barnstorf",
    tenant: { careFacilityTerm } as TenantInfo,
  } as ReturnType<typeof useTenantSafe>);
}

describe("care facility term (#6042)", () => {
  it("keeps OGS for schools without a term and for missing metadata", () => {
    expect(normalizeCareFacilityTerm(undefined)).toBe("OGS");
    expect(normalizeCareFacilityTerm("  ")).toBe("OGS");
    expect(normalizeCareFacilityTerm(" GTS ")).toBe("GTS");
    const { result } = renderHook(() => useCareFacilityTerm());
    expect(result.current).toBe("OGS");
  });

  it("fills the tenant's term into the fixed enrollment texts", () => {
    tenantWithTerm("GTS");
    const { result } = renderHook(() =>
      useCareTermTranslations("enrollmentPublic"),
    );
    expect(result.current("reviewTitle")).toBe("Prüfung durch die GTS");
    expect(result.current("reviewText")).toMatch(/^Die GTS prüft/);
  });

  it("keeps the caller's own values next to the term", () => {
    tenantWithTerm("Ganztag Nord");
    const { result } = renderHook(() =>
      useCareTermTranslations("enrollmentForm"),
    );
    expect(result.current("care.noOfferingsForGrade", { grade: 3 })).toBe(
      "Für die 3. Klasse gibt es hier keine passenden Betreuungsangebote. Bei Fragen wenden Sie sich bitte an die Ganztag Nord.",
    );
  });

  it("prefers the school a loaded response named over the tenant in context", () => {
    tenantWithTerm("OGS");
    function Status({ term }: { readonly term?: string }) {
      useProvideCareFacilityTerm(term);
      const t = useCareTermTranslations("enrollmentStatus");
      return <p>{t("stepOneStatus")}</p>;
    }
    const { rerender } = render(
      <CareFacilityTermScope>
        <Status />
      </CareFacilityTermScope>,
    );
    expect(
      screen.getByText("Die Anmeldung liegt der OGS vor."),
    ).toBeInTheDocument();
    rerender(
      <CareFacilityTermScope>
        <Status term="GTS" />
      </CareFacilityTermScope>,
    );
    expect(
      screen.getByText("Die Anmeldung liegt der GTS vor."),
    ).toBeInTheDocument();
  });

  it("keeps the translator when the term arrives, so loads do not run again", () => {
    // Status and edit pages load their data in effects that depend on the
    // translator; a new identity for the term would load everything again.
    tenantWithTerm("OGS");
    const seen: unknown[] = [];
    function Probe({ term }: { readonly term?: string }) {
      useProvideCareFacilityTerm(term);
      const t = useCareTermTranslations("enrollmentStatus");
      seen.push(t);
      return <p>{t("stepOneStatus")}</p>;
    }
    const { rerender } = render(
      <CareFacilityTermScope>
        <Probe />
      </CareFacilityTermScope>,
    );
    rerender(
      <CareFacilityTermScope>
        <Probe term="GTS" />
      </CareFacilityTermScope>,
    );
    expect(
      screen.getByText("Die Anmeldung liegt der GTS vor."),
    ).toBeInTheDocument();
    expect(new Set(seen).size).toBe(1);
  });
});

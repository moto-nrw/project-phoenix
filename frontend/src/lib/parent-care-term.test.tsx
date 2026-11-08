import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { useCareTermTranslations } from "~/lib/care-facility-term";
import type { Child, ChildFeatures } from "~/lib/parent-api";
import {
  accountCareTerm,
  ParentCareTermScope,
  ParentChildCareTermScope,
  useParentCareTerms,
} from "./parent-care-term";

const resources = vi.hoisted(() => ({
  children: [] as Child[],
  features: {} as Record<string, Partial<ChildFeatures>>,
}));

vi.mock("~/lib/hooks/use-parent-resources", () => ({
  useParentChildren: () => ({ data: resources.children }),
  useParentFeatures: (children: readonly Child[] | undefined) => ({
    data: children ? resources.features : undefined,
  }),
}));

function child(studentId: string, schoolName: string): Child {
  return {
    student_id: studentId,
    tenant_id: `t-${schoolName}`,
    first_name: `Kind ${studentId}`,
    last_name: "Muster",
    status: "active",
    school_name: schoolName,
    school_slug: schoolName.toLowerCase(),
  };
}

function link(...rows: [Child, string | undefined][]) {
  resources.children = rows.map(([row]) => row);
  resources.features = Object.fromEntries(
    rows.map(([row, term]) => [row.student_id, { care_facility_term: term }]),
  );
}

function Today() {
  const t = useCareTermTranslations("parentToday");
  return <p>{t("atOgs")}</p>;
}

describe("care term in the parents portal (#6082)", () => {
  it("names one term when all schools share it, else every term once", () => {
    expect(accountCareTerm([])).toBeUndefined();
    expect(accountCareTerm(["GTS", " GTS "])).toBe("GTS");
    expect(accountCareTerm(["OGS", "GTS", "OGS"])).toBe("OGS/GTS");
    expect(accountCareTerm([undefined, "GTS"])).toBe("GTS");
  });

  it("names each child's care with its own school's term", () => {
    link([child("1", "Barnstorf"), "GTS"], [child("2", "Sonnenschule"), "OGS"]);
    render(
      <ParentCareTermScope enabled>
        <ParentChildCareTermScope studentId="1">
          <Today />
        </ParentChildCareTermScope>
        <ParentChildCareTermScope studentId="2">
          <Today />
        </ParentChildCareTermScope>
      </ParentCareTermScope>,
    );
    expect(screen.getByText("In der GTS")).toBeInTheDocument();
    expect(screen.getByText("In der OGS")).toBeInTheDocument();
  });

  it("uses the account's term for texts without a child", () => {
    link([child("1", "Barnstorf"), "GTS"], [child("2", "Barnstorf"), "GTS"]);
    const { unmount } = render(
      <ParentCareTermScope enabled>
        <Today />
      </ParentCareTermScope>,
    );
    expect(screen.getByText("In der GTS")).toBeInTheDocument();
    unmount();

    link([child("1", "Barnstorf"), "GTS"], [child("2", "Sonnenschule"), "OGS"]);
    render(
      <ParentCareTermScope enabled>
        <Today />
      </ParentCareTermScope>,
    );
    expect(screen.getByText("In der GTS/OGS")).toBeInTheDocument();
  });

  it("keeps a child's subtree mounted when its term arrives late", () => {
    link([child("1", "Barnstorf"), undefined]);
    function Draft() {
      const [draft, setDraft] = useState("");
      return (
        <>
          <Today />
          <input
            aria-label="Entwurf"
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
          />
        </>
      );
    }
    const tree = () => (
      <ParentCareTermScope enabled>
        <ParentChildCareTermScope studentId="1">
          <Draft />
        </ParentChildCareTermScope>
      </ParentCareTermScope>
    );
    const { rerender } = render(tree());
    fireEvent.change(screen.getByLabelText("Entwurf"), {
      target: { value: "Bis gleich" },
    });
    expect(screen.getByText("In der OGS")).toBeInTheDocument();

    link([child("1", "Barnstorf"), "GTS"]);
    rerender(tree());
    expect(screen.getByText("In der GTS")).toBeInTheDocument();
    expect(screen.getByLabelText("Entwurf")).toHaveValue("Bis gleich");
  });

  it("says OGS outside the parents shell", () => {
    let terms: ReturnType<typeof useParentCareTerms> | undefined;
    function Probe() {
      terms = useParentCareTerms();
      return null;
    }
    render(
      <ParentChildCareTermScope studentId="1">
        <Today />
        <Probe />
      </ParentChildCareTermScope>,
    );
    expect(screen.getByText("In der OGS")).toBeInTheDocument();
    expect(terms?.termOf("1")).toBe("OGS");
  });

  it("keeps OGS while no school named its term", () => {
    link([child("1", "Sonnenschule"), undefined]);
    render(
      <ParentCareTermScope enabled>
        <ParentChildCareTermScope studentId="1">
          <Today />
        </ParentChildCareTermScope>
      </ParentCareTermScope>,
    );
    expect(screen.getByText("In der OGS")).toBeInTheDocument();
  });

  it("finds a school's term for rows that name a school, not a child", () => {
    link([child("1", "Barnstorf"), "GTS"], [child("2", "Sonnenschule"), "OGS"]);
    let terms: ReturnType<typeof useParentCareTerms> | undefined;
    function Probe() {
      terms = useParentCareTerms();
      return null;
    }
    render(
      <ParentCareTermScope enabled>
        <Probe />
      </ParentCareTermScope>,
    );
    expect(terms?.termOfSchool("Barnstorf")).toBe("GTS");
    expect(terms?.termOfSchool("Sonnenschule")).toBe("OGS");
    expect(terms?.termOfSchool("Unbekannt")).toBe("GTS/OGS");
    expect(terms?.termOf("2")).toBe("OGS");
  });
});

import { render, screen } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

import { PersonalInfoSection } from "./student-form-fields";
import { PARENT_VISIBLE_HINTS } from "~/lib/parent-visible-fields";

// The point of the marker (#2163) is that staff can tell, before saving, which
// Stammdaten the parent portal mirrors. The child's address is the field
// schools actually asked about, and it is NOT mirrored — so its absence from
// the marked set is the assertion that matters most here.
describe("Für-Eltern-sichtbar Kennzeichnung", () => {
  function renderPersonalInfo() {
    render(
      <PersonalInfoSection formData={{}} onChange={vi.fn()} errors={{}} />,
    );
  }

  /**
   * True when the field with this label carries a marker. The marker sits NEXT
   * TO the <label>, never inside it: inside, its text would join the input's
   * accessible name and rename the field for screen readers.
   */
  function isMarked(labelText: string): boolean {
    const label = screen.getByText(new RegExp(`^${labelText}`));
    expect(label.textContent).not.toContain("Für Eltern sichtbar");
    return Boolean(
      label.parentElement?.textContent?.includes("Für Eltern sichtbar"),
    );
  }

  it("marks the fields the parent portal mirrors", () => {
    renderPersonalInfo();

    for (const field of ["Vorname", "Nachname", "Klasse", "Geburtstag"]) {
      expect(isMarked(field), `${field} sollte markiert sein`).toBe(true);
    }
  });

  it("leaves the child's address and group unmarked", () => {
    renderPersonalInfo();

    for (const field of ["Straße und Hausnummer", "PLZ", "Ort", "Gruppe"]) {
      expect(isMarked(field), `${field} darf nicht markiert sein`).toBe(false);
    }
  });

  it("states the inverse in the legend, which the markers only imply", () => {
    renderPersonalInfo();

    expect(
      screen.getByText(/Nicht gekennzeichnete Angaben bleiben intern/),
    ).toBeInTheDocument();
  });

  // Whether a parent may edit or request a change depends on their per-child
  // guardian permission, on tenant settings, and per field on further state (an
  // "accompanied" departure day blocks the request outright). A static marker
  // cannot know any of that, so no hint may promise it.
  it("promises visibility only, never that parents may change a value", () => {
    for (const [key, hint] of Object.entries(PARENT_VISIBLE_HINTS)) {
      expect(hint, `${key} darf keine Änderung zusagen`).not.toMatch(
        /ändern|beantragen|anpassen|bearbeiten/i,
      );
      expect(hint, `${key} muss die Sichtbarkeit nennen`).toMatch(/sehen/i);
    }
  });
});

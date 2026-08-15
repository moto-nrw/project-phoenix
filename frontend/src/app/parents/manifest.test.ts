import { describe, expect, it } from "vitest";
import parentsManifest from "./manifest";

describe("parentsManifest", () => {
  const manifest = parentsManifest();

  it("startet eigenstaendig im Hochformat", () => {
    expect(manifest.display).toBe("standalone");
    expect(manifest.orientation).toBe("portrait-primary");
    expect(manifest.lang).toBe("de");
  });

  it("traegt einen eltern-spezifischen Namen", () => {
    expect(manifest.name).toBe("moto Eltern");
    expect(manifest.short_name).toBe("moto");
    expect(manifest.description).toMatch(/OGS/);
  });

  it("nutzt die oeffentliche Sicht des Eltern-Hosts als Startadresse", () => {
    // Der Proxy bildet den Eltern-Host intern auf /parents/* ab. Oeffentlich
    // beginnen die Adressen ohne dieses Praefix, deshalb ist die Wurzel
    // Startadresse und Geltungsbereich. Ein Geltungsbereich /parents wuerde
    // jede echte Adresse ausschliessen und die Installation verhindern.
    expect(manifest.start_url).toBe("/");
    expect(manifest.scope).toBe("/");
  });

  it("liefert ein 192er- und ein 512er-Symbol", () => {
    const sizes = (manifest.icons ?? []).map((icon) => icon.sizes);
    expect(sizes).toContain("192x192");
    expect(sizes).toContain("512x512");
    for (const icon of manifest.icons ?? []) {
      expect(icon.src).toMatch(/^\/favicons\/eltern\//);
    }
  });
});

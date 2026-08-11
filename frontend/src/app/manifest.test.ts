import { beforeEach, describe, expect, it, vi } from "vitest";
import manifest from "./manifest";

const headerState = vi.hoisted(() => ({
  host: "moto-app.de",
}));

vi.mock("next/headers", () => ({
  headers: () =>
    Promise.resolve({
      get: (name: string) =>
        name.toLowerCase() === "host" ? headerState.host : null,
    }),
}));

describe("manifest", () => {
  beforeEach(() => {
    headerState.host = "moto-app.de";
    vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", "operator.moto-app.de");
    vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", "eltern.moto-app.de");
    vi.stubEnv("NEXT_PUBLIC_SCHOOL_HOSTNAME", "schule.moto-app.de");
    vi.stubEnv("TENANT_DOMAIN", "moto-app.de");
  });

  it("returns normal install icons for the main app host", async () => {
    await expect(manifest()).resolves.toMatchObject({
      icons: [
        { src: "/favicons/normal/icon-192.png", purpose: "any" },
        { src: "/favicons/normal/icon-512.png", purpose: "any" },
        { src: "/favicons/normal/icon-maskable-192.png", purpose: "maskable" },
        { src: "/favicons/normal/icon-maskable-512.png", purpose: "maskable" },
      ],
    });
  });

  it("returns staging operator install icons for the staging operator host", async () => {
    headerState.host = "operator.staging.moto-app.de";
    vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", "operator.staging.moto-app.de");
    vi.stubEnv("NEXT_PUBLIC_PARENTS_HOSTNAME", "eltern.staging.moto-app.de");
    vi.stubEnv("NEXT_PUBLIC_SCHOOL_HOSTNAME", "schule.staging.moto-app.de");

    await expect(manifest()).resolves.toMatchObject({
      icons: [
        { src: "/favicons/operator-staging/icon-192.png", purpose: "any" },
        { src: "/favicons/operator-staging/icon-512.png", purpose: "any" },
        {
          src: "/favicons/operator-staging/icon-maskable-192.png",
          purpose: "maskable",
        },
        {
          src: "/favicons/operator-staging/icon-maskable-512.png",
          purpose: "maskable",
        },
      ],
    });
  });

  it("returns the moto schule identity for the school host", async () => {
    headerState.host = "schule.moto-app.de";

    await expect(manifest()).resolves.toMatchObject({
      name: "moto schule",
      short_name: "moto schule",
      icons: [
        { src: "/favicons/schule/icon-192.png", purpose: "any" },
        { src: "/favicons/schule/icon-512.png", purpose: "any" },
        { src: "/favicons/schule/icon-maskable-192.png", purpose: "maskable" },
        { src: "/favicons/schule/icon-maskable-512.png", purpose: "maskable" },
      ],
    });
  });
});

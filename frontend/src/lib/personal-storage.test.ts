import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  stashAnnouncementStudents,
  takeAnnouncementStudents,
} from "./announcement-prefill";
import {
  bindPersonalStorage,
  clearPersonalStorage,
  isPersonalStorageKey,
  PERSONAL_STORAGE_PREFIXES,
  personalStorageOwner,
} from "./personal-storage";
import { sidebarMemoryKey } from "./sidebar-memory";

const OWNER = "1:7";

// What an account leaves behind after a working day (#5000), one entry per
// inventoried kind, including the unscoped keys written before #5000.
const PERSONAL_LOCAL = {
  [sidebarMemoryKey("sidebar-last-group", OWNER)]: "12",
  [sidebarMemoryKey("sidebar-last-group-name", OWNER)]: "Eulen",
  [sidebarMemoryKey("sidebar-last-room", OWNER)]: "4",
  [sidebarMemoryKey("sidebar-last-room-name", OWNER)]: "Raum Sonne",
  [sidebarMemoryKey("supervision-last-session", OWNER)]: "88",
  [sidebarMemoryKey("sidebar-last-database", OWNER)]: "/database/students",
  "sidebar-last-group": "12",
  "sidebar-last-group-name": "Eulen",
  "sidebar-last-room": "4",
  "sidebar-last-room-name": "Raum Sonne",
  "supervision-last-session": "88",
  "sidebar-last-database": "/database/students",
  "messages_unread_count:schule:7": '{"count":3,"timestamp":1}',
  "team_chat_unread_count:schule:7": '{"count":1,"timestamp":1}',
  "enrollments_unread_count:schule:7": '{"count":2,"timestamp":1}',
  "enrollment_requests_pending_count:schule:7": '{"count":2,"timestamp":1}',
  "change_requests_pending_count:schule:7": '{"count":2,"timestamp":1}',
  "care_withdrawals_pending_count:schule:7": '{"count":2,"timestamp":1}',
  "staff_absences_pending_count:schule:7": '{"count":2,"timestamp":1}',
  "staff_notices_pending:schule:7": '{"count":2,"timestamp":1}',
  "school_team_chat_unread:1:7": '{"count":2,"timestamp":1}',
  "school_staff_notices_pending:1:7": '{"count":2,"timestamp":1}',
  "student-search:last-filters:tenant-1:7":
    '{"room_name":"Raum Sonne","group_id":"12"}',
  "moto:present-children-offering:schule:7": "31",
  "moto:personal-storage-owner": OWNER,
};

const PERSONAL_SESSION = {
  "moto:personal-storage-owner": OWNER,
  "moto:announcement-prefill:legacy-token":
    '{"tenantSlug":"schule","students":[{"id":"5","name":"Mia Arslan"}]}',
  "timetable-reopenable-instance":
    '{"instanceId":"9","title":"Fußball","roomId":"4","expiresAt":1}',
  "school-setup-opened:7": "1",
  "staff-onboarding-opened:schule:7": "1",
  "parent-first-steps-opened:7": "1",
};

// Settings without data of a person; they stay on the device.
const KEPT_LOCAL = {
  "sidebar-collapsed": "true",
  "sidebar-open-groups": '["planning"]',
  "sidebar-accordion-expanded": "groups",
  "collection-view:students:tenant-1:7": '{"view":"table"}',
  "moto.tenant.notification-setup.v1.schule.7": '{"done":true}',
  "parent-first-steps:v1:7": '{"completed":[],"dismissed":true}',
  "moto-pwa-install-hint-dismissed": "1",
};

const KEPT_SESSION = {
  deliberateLogout: "1",
  "tagesplan-scroll": '{"day":"2026-09-09","y":120}',
};

function seed(storage: Storage, entries: Record<string, string>) {
  for (const [key, value] of Object.entries(entries)) {
    storage.setItem(key, value);
  }
}

function keys(storage: Storage): string[] {
  const result: string[] = [];
  for (let index = 0; index < storage.length; index += 1) {
    const key = storage.key(index);
    if (key !== null) result.push(key);
  }
  return result.sort();
}

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
});

afterEach(() => {
  localStorage.clear();
  sessionStorage.clear();
});

describe("clearPersonalStorage", () => {
  it("removes every personal entry of the account and keeps settings", () => {
    seed(localStorage, { ...PERSONAL_LOCAL, ...KEPT_LOCAL });
    seed(sessionStorage, { ...PERSONAL_SESSION, ...KEPT_SESSION });
    const token = stashAnnouncementStudents("schule", [
      { id: "5", name: "Mia Arslan" },
    ]);
    expect(token).not.toBeNull();

    clearPersonalStorage();

    expect(keys(localStorage)).toEqual(Object.keys(KEPT_LOCAL).sort());
    expect(keys(sessionStorage)).toEqual(Object.keys(KEPT_SESSION).sort());
    expect(takeAnnouncementStudents("schule", token!)).toEqual([]);
  });

  it("classifies every seeded key the way the inventory says", () => {
    for (const key of [
      ...Object.keys(PERSONAL_LOCAL),
      ...Object.keys(PERSONAL_SESSION),
    ]) {
      expect(isPersonalStorageKey(key), key).toBe(true);
    }
    for (const key of [
      ...Object.keys(KEPT_LOCAL),
      ...Object.keys(KEPT_SESSION),
    ]) {
      expect(isPersonalStorageKey(key), key).toBe(false);
    }
  });
});

describe("bindPersonalStorage", () => {
  it("keeps the entries of the same account at the same school", () => {
    seed(localStorage, PERSONAL_LOCAL);
    seed(sessionStorage, PERSONAL_SESSION);

    bindPersonalStorage(OWNER);

    expect(keys(localStorage)).toEqual(Object.keys(PERSONAL_LOCAL).sort());
    expect(keys(sessionStorage)).toEqual(Object.keys(PERSONAL_SESSION).sort());
    expect(sessionStorage.getItem("timetable-reopenable-instance")).toBe(
      PERSONAL_SESSION["timetable-reopenable-instance"],
    );
  });

  it("removes the entries when another account signs in", () => {
    seed(localStorage, { ...PERSONAL_LOCAL, ...KEPT_LOCAL });
    seed(sessionStorage, PERSONAL_SESSION);

    bindPersonalStorage("1:8");

    expect(keys(localStorage)).toEqual(
      [...Object.keys(KEPT_LOCAL), "moto:personal-storage-owner"].sort(),
    );
    expect(localStorage.getItem("moto:personal-storage-owner")).toBe("1:8");
    expect(keys(sessionStorage)).toEqual(["moto:personal-storage-owner"]);
    expect(sessionStorage.getItem("moto:personal-storage-owner")).toBe("1:8");
  });

  it("removes the entries when the same account switches school", () => {
    seed(localStorage, PERSONAL_LOCAL);

    bindPersonalStorage("2:7");

    expect(keys(localStorage)).toEqual(["moto:personal-storage-owner"]);
  });

  it("removes entries nobody claimed, such as those from before #5000", () => {
    localStorage.setItem("sidebar-last-room-name", "Raum Sonne");
    localStorage.setItem("sidebar-last-room", "4");

    bindPersonalStorage(OWNER);

    expect(keys(localStorage)).toEqual(["moto:personal-storage-owner"]);
  });
});

describe("personalStorageOwner", () => {
  it("names school and account of the session", () => {
    expect(personalStorageOwner({ id: "7", tenantId: 1 })).toBe("1:7");
    expect(personalStorageOwner({ id: "7", orgId: 3 })).toBe("3:7");
    expect(personalStorageOwner({ id: "7" })).toBe("-:7");
  });

  it("knows no owner without an account", () => {
    expect(personalStorageOwner(null)).toBeNull();
    expect(personalStorageOwner({ id: "", tenantId: 1 })).toBeNull();
  });
});

describe("inventory", () => {
  // Every unread or pending badge caches its count per account. A new badge
  // whose cache key the inventory does not know would survive the logout.
  it("covers the cache key of every count badge", () => {
    const hooksDir = join(process.cwd(), "src/lib/hooks");
    const cacheKeys = readdirSync(hooksDir)
      .filter((file) => file.endsWith(".ts") && !file.endsWith(".test.ts"))
      .flatMap((file) =>
        Array.from(
          readFileSync(join(hooksDir, file), "utf8").matchAll(
            /cacheKey:\s*`([^`$]+)\$\{/g,
          ),
          (match) => match[1]!,
        ),
      );

    expect(cacheKeys.length).toBeGreaterThanOrEqual(10);
    for (const prefix of cacheKeys) {
      expect(
        PERSONAL_STORAGE_PREFIXES.some((known) => prefix.startsWith(known)),
        prefix,
      ).toBe(true);
    }
  });
});

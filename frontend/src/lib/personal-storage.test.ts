import { readdirSync, readFileSync } from "node:fs";
import { join, relative } from "node:path";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
  stashAnnouncementStudents,
  takeAnnouncementStudents,
} from "./announcement-prefill";
import {
  bindPersonalStorage,
  clearPersonalStorage,
  isPersonalStorageKey,
  KEPT_STORAGE_KEYS,
  PERSONAL_STORAGE_PREFIXES,
  mayWritePersonalStorage,
  personalStorageOwner,
} from "./personal-storage";
import { SIDEBAR_MEMORY_ENTRIES, sidebarMemoryKey } from "./sidebar-memory";

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
  "moto-pwa-usage-reported.tenant.7.1": "1",
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

describe("mayWritePersonalStorage", () => {
  it("lets only the account the entries are bound to write", () => {
    bindPersonalStorage(OWNER);

    expect(mayWritePersonalStorage(OWNER)).toBe(true);
    expect(mayWritePersonalStorage("1:8")).toBe(false);
    expect(mayWritePersonalStorage("2:7")).toBe(false);
    expect(mayWritePersonalStorage(null)).toBe(false);
  });

  it("lets nobody write after a logout", () => {
    bindPersonalStorage(OWNER);

    clearPersonalStorage();

    expect(localStorage.getItem("moto:personal-storage-owner")).toBeNull();
    expect(mayWritePersonalStorage(OWNER)).toBe(false);
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

// Every key the frontend writes to web storage must be classified: personal
// (PERSONAL_STORAGE_PREFIXES, removed at logout and account switch) or kept
// (KEPT_STORAGE_KEYS, with its reason). The scan reads the keys out of the
// source, so a new key, including a key built from the account, fails here
// until someone decides which list it belongs to.
describe("inventory", () => {
  const SRC = join(process.cwd(), "src");

  // Files that read or write web storage, hand a key to a helper that does,
  // or build a badge cache key.
  const STORAGE_FILE =
    /\b(?:localStorage|sessionStorage)\b|useLocalStorageValue\(|useDemoDayNotice\(|StorageKey\b|\bcacheKey:/;

  // Storage files whose keys come from elsewhere. Each line names where.
  const KEYS_FROM_ELSEWHERE: Record<string, string> = {
    "src/lib/sidebar-memory.ts": "SIDEBAR_MEMORY_ENTRIES, checked below",
    "src/lib/hooks/use-local-storage-value.ts": "key from the caller",
    "src/lib/hooks/use-unread-count.ts": "cacheKey from the badge hooks",
    "src/lib/shell-auth-context.tsx": "DELIBERATE_LOGOUT_KEY (session-cache)",
    "src/lib/breadcrumb-context.tsx": "sidebar memory",
    "src/lib/ogs-group-live-api.ts": "mentions storage in a comment only",
    "src/app/[tenant]/(protected)/ogs-groups/page.tsx": "sidebar memory",
    "src/app/[tenant]/(protected)/students/[id]/page.tsx": "sidebar memory",
    "src/components/dashboard/sidebar.tsx": "sidebar memory",
    "src/app/[tenant]/page.tsx": "DELIBERATE_LOGOUT_KEY (session-cache)",
    "src/app/caterer/login/page.tsx": "DELIBERATE_LOGOUT_KEY (session-cache)",
    "src/app/school/login/page.tsx": "DELIBERATE_LOGOUT_KEY (session-cache)",
    "src/app/traeger/login/page.tsx": "DELIBERATE_LOGOUT_KEY (session-cache)",
    "src/components/tenant/tenant-relogin.ts":
      "DELIBERATE_LOGOUT_KEY (session-cache)",
    "src/components/traeger/traeger-shell.tsx":
      "DELIBERATE_LOGOUT_KEY (session-cache)",
    "src/components/ui/logout-modal.tsx":
      "DELIBERATE_LOGOUT_KEY (session-cache)",
    "src/components/notifications/notification-setup-dialog.tsx":
      "setupStorageKey (notification-setup-decision)",
    "src/components/notifications/deferred-notification-setup.tsx":
      "setupStorageKey (notification-setup-decision)",
  };

  const LITERAL =
    String.raw`("(?:[^"\\\n]|\\.)*"|'(?:[^'\\\n]|\\.)*'|` + "`[^`]*`)";
  const KEY_RULES = [
    // localStorage.setItem("key"), sessionStorage.getItem(`key:${id}`)
    String.raw`\.(?:getItem|setItem|removeItem)\(\s*` + LITERAL,
    String.raw`\buseLocalStorageValue\(\s*` + LITERAL,
    // const SEEN_KEY = "moto-demo-weekend-notice"
    String.raw`\bconst\s+[A-Z][A-Z0-9_]*KEY\s*=\s*` + LITERAL,
    // const key = `school-setup-opened:${accountID}`, const sessionKey = …
    String.raw`\b(?:const|let)\s+(?:key|\w*(?:[sS]torage|[sS]ession)Key)\s*=\s*` +
      LITERAL,
    // rateLimitStorageKey = "…" as default parameter or JSX prop
    String.raw`\b\w*StorageKey\s*=\s*` + LITERAL,
    String.raw`\bcacheKey:\s*` + LITERAL,
  ].map((rule) => new RegExp(rule, "g"));
  // What a function named …Key returns: `return "…";` or `? "…" : null`.
  const KEY_FUNCTION_RESULT = new RegExp(
    String.raw`(?:\breturn|(?<![?.])\?(?![?.]))\s*` +
      LITERAL +
      String.raw`(?=\s*(?:;|:\s*null))`,
    "g",
  );

  function sourceFiles(dir: string): string[] {
    return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
      const path = join(dir, entry.name);
      if (entry.isDirectory()) return sourceFiles(path);
      return /\.tsx?$/.test(entry.name) &&
        !/\.(?:test|spec|stories)\.tsx?$/.test(entry.name)
        ? [path]
        : [];
    });
  }

  function keyFunctionBodies(text: string): string[] {
    const bodies: string[] = [];
    for (const match of text.matchAll(/\bfunction\s+\w*Key\s*\(/g)) {
      let index = match.index + match[0].length;
      for (let depth = 1; depth > 0 && index < text.length; index += 1) {
        if (text[index] === "(") depth += 1;
        if (text[index] === ")") depth -= 1;
      }
      const open = text.indexOf("{", index);
      let end = open + 1;
      for (let depth = 1; depth > 0 && end < text.length; end += 1) {
        if (text[end] === "{") depth += 1;
        if (text[end] === "}") depth -= 1;
      }
      bodies.push(text.slice(open, end));
    }
    return bodies;
  }

  /** Keys of one file; `*` stands for a value filled in at runtime. */
  function storageKeys(text: string): string[] {
    const constants = new Map<string, string>();
    for (const match of text.matchAll(
      /\bconst\s+([A-Za-z_]\w*)\s*=\s*(?:"([^"\\\n]*)"|'([^'\\\n]*)'|(\d+)\b)/g,
    )) {
      constants.set(match[1]!, match[2] ?? match[3] ?? match[4]!);
    }
    const literals = KEY_RULES.flatMap((rule) =>
      Array.from(text.matchAll(rule), (match) => match[1]!),
    );
    for (const body of keyFunctionBodies(text)) {
      for (const match of body.matchAll(KEY_FUNCTION_RESULT)) {
        literals.push(match[1]!);
      }
    }
    return literals.map((literal) => {
      const inner = literal.slice(1, -1);
      if (!literal.startsWith("`")) return inner;
      return inner.replace(
        /\$\{\s*([A-Za-z_]\w*)\s*\}|\$\{[^}]*\}/g,
        (_, name?: string) => (name && constants.get(name)) ?? "*",
      );
    });
  }

  const keptPattern = (glob: string) =>
    new RegExp(
      `^${glob.replace(/[.+?^${}()|[\]\\]/g, "\\$&").replace(/\*/g, ".*")}$`,
    );
  const isPersonal = (key: string) =>
    PERSONAL_STORAGE_PREFIXES.some((prefix) => key.startsWith(prefix));
  const isKept = (key: string) =>
    KEPT_STORAGE_KEYS.some((glob) => keptPattern(glob).test(key));

  const scanned = sourceFiles(SRC)
    .map((path) => ({
      file: relative(process.cwd(), path),
      text: readFileSync(path, "utf8"),
    }))
    .filter(
      ({ file, text }) =>
        !file.startsWith("src/test/") && STORAGE_FILE.test(text),
    )
    .map(({ file, text }) => ({ file, keys: storageKeys(text) }));
  const allKeys = scanned.flatMap(({ keys }) => keys);

  it("finds the keys it is meant to find", () => {
    expect(allKeys).toEqual(
      expect.arrayContaining([
        "moto-pwa-usage-reported.*.*.*",
        "parentPasswordResetRateLimitUntil",
        "messages_unread_count:*:*",
        "collection-view:*:*:*",
        "moto.*.notification-setup.v1*.*",
        "parent-first-steps:v1:*",
        "moto:present-children-offering:*:*",
        "deliberateLogout",
      ]),
    );
  });

  it("classifies every key as personal or kept", () => {
    const unclassified = allKeys.filter(
      (key) => !key.startsWith("*") && !isPersonal(key) && !isKept(key),
    );
    expect(unclassified).toEqual([]);
  });

  it("never lists a key both as personal and as kept", () => {
    expect(allKeys.filter((key) => isPersonal(key) && isKept(key))).toEqual([]);
  });

  it("needs a known source for every storage file without own keys", () => {
    const unexplained = scanned
      .filter(({ file, keys }) => {
        const resolved = keys.filter((key) => !key.startsWith("*"));
        return resolved.length === 0 && !(file in KEYS_FROM_ELSEWHERE);
      })
      .map(({ file }) => file);
    expect(unexplained).toEqual([]);
    // A stale entry would hide a file that stopped touching storage.
    const files = new Set(scanned.map(({ file }) => file));
    expect(
      Object.keys(KEYS_FROM_ELSEWHERE).filter((file) => !files.has(file)),
    ).toEqual([]);
  });

  it("still finds every kept key in the source", () => {
    expect(
      KEPT_STORAGE_KEYS.filter(
        (glob) => !allKeys.some((key) => keptPattern(glob).test(key)),
      ),
    ).toEqual([]);
  });

  it("removes every sidebar memory entry", () => {
    for (const entry of SIDEBAR_MEMORY_ENTRIES) {
      expect(isPersonalStorageKey(sidebarMemoryKey(entry, OWNER)), entry).toBe(
        true,
      );
    }
  });
});

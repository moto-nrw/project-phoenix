import assert from "node:assert/strict";
import test from "node:test";

import {
  browserMismatch,
  orphanedBaselines,
  parseKitPixelArgs,
  selectStories,
} from "./kit-pixel-lib.mjs";

const index = {
  v: 5,
  entries: {
    "components-ui-modal--default": {
      type: "story",
      id: "components-ui-modal--default",
      importPath: "./src/components/ui/modal.stories.tsx",
    },
    "components-ui-modal--docs": {
      type: "docs",
      id: "components-ui-modal--docs",
      importPath: "./src/components/ui/modal.stories.tsx",
    },
    "ui-page-header-overflowmenu--default": {
      type: "story",
      id: "ui-page-header-overflowmenu--default",
      importPath: "./src/components/ui/page-header/OverflowMenu.stories.tsx",
    },
    "components-auth-passwordresetmodal--success": {
      type: "story",
      id: "components-auth-passwordresetmodal--success",
      importPath: "./src/components/auth/password-reset-modal.stories.tsx",
    },
  },
};

test("selects only kit stories by default, sorted by id", () => {
  assert.deepEqual(
    selectStories(index, {}).map((story) => story.id),
    ["components-ui-modal--default", "ui-page-header-overflowmenu--default"],
  );
});

test("--all-stories includes stories outside components/ui", () => {
  assert.deepEqual(
    selectStories(index, { allStories: true }).map((story) => story.id),
    [
      "components-auth-passwordresetmodal--success",
      "components-ui-modal--default",
      "ui-page-header-overflowmenu--default",
    ],
  );
});

test("the filter is a regular expression on the story id", () => {
  assert.deepEqual(
    selectStories(index, { filter: "overflowmenu|^components-ui-modal--" }).map(
      (story) => story.id,
    ),
    ["components-ui-modal--default", "ui-page-header-overflowmenu--default"],
  );
  assert.deepEqual(selectStories(index, { filter: "^modal" }), []);
});

test("an invalid filter fails with the offending pattern", () => {
  assert.throws(() => selectStories(index, { filter: "(" }), /"\("/);
});

test("rejects an index format it does not know", () => {
  assert.throws(
    () => selectStories({ v: 4, stories: {} }, {}),
    /index\.json/,
  );
});

test("parses baseline with ref and filter", () => {
  assert.deepEqual(
    parseKitPixelArgs(["baseline", "--ref", "development", "--filter", "modal"]),
    {
      command: "baseline",
      ref: "development",
      filter: "modal",
      allStories: false,
    },
  );
});

test("baseline defaults to HEAD; compare has no ref", () => {
  assert.equal(parseKitPixelArgs(["baseline"]).ref, "HEAD");
  assert.deepEqual(parseKitPixelArgs(["compare", "--all-stories"]), {
    command: "compare",
    ref: undefined,
    filter: undefined,
    allStories: true,
  });
});

test("ignores the -- separator pnpm forwards", () => {
  assert.equal(parseKitPixelArgs(["--", "compare"]).command, "compare");
});

test("rejects a missing or unknown command and --ref on compare", () => {
  assert.throws(() => parseKitPixelArgs([]), /baseline|compare/);
  assert.throws(() => parseKitPixelArgs(["update"]), /baseline|compare/);
  assert.throws(
    () => parseKitPixelArgs(["compare", "--ref", "HEAD"]),
    /--ref/,
  );
  assert.throws(() => parseKitPixelArgs(["compare", "--nope"]), /nope/);
});

test("names both browser versions when they differ", () => {
  assert.equal(browserMismatch("156.0.1.2", "156.0.1.2"), null);
  assert.match(
    browserMismatch("156.0.1.2", "157.0.0.1"),
    /156\.0\.1\.2.*157\.0\.0\.1/s,
  );
});

test("names baseline images whose story no longer exists", () => {
  assert.deepEqual(
    orphanedBaselines(
      ["components-ui-modal--default.png", "components-ui-modal--old.png"],
      ["components-ui-modal--default", "components-ui-modal--new"],
    ),
    ["components-ui-modal--old"],
  );
  assert.deepEqual(orphanedBaselines([], ["components-ui-modal--new"]), []);
});

import { builtinModules } from "node:module";
import { promisify } from "node:util";

import { afterAll, vi } from "vitest";

/**
 * Worker reuse with a per-file reset (docs/agents/frontend-testing.md).
 *
 * The projects run with `isolate: false`, so a worker keeps its happy-dom
 * window and the natively loaded packages (react, react-dom, happy-dom) for
 * many test files instead of loading them again for every file. What a fresh
 * worker would not have is reset after each file:
 *
 * - every module the Vite module runner evaluated, including its fetched
 *   meta and automock cache, and every vi.mock registration;
 * - the file's vi.spyOn spies (restored) and its mocks in the registry of
 *   vi.clearAllMocks and vi.resetAllMocks;
 * - own properties of globalThis, of every global constructor and its
 *   prototype, of Node's builtin modules with their exported classes,
 *   functions and prototypes, of the window's prototype chains, and the
 *   string keys of document, html, head, body, navigator, location, history
 *   and screen;
 * - process.env, timer mode, the DOM tree and its attributes, cookies, URL
 *   and viewport;
 * - listeners added to window, document, html, head and body;
 * - Node and window timers the file started that are still pending.
 *
 * A property the reset cannot restore fails the file instead of reaching the
 * next one. This file must be the first setup file of its project.
 */

type Descriptors = Map<PropertyKey, PropertyDescriptor>;

interface ObjectSnapshot {
  target: object;
  /**
   * happy-dom instances keep their internal state in symbol-keyed fields;
   * only their string keys (React's `__reactFiber$…`, test overrides) are
   * restored.
   */
  stringKeysOnly: boolean;
  descriptors: Descriptors;
}

interface WorkerBaseline {
  objects: ObjectSnapshot[];
  /** Values read through populated window getters, by key. */
  globalValues: Map<PropertyKey, unknown>;
  env: Record<string, string | undefined>;
  dom: DomBaseline | null;
  listeners: Array<[EventTarget, string, unknown, unknown]>;
  /** Node timers a test file started and that have not run or been cleared. */
  timers: Set<NodeJS.Timeout | NodeJS.Immediate>;
  mocks: MockRegistry;
  /** setImmediate as it was before tracking, for the reset itself. */
  untrackedImmediate: typeof setImmediate;
  clearTimers?: () => void;
}

interface DomBaseline {
  /** Empties the browser context's cookie jar (all paths and domains). */
  clearCookies: () => void;
  /**
   * Clears the window's pending setTimeout/setInterval timers. A fresh
   * worker ends them with the thread; left running, an interval of an
   * unmounted component keeps calling the next file's globals (fetch).
   */
  clearWindowTimers: () => void;
  href: string;
  width: number;
  height: number;
  htmlAttributes: Array<[string, string]>;
  bodyAttributes: Array<[string, string]>;
}

interface HappyDomApi {
  setURL(url: string): void;
  setWindowSize(size: { width?: number; height?: number }): void;
}

const STATE = Symbol.for("moto.test.worker-reuse");
/**
 * Registrations of natively loaded packages that live as long as the worker,
 * like the package itself: signal-exit defines its process-wide emitter
 * non-configurable on first import and expects to find it again.
 */
const WORKER_LIFETIME_KEYS = new Set<PropertyKey>([
  Symbol.for("signal-exit emitter"),
]);
const VITEST_INTERNAL = [
  /\/vitest\/dist\//,
  /vitest-virtual-\w+\/dist/,
  /@vitest\/dist/,
];

interface ModuleNode {
  promise?: unknown;
  exports?: unknown;
  /** Automocked exports, cached per module node by Vitest's mocker. */
  mockedExports?: unknown;
  evaluated: boolean;
  importers: Set<string>;
}

interface WorkerGlobals {
  __vitest_worker__?: {
    /** Module ids the runner is still resolving. */
    resolvingModules: Set<string>;
    evaluatedModules: {
      idToModuleMap: Map<string, ModuleNode>;
      invalidateModule(node: ModuleNode): void;
    };
  };
  __vitest_mocker__?: { reset(): void };
  happyDOM?: HappyDomApi;
}

function ownKeys(target: object, stringKeysOnly: boolean): PropertyKey[] {
  const keys = Reflect.ownKeys(target);
  return stringKeysOnly ? keys.filter((key) => typeof key === "string") : keys;
}

function ownDescriptors(target: object, stringKeysOnly: boolean): Descriptors {
  const descriptors: Descriptors = new Map();
  for (const key of ownKeys(target, stringKeysOnly)) {
    const descriptor = Reflect.getOwnPropertyDescriptor(target, key);
    if (descriptor) descriptors.set(key, descriptor);
  }
  return descriptors;
}

/**
 * Builtins left out of the snapshot: `_http_server` and the other internal
 * aliases export what their public modules export, `node:test` and the other
 * prefix-only modules are special or experimental, `punycode`, `sys` and
 * `wasi` warn when loaded, and worker threads do not offer `trace_events`.
 */
const UNSNAPSHOTTED_BUILTINS = new Set([
  "punycode",
  "sys",
  "trace_events",
  "wasi",
]);

/**
 * Node's builtin modules, their exported classes and functions and those
 * prototypes. A builtin is loaded once per worker, like the natively loaded
 * packages; a file that patches one (the session cookie exit lock wraps
 * `ServerResponse.prototype.writeHead`, a test spies on `fs.readFileSync`)
 * would hand its patch, with the file's own module instances in its closure,
 * to every later file. Only descriptors are read: deprecated getters such
 * as `fs.F_OK` warn when called.
 */
const builtinNames = new WeakMap<object, string>();

function builtinTargets(): object[] {
  const targets: object[] = [];
  for (const name of builtinModules) {
    if (name.startsWith("_") || name.startsWith("node:")) continue;
    if (UNSNAPSHOTTED_BUILTINS.has(name)) continue;
    const exports = process.getBuiltinModule(name) as object;
    builtinNames.set(exports, `node:${name}`);
    targets.push(exports);
    for (const key of Reflect.ownKeys(exports)) {
      const value: unknown = Reflect.getOwnPropertyDescriptor(
        exports,
        key,
      )?.value;
      if (typeof value !== "function") continue;
      targets.push(value);
      const prototype: unknown = value.prototype;
      if (prototype && typeof prototype === "object") targets.push(prototype);
    }
  }
  return targets;
}

function snapshotTargets(): ObjectSnapshot[] {
  const targets = new Set<object>([globalThis, ...builtinTargets()]);
  for (const key of Reflect.ownKeys(globalThis)) {
    let value: unknown;
    try {
      value = Reflect.get(globalThis, key);
    } catch {
      continue;
    }
    if (typeof value !== "function") continue;
    targets.add(value);
    const prototype = (value as { prototype?: unknown }).prototype;
    if (prototype && typeof prototype === "object") targets.add(prototype);
  }
  if (typeof process !== "undefined") targets.add(process);
  if (typeof document !== "undefined") {
    // Prototype chains of the window's own instances, including classes the
    // window does not expose as globals.
    for (const instance of [document, document.body, navigator, location]) {
      let prototype = Object.getPrototypeOf(instance);
      while (prototype && prototype !== Object.prototype) {
        targets.add(prototype);
        prototype = Object.getPrototypeOf(prototype);
      }
    }
  }
  const snapshots = [...targets].map((target) => ({
    target,
    stringKeysOnly: false,
    descriptors: ownDescriptors(target, false),
  }));
  if (typeof document !== "undefined") {
    const instances = [
      document,
      document.documentElement,
      document.head,
      document.body,
      navigator,
      location,
      history,
      screen,
    ];
    for (const target of instances) {
      snapshots.push({
        target,
        stringKeysOnly: true,
        descriptors: ownDescriptors(target, true),
      });
    }
  }
  return snapshots;
}

function sameDescriptor(a: PropertyDescriptor, b: PropertyDescriptor) {
  return (
    a.get === b.get &&
    a.set === b.set &&
    Object.is(a.value, b.value) &&
    a.writable === b.writable &&
    a.enumerable === b.enumerable &&
    a.configurable === b.configurable
  );
}

/** Returns the keys it could not return to their baseline. */
function restoreObject({
  target,
  stringKeysOnly,
  descriptors,
}: ObjectSnapshot): PropertyKey[] {
  const failed: PropertyKey[] = [];
  for (const key of ownKeys(target, stringKeysOnly)) {
    if (descriptors.has(key) || key === STATE) continue;
    if (WORKER_LIFETIME_KEYS.has(key)) continue;
    if (typeof key === "string" && key.startsWith("__vitest")) continue;
    if (!Reflect.deleteProperty(target, key)) failed.push(key);
  }
  for (const [key, descriptor] of descriptors) {
    const current = Reflect.getOwnPropertyDescriptor(target, key);
    if (current && sameDescriptor(current, descriptor)) continue;
    if (current && !current.configurable) {
      if (!current.writable || !("value" in descriptor)) failed.push(key);
      else if (!Reflect.set(target, key, descriptor.value)) failed.push(key);
      continue;
    }
    if (!Reflect.defineProperty(target, key, descriptor)) failed.push(key);
  }
  return failed;
}

function describeTarget(target: object): string {
  if (target === globalThis) return "globalThis";
  const builtin = builtinNames.get(target);
  if (builtin) return builtin;
  if (typeof target === "function") return target.name || "function";
  const owner = (target as { constructor?: { name?: string } }).constructor;
  return Object.hasOwn(target, "constructor")
    ? `${owner?.name ?? "?"}.prototype`
    : `${owner?.name ?? "object"} instance`;
}

function attributes(element: Element): Array<[string, string]> {
  return Array.from(element.attributes, (attr) => [attr.name, attr.value]);
}

function restoreAttributes(element: Element, wanted: Array<[string, string]>) {
  for (const name of element.getAttributeNames()) {
    element.removeAttribute(name);
  }
  for (const [name, value] of wanted) element.setAttribute(name, value);
}

/**
 * Records listeners on the nodes that outlive a test file: window, document,
 * html, head and body. React portals into document.body attach root listeners
 * there; left in place they reach the next file's React roots.
 */
function trackListeners(baseline: WorkerBaseline) {
  if (typeof document === "undefined") return;
  const persistent = new Set<EventTarget>([
    document,
    document.documentElement,
    document.head,
    document.body,
  ]);
  // The prototype that owns addEventListener for DOM nodes; happy-dom's
  // node classes need not inherit from the global EventTarget.
  let prototype: EventTarget = Object.getPrototypeOf(document.body);
  while (!Object.hasOwn(prototype, "addEventListener")) {
    prototype = Object.getPrototypeOf(prototype);
  }
  const nodeAdd = prototype.addEventListener;
  prototype.addEventListener = function (
    this: EventTarget,
    type,
    listener,
    options,
  ) {
    if (persistent.has(this)) {
      baseline.listeners.push([this, type, listener, options]);
    }
    return nodeAdd.call(this, type, listener, options);
  };
  // The global addEventListener is bound to the happy-dom window by
  // vitest's populateGlobal and bypasses the prototype.
  const windowAdd = globalThis.addEventListener as (
    type: string,
    listener: EventListenerOrEventListenerObject | null,
    options?: boolean | AddEventListenerOptions,
  ) => void;
  globalThis.addEventListener = function (
    type: string,
    listener: EventListenerOrEventListenerObject | null,
    options?: boolean | AddEventListenerOptions,
  ) {
    baseline.listeners.push([globalThis, type, listener, options]);
    windowAdd(type, listener, options);
  } as typeof globalThis.addEventListener;
}

/**
 * A new property defined on a restored object (`Object.defineProperty(
 * document, "visibilityState", { value })`) defaults to non-configurable and
 * could never be removed again. Such definitions stay configurable here, so
 * the reset can return the object to its baseline.
 */
function keepDefinitionsRemovable(restored: WeakSet<object>) {
  const configurable = (
    target: object,
    key: PropertyKey,
    descriptor: PropertyDescriptor,
  ): PropertyDescriptor =>
    restored.has(target) &&
    descriptor.configurable === undefined &&
    !Object.hasOwn(target, key)
      ? { ...descriptor, configurable: true }
      : descriptor;
  const define = Object.defineProperty;
  Object.defineProperty = function <T>(
    target: T,
    key: PropertyKey,
    descriptor: PropertyDescriptor & ThisType<unknown>,
  ): T {
    return define(target, key, configurable(target as object, key, descriptor));
  };
  const defineAll = Object.defineProperties;
  Object.defineProperties = function <T>(
    target: T,
    descriptors: PropertyDescriptorMap & ThisType<unknown>,
  ): T {
    const adjusted: PropertyDescriptorMap = {};
    for (const key of Reflect.ownKeys(descriptors)) {
      const descriptor = descriptors[key as keyof typeof descriptors]!;
      Reflect.set(
        adjusted,
        key,
        configurable(target as object, key, descriptor),
      );
    }
    return defineAll(target, adjusted);
  };
  const reflectDefine = Reflect.defineProperty;
  Reflect.defineProperty = (target, key, descriptor) =>
    reflectDefine(target, key, configurable(target, key, descriptor));
}

interface MockRegistry {
  /** Every mock function the worker created and has not dropped yet. */
  registered: Set<unknown>;
  /** Members of `registered` that predate the first test file. */
  baseline: Set<unknown>;
}

/**
 * @vitest/spy keeps every mock function it creates in one module-level set,
 * which vi.clearAllMocks and vi.resetAllMocks walk. The package is loaded once
 * per worker, so in a reused worker the set holds the mocks of every earlier
 * file: each clearAllMocks call (most files call it before every test) walks
 * all of them, and the set keeps their recorded calls and the earlier files'
 * module instances in their closures alive. A fresh worker has none of them.
 *
 * The set is not exported; clearAllMocks hands it over as the receiver of
 * Set.prototype.forEach, which is stubbed for that one call so no mock is
 * cleared.
 */
function mockRegistry(): MockRegistry {
  const forEach = Set.prototype.forEach;
  let registered: Set<unknown> | undefined;
  // oxlint-disable-next-line no-extend-native -- stubbed for this one call, restored below
  Set.prototype.forEach = function (this: Set<unknown>) {
    registered = this;
  };
  try {
    vi.clearAllMocks();
  } finally {
    // oxlint-disable-next-line no-extend-native -- restores the original method
    Set.prototype.forEach = forEach;
  }
  const probe = vi.fn();
  if (!registered?.delete(probe)) {
    throw new Error(
      "setup-worker-reuse: vi.clearAllMocks no longer walks the mock registry with Set.prototype.forEach; update mockRegistry()",
    );
  }
  return { registered, baseline: new Set(registered) };
}

interface WindowTimers {
  runningTimers: NodeJS.Timeout[];
}

async function browserState(): Promise<
  Pick<DomBaseline, "clearCookies" | "clearWindowTimers">
> {
  const { PropertySymbol } = await import("happy-dom");
  const { default: WindowBrowserContext } =
    await import("happy-dom/lib/window/WindowBrowserContext.js");
  const window = Reflect.get(
    document,
    PropertySymbol.window,
  ) as ConstructorParameters<typeof WindowBrowserContext>[0];
  const context = new WindowBrowserContext(window);
  const container = context.getBrowserContext()?.cookieContainer;
  const frame = context.getBrowserFrame();
  if (!container || !frame) {
    throw new Error("happy-dom window without browser context");
  }
  const tasks = Reflect.get(
    frame,
    PropertySymbol.asyncTaskManager,
  ) as unknown as WindowTimers;
  return {
    clearCookies: () => container.clearCookies(),
    clearWindowTimers: () => {
      for (const timer of tasks.runningTimers.splice(0)) clearTimeout(timer);
    },
  };
}

/**
 * Records the Node timers a test file starts. A fresh worker ends pending
 * timers with its thread; in a reused one a debounce or retry of the previous
 * file would otherwise run inside the next file and call its mocks or fetch.
 *
 * Modules loaded once per worker keep their own timers across files: React's
 * scheduler captures setTimeout and setImmediate when react-dom loads, so
 * react-dom is loaded before the wrappers exist (see createBaseline).
 */
function trackTimers(baseline: WorkerBaseline) {
  const { timers } = baseline;
  const once = <Handle extends NodeJS.Timeout | NodeJS.Immediate>(
    schedule: (callback: () => void) => Handle,
    callback: (...args: unknown[]) => void,
    args: unknown[],
  ): Handle => {
    const handle: Handle = schedule(() => {
      timers.delete(handle);
      callback(...args);
    });
    timers.add(handle);
    return handle;
  };
  const setTimeoutOriginal = globalThis.setTimeout;
  const setIntervalOriginal = globalThis.setInterval;
  const setImmediateOriginal = globalThis.setImmediate;
  const clearTimeoutOriginal = globalThis.clearTimeout;
  const clearIntervalOriginal = globalThis.clearInterval;
  const clearImmediateOriginal = globalThis.clearImmediate;
  const setTimeoutTracked = (
    callback: (...args: unknown[]) => void,
    delay?: number,
    ...args: unknown[]
  ) => once((run) => setTimeoutOriginal(run, delay), callback, args);
  const setIntervalTracked = (
    callback: (...args: unknown[]) => void,
    delay?: number,
    ...args: unknown[]
  ) => {
    const handle = setIntervalOriginal(callback, delay, ...args);
    timers.add(handle);
    return handle;
  };
  const setImmediateTracked = (
    callback: (...args: unknown[]) => void,
    ...args: unknown[]
  ) => once((run) => setImmediateOriginal(run), callback, args);
  // Keep util.promisify(setTimeout) and friends working.
  for (const [tracked, original] of [
    [setTimeoutTracked, setTimeoutOriginal],
    [setImmediateTracked, setImmediateOriginal],
  ] as const) {
    Reflect.set(
      tracked,
      promisify.custom,
      Reflect.get(original, promisify.custom),
    );
  }
  globalThis.setTimeout = setTimeoutTracked as typeof setTimeout;
  globalThis.setInterval = setIntervalTracked as typeof setInterval;
  globalThis.setImmediate = setImmediateTracked as typeof setImmediate;
  globalThis.clearTimeout = ((handle?: NodeJS.Timeout) => {
    if (handle) timers.delete(handle);
    clearTimeoutOriginal(handle);
  }) as typeof clearTimeout;
  globalThis.clearInterval = ((handle?: NodeJS.Timeout) => {
    if (handle) timers.delete(handle);
    clearIntervalOriginal(handle);
  }) as typeof clearInterval;
  globalThis.clearImmediate = ((handle?: NodeJS.Immediate) => {
    if (handle) timers.delete(handle);
    clearImmediateOriginal(handle);
  }) as typeof clearImmediate;
  return () => {
    for (const handle of timers) {
      if ("hasRef" in handle && "refresh" in handle) {
        clearTimeoutOriginal(handle as NodeJS.Timeout);
      } else {
        clearImmediateOriginal(handle as NodeJS.Immediate);
      }
    }
    timers.clear();
  };
}

async function createBaseline(): Promise<WorkerBaseline> {
  const baseline: WorkerBaseline = {
    objects: [],
    globalValues: new Map(),
    env: { ...process.env },
    dom: null,
    listeners: [],
    timers: new Set(),
    mocks: mockRegistry(),
    untrackedImmediate: setImmediate,
  };
  // React's scheduler keeps its timer functions from the first load on.
  if (typeof document !== "undefined") await import("react-dom/client");
  baseline.clearTimers = trackTimers(baseline);
  const restored = new WeakSet<object>();
  trackListeners(baseline);
  keepDefinitionsRemovable(restored);
  baseline.objects = snapshotTargets();
  for (const { target } of baseline.objects) restored.add(target);
  // Window properties are getters/setters on globalThis (vitest
  // populateGlobal); an assignment such as `window.fetch = vi.fn()` keeps
  // the descriptor and only changes what the getter returns.
  for (const [key, descriptor] of baseline.objects[0]!.descriptors) {
    if (!descriptor.get || !descriptor.set) continue;
    try {
      const first = Reflect.get(globalThis, key);
      if (Object.is(first, Reflect.get(globalThis, key))) {
        baseline.globalValues.set(key, first);
      }
    } catch {
      // A getter that throws has nothing to restore.
    }
  }
  if (typeof document !== "undefined") {
    baseline.dom = {
      ...(await browserState()),
      href: location.href,
      width: innerWidth,
      height: innerHeight,
      htmlAttributes: attributes(document.documentElement),
      bodyAttributes: attributes(document.body),
    };
  }
  return baseline;
}

function resetDom(dom: DomBaseline) {
  const happyDOM = (globalThis as WorkerGlobals).happyDOM;
  document.body.replaceChildren();
  document.head.replaceChildren();
  document.title = "";
  restoreAttributes(document.documentElement, dom.htmlAttributes);
  restoreAttributes(document.body, dom.bodyAttributes);
  dom.clearCookies();
  dom.clearWindowTimers();
  happyDOM?.setURL(dom.href);
  happyDOM?.setWindowSize({ width: dom.width, height: dom.height });
}

function resetModules() {
  const worker = globalThis as WorkerGlobals;
  worker.__vitest_mocker__?.reset();
  const modules = worker.__vitest_worker__?.evaluatedModules;
  modules?.idToModuleMap.forEach((node, path) => {
    if (VITEST_INTERNAL.some((pattern) => pattern.test(path))) return;
    // Vitest keeps a node's fetched meta (including a file's mock
    // decision) for the worker's lifetime; invalidating it makes the next
    // file fetch and evaluate the module like a fresh worker would.
    modules.invalidateModule(node);
    node.importers.clear();
    node.mockedExports = undefined;
  });
}

/**
 * Waits until no import of the file is still being fetched or evaluated, as
 * Vitest's own waitForImportsToResolve does before a worker ends. An import
 * still in flight at the reset would otherwise finish inside the next file,
 * against that file's mocks.
 */
async function settleImports(nextMacrotask: typeof setImmediate) {
  const worker = (globalThis as WorkerGlobals).__vitest_worker__;
  if (!worker) return;
  for (;;) {
    await new Promise((resolve) => nextMacrotask(resolve));
    const pending: unknown[] = [];
    for (const node of worker.evaluatedModules.idToModuleMap.values()) {
      if (node.promise && !node.evaluated) pending.push(node.promise);
    }
    if (!pending.length && !worker.resolvingModules.size) return;
    await Promise.allSettled(pending);
  }
}

function dropFileMocks({ registered, baseline }: MockRegistry) {
  for (const mock of registered) {
    if (!baseline.has(mock)) registered.delete(mock);
  }
}

function resetWorker(baseline: WorkerBaseline) {
  baseline.clearTimers?.();
  // Spied globals and builtins return to their baseline below anyway;
  // restoring the spies also undoes those on natively loaded packages,
  // which the snapshot does not cover.
  vi.restoreAllMocks();
  dropFileMocks(baseline.mocks);
  for (const [target, type, listener, options] of baseline.listeners.splice(
    0,
  )) {
    target.removeEventListener(
      type,
      listener as EventListener,
      options as EventListenerOptions,
    );
  }
  if (baseline.dom) resetDom(baseline.dom);
  const unrestored = baseline.objects.flatMap((snapshot) =>
    restoreObject(snapshot).map(
      (key) => `${describeTarget(snapshot.target)}.${String(key)}`,
    ),
  );
  for (const [key, value] of baseline.globalValues) {
    if (!Object.is(Reflect.get(globalThis, key), value)) {
      Reflect.set(globalThis, key, value);
    }
  }
  for (const key of Object.keys(process.env)) {
    if (!(key in baseline.env)) delete process.env[key];
  }
  for (const [key, value] of Object.entries(baseline.env)) {
    if (process.env[key] !== value) process.env[key] = value;
  }
  resetModules();
  if (unrestored.length) {
    throw new Error(
      `setup-worker-reuse: this file left ${unrestored.join(", ")} changed beyond reset; the next file in this worker would see it`,
    );
  }
}

const holder = globalThis as typeof globalThis & {
  [STATE]?: WorkerBaseline;
};
const baseline = (holder[STATE] ??= await createBaseline());

afterAll(async () => {
  // A file's fake timers end with it; the next file's setup freezes the clock
  // again.
  vi.useRealTimers();
  // Let promise chains and imports the file started finish in its own
  // context.
  await settleImports(baseline.untrackedImmediate);
  resetWorker(baseline);
});

import { setFlagsFromString } from "node:v8";
import { runInNewContext } from "node:vm";
import { Worker } from "node:worker_threads";
import { ThreadsPoolWorker } from "vitest/node";
import type { PoolOptions, PoolWorker } from "vitest/node";

/**
 * Vitest pool for projects with `isolate: false` and the per-file reset in
 * `src/test/setup-worker-reuse.ts`: Vitest's own thread worker, started with
 * a bounded V8 heap and replaced once it ends a test file above a heap size.
 *
 * Without bounds a reused thread lets its heap grow towards V8's default
 * limit (about 4 GB per thread on a 16 GB+ machine) before it collects. The
 * limits make V8 collect early; replacing a thread after a heavy file hands
 * its memory back. A file that needs more old space than the limit fails
 * with "Worker exited unexpectedly" instead of passing on borrowed memory.
 */
export interface ReusedThreadsOptions {
  /** V8 old-space limit per worker thread. */
  maxOldGenerationSizeMb: number;
  /** V8 young-generation limit per worker thread. */
  maxYoungGenerationSizeMb: number;
  /** Heap use after a test file above which the thread is replaced. */
  recycleHeapMb: number;
}

type PoolTask = Parameters<NonNullable<PoolWorker["canReuse"]>>[0];

class ReusedThreadsWorker extends ThreadsPoolWorker {
  override readonly name = "reused-threads";
  // Vitest then reports the thread's heap use after every test file.
  readonly reportMemory = true;
  private heapUsed = 0;
  private readonly environment: string;

  constructor(
    options: PoolOptions,
    private readonly limits: ReusedThreadsOptions,
  ) {
    super(options);
    this.environment = JSON.stringify(options.environment);
  }

  override async start() {
    // ThreadsPoolWorker creates its thread only when none exists yet
    // (`this._thread ||= new Worker(...)`); this one carries the limits.
    const self = this as unknown as { _thread?: Worker };
    self._thread ??= new Worker(this.entrypoint, {
      env: this.env,
      execArgv: this.execArgv,
      stdout: true,
      stderr: true,
      resourceLimits: {
        maxOldGenerationSizeMb: this.limits.maxOldGenerationSizeMb,
        maxYoungGenerationSizeMb: this.limits.maxYoungGenerationSizeMb,
      },
    });
    await super.start();
    this.on("message", (message: { type?: string; usedMemory?: number }) => {
      if (message?.type === "testfileFinished" && message.usedMemory) {
        this.heapUsed = message.usedMemory;
      }
    });
  }

  // Asked before the next file of the same project; false ends this thread.
  canReuse(task: PoolTask) {
    return (
      this.heapUsed < this.limits.recycleHeapMb * 1024 * 1024 &&
      JSON.stringify(task.context.environment) === this.environment
    );
  }
}

export function reusedThreads(limits: ReusedThreadsOptions) {
  return {
    name: "reused-threads",
    createPoolWorker: (options: PoolOptions): PoolWorker =>
      new ReusedThreadsWorker(options, limits),
  };
}

/**
 * Collects the Vitest main process's garbage once its heap exceeds a size.
 * V8 would otherwise let it grow by several hundred megabytes of transform
 * and RPC garbage before collecting. `--expose-gc` set at run time exposes
 * `gc` to contexts created afterwards.
 */
export function collectMainProcessGarbage({
  aboveHeapMb,
  everyMs,
}: {
  aboveHeapMb: number;
  everyMs: number;
}) {
  setFlagsFromString("--expose-gc");
  const gc = runInNewContext("gc") as () => void;
  const limit = aboveHeapMb * 1024 * 1024;
  setInterval(() => {
    if (process.memoryUsage().heapUsed > limit) gc();
  }, everyMs).unref();
}

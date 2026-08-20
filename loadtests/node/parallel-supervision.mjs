const apiBaseUrl = requiredEnv("API_BASE_URL").replace(/\/$/, "");
const staffTokens = csvEnv("WEB_JWTS", 2);
const timetableInstanceId = requiredEnv("TIMETABLE_INSTANCE_ID");
const activeGroupId = requiredEnv("ACTIVE_GROUP_ID");
const studentBatches = requiredEnv("STUDENT_IDS")
  .split(";")
  .map((batch) =>
    batch
      .split(",")
      .map((id) => id.trim())
      .filter(Boolean),
  );

const expectedStudentsPerStaff = 10;
const updateBudgetMs = 2000;
const actionIntervalMs = 500;

if (
  studentBatches.length !== 2 ||
  studentBatches.some((batch) => batch.length !== expectedStudentsPerStaff)
) {
  throw new Error(
    "STUDENT_IDS must contain two semicolon-separated batches of 10 IDs",
  );
}
if (new Set(studentBatches.flat()).size !== 20) {
  throw new Error("STUDENT_IDS must contain 20 distinct IDs");
}

const streams = staffTokens.map((token, index) => startSSE(token, index));

try {
  await Promise.all(streams.map((stream) => stream.ready));
  const results = await Promise.all(
    staffTokens.map((token, index) => runStaffSequence(token, index)),
  );
  const latencies = results.flat();
  const maxLatency = Math.max(...latencies);
  const averageLatency =
    latencies.reduce((sum, value) => sum + value, 0) / latencies.length;

  console.log(
    JSON.stringify({
      checkins: latencies.length,
      rate_limit_responses: 0,
      sse_delivery_average_ms: Math.round(averageLatency),
      sse_delivery_max_ms: Math.round(maxLatency),
      sse_delivery_budget_ms: updateBudgetMs,
    }),
  );
} finally {
  for (const stream of streams) stream.controller.abort();
  await Promise.allSettled(streams.map((stream) => stream.done));
}

async function runStaffSequence(token, index) {
  const latencies = [];
  for (const studentId of studentBatches[index]) {
    const observer = streams[index === 0 ? 1 : 0];
    // Both staff act in the same round. Waiting for two combined frames on
    // the colleague's stream proves that both writes reached the other tab.
    const observed = observer.waitFor(activeGroupId, 2);
    try {
      await postCheckin(token, index, studentId);
    } catch (error) {
      observer.cancelWait(activeGroupId);
      throw error;
    }
    latencies.push(await observed);
    await sleep(actionIntervalMs);
  }
  return latencies;
}

async function postCheckin(token, index, studentId) {
  const response = await fetch(
    `${apiBaseUrl}/api/timetable/operations/instances/${timetableInstanceId}/students/${studentId}/check-in`,
    {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
        Accept: "application/json",
      },
    },
  );
  if (response.ok) return;
  if (response.status === 429) {
    throw new Error(
      `staff ${index + 1}, student ${studentId}: received HTTP 429`,
    );
  }
  const body = (await response.text()).slice(0, 200);
  throw new Error(
    `staff ${index + 1}, student ${studentId}: HTTP ${response.status}: ${body}`,
  );
}

function startSSE(token, index) {
  const controller = new AbortController();
  const ready = deferred();
  const waiters = new Map();
  const stream = {
    controller,
    ready: ready.promise,
    waitFor: (groupId, frames) =>
      waitForRefresh(waiters, index, groupId, frames),
    cancelWait: (activeGroupId) => cancelWait(waiters, activeGroupId),
    done: null,
  };
  stream.done = consumeSSE(token, index, controller, ready.resolve, (event) =>
    observeRefresh(waiters, event),
  ).catch((error) => {
    ready.reject(error);
    if (!controller.signal.aborted) throw error;
  });
  return stream;
}

function waitForRefresh(waiters, index, activeGroupId, expectedFrames) {
  if (waiters.has(activeGroupId)) {
    throw new Error(
      `staff ${index + 1}: overlapping waiter for active group ${activeGroupId}`,
    );
  }
  const startedAt = performance.now();
  return new Promise((resolve, reject) => {
    const timeout = setTimeout(() => {
      waiters.delete(activeGroupId);
      reject(
        new Error(
          `staff ${index + 1}: no combined SSE refresh within ${updateBudgetMs} ms`,
        ),
      );
    }, updateBudgetMs);
    waiters.set(activeGroupId, {
      startedAt,
      resolve,
      timeout,
      remaining: expectedFrames,
    });
  });
}

function cancelWait(waiters, activeGroupId) {
  const waiter = waiters.get(activeGroupId);
  if (!waiter) return;
  waiters.delete(activeGroupId);
  clearTimeout(waiter.timeout);
  waiter.resolve(0);
}

function observeRefresh(waiters, event) {
  if (!event.data?.reason || !event.active_group_id) return;
  const activeGroupId = String(event.active_group_id);
  const waiter = waiters.get(activeGroupId);
  if (!waiter) return;
  waiter.remaining--;
  if (waiter.remaining > 0) return;
  waiters.delete(activeGroupId);
  clearTimeout(waiter.timeout);
  waiter.resolve(performance.now() - waiter.startedAt);
}

async function consumeSSE(token, index, controller, onConnected, onRefresh) {
  const response = await fetch(`${apiBaseUrl}/api/sse/events`, {
    headers: { Authorization: `Bearer ${token}`, Accept: "text/event-stream" },
    signal: controller.signal,
  });
  if (!response.ok || !response.body) {
    throw new Error(
      `staff ${index + 1}: SSE failed to open: HTTP ${response.status}`,
    );
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  const state = { buffer: "" };
  while (!controller.signal.aborted) {
    const { done, value } = await reader.read();
    if (done) break;
    appendSSEChunk(state, decoder.decode(value, { stream: true }), (frame) =>
      handleSSEFrame(frame, onConnected, onRefresh),
    );
  }
}

function appendSSEChunk(state, chunk, handleFrame) {
  state.buffer += chunk.replaceAll("\r\n", "\n");
  let boundary;
  while ((boundary = state.buffer.indexOf("\n\n")) !== -1) {
    const frame = state.buffer.slice(0, boundary);
    state.buffer = state.buffer.slice(boundary + 2);
    handleFrame(frame);
  }
}

function handleSSEFrame(frame, onConnected, onRefresh) {
  const lines = frame.split("\n");
  const eventType = lines.find((line) => line.startsWith("event: "))?.slice(7);
  const data = lines.find((line) => line.startsWith("data: "))?.slice(6);
  if (eventType === "connected") return onConnected();
  if (eventType !== "dashboard_counts_changed" || !data) return;
  onRefresh(JSON.parse(data));
}

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function requiredEnv(name) {
  const value = process.env[name];
  if (!value) throw new Error(`${name} is required`);
  return value;
}

function csvEnv(name, expectedLength) {
  const values = requiredEnv(name)
    .split(",")
    .map((value) => value.trim())
    .filter(Boolean);
  if (values.length !== expectedLength) {
    throw new Error(
      `${name} must contain exactly ${expectedLength} comma-separated values`,
    );
  }
  return values;
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

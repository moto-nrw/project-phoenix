import { mapConcurrent } from "./concurrency";
import type { DeviceId } from "./devices";
import { BrokenShotError, brokenShotsError } from "./quality";
import { devicesFor, type Shot } from "./shot-list";

/** These views advance shared read cursors, including through GET requests. */
function changesReadState(shot: Shot): boolean {
  const path = shot.pfad.split("?")[0]!;
  return (
    /^\/(messages|team-chat)(\/|$)/.test(path) ||
    /^\/admin\/enrollments(\/|$)/.test(path) ||
    (shot.portal === "eltern" && /^\/news(\/|$)/.test(path))
  );
}

/**
 * Independent shots overlap. Communication and enrollment shots are barriers: their
 * read-marker changes happen in catalog order, never during another image.
 * Devices of one shot stay sequential, including after a broken first device.
 */
export async function captureInOrder<T>(
  shots: readonly Shot[],
  concurrency: number,
  capture: (shot: Shot, device: DeviceId) => Promise<T>,
  onBroken: (error: BrokenShotError) => void,
): Promise<T[]> {
  const broken: BrokenShotError[] = [];
  const results: T[] = [];
  async function captureShot(shot: Shot): Promise<T[]> {
    const devices: T[] = [];
    for (const device of devicesFor(shot)) {
      try {
        devices.push(await capture(shot, device));
      } catch (error) {
        if (!(error instanceof BrokenShotError)) throw error;
        broken.push(error);
        onBroken(error);
        break;
      }
    }
    return devices;
  }
  let independent: Shot[] = [];
  async function flush(): Promise<void> {
    const batch = independent;
    independent = [];
    results.push(
      ...(await mapConcurrent(batch, concurrency, captureShot)).flat(),
    );
  }
  for (const shot of shots) {
    if (changesReadState(shot)) {
      await flush();
      results.push(...(await captureShot(shot)));
    } else independent.push(shot);
  }
  await flush();
  if (broken.length > 0) {
    const positions = new Map(shots.map((shot, index) => [shot.id, index]));
    broken.sort((a, b) => positions.get(a.shotId)! - positions.get(b.shotId)!);
    throw brokenShotsError(broken);
  }
  return results;
}

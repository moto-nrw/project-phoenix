import { proxyPost } from "~/lib/route-wrapper.server";

// The backend's maxSchoolCheckinBatchBytes; checking it while reading keeps an
// oversized payload out of this process.
export const POST = proxyPost("/api/students/school-checkin/batch", {
  maxBodyBytes: 64 * 1024,
});

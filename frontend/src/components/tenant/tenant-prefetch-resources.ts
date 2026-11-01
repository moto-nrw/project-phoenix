import type { Session } from "next-auth";
import type { PrefetchResource } from "~/components/ui/data-prefetch-intent";
import {
  hasPermission,
  hasEffectiveAdminScope,
  canReadSupervisionData,
} from "~/lib/auth-utils";
import { canReadStudents } from "~/lib/student-access";
import { canReviewChangeRequests } from "~/lib/change-request-access";
import { groups } from "~/lib/swr/groups";
import { rooms } from "~/lib/swr/rooms";
import { studentKeys, homeLayoutKeys } from "~/lib/swr/portal-keys";
import { dashboardKeys } from "~/lib/swr/shell-keys";
import { fetchStudentSearch } from "~/lib/student-search-resource";
import { berlinTodayISO } from "~/lib/date-helpers";
import { studentService } from "~/lib/api";
import { fetchOgsGroupLive } from "~/lib/ogs-group-live-api";
import { courseWaitlistKeys } from "~/lib/swr/course-waitlist-keys";
import { fetchCourseWaitlists } from "~/lib/course-waitlist-api";
import { fetchHomeLayout } from "~/lib/home-layout-api";
import { fetchChangeRequestAccess } from "~/lib/change-requests-api";

/** Explicit opt-in destinations. Never a URL-to-API conversion or an audited read. */
export async function tenantPrefetchResources(
  href: string,
  context: {
    session: Session;
    slug: string;
    groupId: string | null;
    groups: readonly string[];
    hasSupervision: boolean;
    teacherMode: boolean;
    detailedMode: boolean;
    photoState: "photos-loading" | "photos-enabled" | "photos-disabled";
  },
): Promise<readonly PrefetchResource[]> {
  if (!href.startsWith("/") || href.startsWith("//")) return [];
  const [pathname, query] = href.split("?");
  const path = pathname?.startsWith(`/${context.slug}/`)
    ? pathname.slice(context.slug.length + 1)
    : pathname;
  const params = new URLSearchParams(query);
  const { session } = context;
  if (session.user.scope && session.user.scope !== "tenant") return [];

  if (path === "/students/search" && canReadStudents(session)) {
    const date = berlinTodayISO();
    // Only the unfiltered link is known here. Other filters still get shared
    // options, never a guessed main-list key. The page restores stored filters
    // after mount; its initial unfiltered reader consumes this same request.
    const main =
      params.size === 0
        ? [
            {
              key: studentKeys.search({
                groupIds: [],
                search: "",
                schoolClasses: [],
                roomId: "",
                dayStatus: "all",
                date,
                isToday: true,
                photoState: context.photoState,
                bus: "all",
                photoConsent: "all",
                companions: false,
                years: [],
                pickupStatus: "all",
              }),
              fetcher: () =>
                fetchStudentSearch(
                  {
                    search: "",
                    groupId: [],
                    schoolClass: [],
                    gradeLevel: [],
                    includePickupTimes: true,
                    includeArrivalTimes: true,
                    includeCompanions: false,
                    view: "slim",
                  },
                  date,
                  true,
                ),
            },
          ]
        : [];
    return [
      ...main,
      ...(hasPermission(session, "groups:read")
        ? [{ key: groups.key(), fetcher: groups.fetcher }]
        : []),
      ...(hasPermission(session, "rooms:read")
        ? [
            {
              key: rooms.key({ page: 1, pageSize: 1000 }),
              fetcher: () => rooms.fetcher({ page: 1, pageSize: 1000 }),
            },
          ]
        : []),
      {
        key: studentKeys.schoolClasses(),
        fetcher: () => studentService.getSchoolClasses(),
      },
    ];
  }
  if (path === "/ogs-groups" && canReadStudents(session)) {
    const id = params.get("group") ?? context.groupId;
    // Membership/supervision is confirmed by the shell, not inferred from URL.
    if (!id || !context.groups.includes(id)) return [];
    return [{ key: groups.live(id), fetcher: () => fetchOgsGroupLive(id) }];
  }
  if (
    path === "/active-supervisions" &&
    context.detailedMode &&
    canReadSupervisionData(session) &&
    context.hasSupervision &&
    !params.has("session") &&
    !params.has("room")
  ) {
    const { fetchSupervisionDashboard } =
      await import("~/components/active-supervisions/supervision-dashboard-resource");
    return [
      {
        key: dashboardKeys.supervision(session.user.id),
        fetcher: () => fetchSupervisionDashboard(null),
      },
    ];
  }
  const student = /^\/students\/([1-9]\d*)$/.exec(path ?? "");
  if (student?.[1] && canReadStudents(session)) {
    const id = student[1];
    const { fetchStudentDetail } =
      await import("~/lib/student-detail-resource");
    return [
      { key: studentKeys.detail(id), fetcher: () => fetchStudentDetail(id) },
    ];
  }
  if (path === "/home")
    return [{ key: homeLayoutKeys.key(), fetcher: fetchHomeLayout }];
  if (path === "/anfragen" && canReviewChangeRequests(session)) {
    if (hasEffectiveAdminScope(session)) {
      return [
        {
          key: courseWaitlistKeys.all(),
          fetcher: () => fetchCourseWaitlists(),
        },
      ];
    }
    // The target checks this access before opening its parent queues.
    if (context.teacherMode)
      return [
        {
          key: studentKeys.requestAccess(session.user.id),
          fetcher: fetchChangeRequestAccess,
        },
      ];
  }
  return [];
}

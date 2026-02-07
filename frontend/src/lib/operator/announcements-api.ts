import { operatorFetch } from "./api-helpers";
import type {
  BackendAnnouncement,
  Announcement,
  AnnouncementStats,
  AnnouncementViewDetail,
  BackendAnnouncementViewDetail,
  CreateAnnouncementRequest,
  UpdateAnnouncementRequest,
} from "./announcements-helpers";
import { mapAnnouncement, mapViewDetail } from "./announcements-helpers";

class OperatorAnnouncementsService {
  async fetchAll(includeInactive = true): Promise<Announcement[]> {
    const data = await operatorFetch<BackendAnnouncement[]>(
      `/api/operator/announcements?include_inactive=${includeInactive}`,
    );
    return data.map(mapAnnouncement);
  }

  async fetchById(id: string): Promise<Announcement> {
    const data = await operatorFetch<BackendAnnouncement>(
      `/api/operator/announcements/${id}`,
    );
    return mapAnnouncement(data);
  }

  async create(data: CreateAnnouncementRequest): Promise<Announcement> {
    const result = await operatorFetch<BackendAnnouncement>(
      "/api/operator/announcements",
      { method: "POST", body: data },
    );
    return mapAnnouncement(result);
  }

  async update(
    id: string,
    data: UpdateAnnouncementRequest,
  ): Promise<Announcement> {
    const result = await operatorFetch<BackendAnnouncement>(
      `/api/operator/announcements/${id}`,
      { method: "PUT", body: data },
    );
    return mapAnnouncement(result);
  }

  async delete(id: string): Promise<void> {
    await operatorFetch(`/api/operator/announcements/${id}`, {
      method: "DELETE",
    });
  }

  async publish(id: string): Promise<void> {
    await operatorFetch(`/api/operator/announcements/${id}/publish`, {
      method: "POST",
    });
  }

  async fetchStats(id: string): Promise<AnnouncementStats> {
    return operatorFetch<AnnouncementStats>(
      `/api/operator/announcements/${id}/stats`,
    );
  }

  async fetchViewDetails(id: string): Promise<AnnouncementViewDetail[]> {
    const data = await operatorFetch<BackendAnnouncementViewDetail[]>(
      `/api/operator/announcements/${id}/views`,
    );
    return data.map(mapViewDetail);
  }
}

export const operatorAnnouncementsService = new OperatorAnnouncementsService();

package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
)

// ListFolders returns the folders the actor may see plus what the actor may
// do, so the UI never has to guess authority.
func (s *Service) ListFolders(ctx context.Context, actor domain.Actor) (overview domain.Overview, err error) {
	err = s.run("list_folders", func(o *op) error {
		overview = domain.Overview{Folders: []domain.FolderView{}, CanManage: actor.Manager}
		viewer, active, err := s.viewer(ctx, actor)
		if err != nil {
			return err
		}
		var items []domain.FolderListItem
		if active {
			var stats domain.OperationStats
			if items, stats, err = s.deps.Folders.ListVisible(ctx, viewer); err != nil {
				return err
			}
			o.add(stats)
		}
		if overview.StaffUploadEnabled, err = s.deps.Settings.StaffUploadEnabled(ctx); err != nil {
			return err
		}
		overview.CanUpload = overview.CanManage || overview.StaffUploadEnabled

		var audiences map[int64]domain.Audience
		if overview.CanManage {
			ids := make([]int64, 0, len(items))
			for _, item := range items {
				ids = append(ids, item.ID)
			}
			var stats domain.OperationStats
			if audiences, stats, err = s.deps.Folders.GetAudience(ctx, ids); err != nil {
				return err
			}
			o.add(stats)
			if overview.UsedBytes, stats, err = s.deps.Files.TotalStoredBytes(ctx); err != nil {
				return err
			}
			o.add(stats)
			if overview.MaxBytes, err = s.deps.Settings.MaxStorageBytes(ctx); err != nil {
				return err
			}
		}
		for _, item := range items {
			overview.Folders = append(overview.Folders, domain.FolderView{FolderListItem: item, Audience: audiences[item.ID]})
		}
		return nil
	})
	return overview, err
}

// CreateFolder creates a folder with its share list and records the event.
func (s *Service) CreateFolder(ctx context.Context, input domain.FolderInput, actor domain.Actor) (view domain.FolderView, err error) {
	err = s.run("create_folder", func(o *op) error {
		if err := requireManager(actor); err != nil {
			return err
		}
		if err := requireActor(actor); err != nil {
			return err
		}
		if err := input.Normalize(); err != nil {
			return err
		}
		audience := input.Audience()
		return s.write(ctx, o, func(txCtx context.Context, o *op) error {
			if err := s.validateAudience(txCtx, audience); err != nil {
				return err
			}
			folder, stats, err := s.deps.Folders.Create(txCtx, domain.Folder{Name: input.Name, Visibility: input.Visibility, CreatedBy: actor.AccountID})
			o.add(stats)
			if err != nil {
				return err
			}
			stats, err = s.deps.Folders.ReplaceAudience(txCtx, folder.ID, audience)
			o.add(stats)
			if err != nil {
				return err
			}
			view = domain.FolderView{FolderListItem: domain.FolderListItem{Folder: folder}, Audience: audience}
			return s.record(txCtx, actor, ports.EventFolderCreated, &folder.ID, nil, nil, folderEventDetail(folder, audience))
		})
	})
	return view, err
}

// UpdateFolder rewrites name, visibility and share list of a folder.
func (s *Service) UpdateFolder(ctx context.Context, folderID int64, input domain.FolderInput, actor domain.Actor) (view domain.FolderView, err error) {
	err = s.run("update_folder", func(o *op) error {
		if err := requireManager(actor); err != nil {
			return err
		}
		if err := requireActor(actor); err != nil {
			return err
		}
		if err := input.Normalize(); err != nil {
			return err
		}
		audience := input.Audience()
		return s.write(ctx, o, func(txCtx context.Context, o *op) error {
			existing, found, stats, err := s.deps.Folders.FindByID(txCtx, folderID)
			o.add(stats)
			if err != nil {
				return err
			}
			if !found {
				return domain.ErrNotFound
			}
			if err := s.validateAudience(txCtx, audience); err != nil {
				return err
			}
			existing.Name = input.Name
			existing.Visibility = input.Visibility
			folder, stats, err := s.deps.Folders.Update(txCtx, existing)
			o.add(stats)
			if err != nil {
				return err
			}
			stats, err = s.deps.Folders.ReplaceAudience(txCtx, folder.ID, audience)
			o.add(stats)
			if err != nil {
				return err
			}
			view = domain.FolderView{FolderListItem: domain.FolderListItem{Folder: folder}, Audience: audience}
			return s.record(txCtx, actor, ports.EventFolderUpdated, &folder.ID, nil, nil, folderEventDetail(folder, audience))
		})
	})
	return view, err
}

// DeleteFolder queues cleanup for every file still on disk and removes the
// folder; the file rows cascade. Bytes are removed by the sweep.
func (s *Service) DeleteFolder(ctx context.Context, folderID int64, actor domain.Actor) error {
	return s.run("delete_folder", func(o *op) error {
		if err := requireManager(actor); err != nil {
			return err
		}
		if err := requireActor(actor); err != nil {
			return err
		}
		return s.write(ctx, o, func(txCtx context.Context, o *op) error {
			folder, found, stats, err := s.deps.Folders.FindByID(txCtx, folderID)
			o.add(stats)
			if err != nil {
				return err
			}
			if !found {
				return domain.ErrNotFound
			}
			// Every object still on disk gets an immediately eligible intent
			// BEFORE the rows cascade away; the intents are all that survive.
			pending, stats, err := s.deps.Files.ListPendingCleanupByOwner(txCtx, folderID)
			o.add(stats)
			if err != nil {
				return err
			}
			now := s.deps.Now()
			for _, file := range pending {
				stats, err := s.deps.FileCleanups.Queue(txCtx, folderID, file.FilenameStored, now)
				o.add(stats)
				if err != nil {
					return err
				}
			}
			stats, err = s.deps.Folders.Delete(txCtx, folderID)
			o.add(stats)
			if err != nil {
				return err
			}
			s.deps.Logger.Info("folder deleted",
				"folder_id", folderID,
				"files_pending_cleanup", len(pending))
			return s.record(txCtx, actor, ports.EventFolderDeleted, &folderID, nil, nil,
				fmt.Sprintf("Ordner „%s“ gelöscht (%d Dateien)", folder.Name, len(pending)))
		})
	})
}

// ListAudienceOptions returns the roles and persons a folder can be shared with.
func (s *Service) ListAudienceOptions(ctx context.Context, actor domain.Actor) (roles []domain.AudienceRole, accounts []domain.AudienceAccount, err error) {
	err = s.run("list_audience_options", func(*op) error {
		if err := requireManager(actor); err != nil {
			return err
		}
		if roles, err = s.audienceRoles(ctx); err != nil {
			return err
		}
		accounts, err = s.audienceAccounts(ctx)
		return err
	})
	return roles, accounts, err
}

// audienceRoles lists the shareable roles by name. The guardian tier is
// dropped because parents never reach the tenant portal.
func (s *Service) audienceRoles(ctx context.Context) ([]domain.AudienceRole, error) {
	roles, err := s.deps.Identity.ListShareableRoles(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(roles, func(left, right domain.AudienceRole) int {
		return strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
	})
	return roles, nil
}

// audienceAccounts keeps the picker on accounts that are backed by a person of
// the school and orders them by name, so it shows names rather than e-mail
// addresses.
func (s *Service) audienceAccounts(ctx context.Context) ([]domain.AudienceAccount, error) {
	ids, err := s.deps.Identity.ListActiveAccountIDs(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]domain.AudienceAccount, 0, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	names, err := s.deps.People.PersonNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		name, found := names[id]
		if !found {
			continue
		}
		result = append(result, domain.AudienceAccount{AccountID: id, FirstName: name.FirstName, LastName: name.LastName})
	}
	slices.SortStableFunc(result, func(left, right domain.AudienceAccount) int {
		if order := strings.Compare(strings.ToLower(left.LastName), strings.ToLower(right.LastName)); order != 0 {
			return order
		}
		return strings.Compare(strings.ToLower(left.FirstName), strings.ToLower(right.FirstName))
	})
	return result, nil
}

// validateAudience refuses roles and accounts that are not shareable at this
// school, so a share list can never point outside the tenant.
func (s *Service) validateAudience(ctx context.Context, audience domain.Audience) error {
	if len(audience.RoleIDs) > 0 {
		roles, err := s.audienceRoles(ctx)
		if err != nil {
			return err
		}
		allowed := make(map[int64]struct{}, len(roles))
		for _, role := range roles {
			allowed[role.ID] = struct{}{}
		}
		for _, id := range audience.RoleIDs {
			if _, ok := allowed[id]; !ok {
				return fmt.Errorf("%w: unknown role", domain.ErrInvalid)
			}
		}
	}
	if len(audience.AccountIDs) > 0 {
		accounts, err := s.audienceAccounts(ctx)
		if err != nil {
			return err
		}
		allowed := make(map[int64]struct{}, len(accounts))
		for _, account := range accounts {
			allowed[account.AccountID] = struct{}{}
		}
		for _, id := range audience.AccountIDs {
			if _, ok := allowed[id]; !ok {
				return fmt.Errorf("%w: unknown person", domain.ErrInvalid)
			}
		}
	}
	return nil
}

func folderEventDetail(folder domain.Folder, audience domain.Audience) string {
	switch folder.Visibility {
	case domain.VisibilitySelected:
		return fmt.Sprintf("Ordner „%s“: Sichtbar für %d Rollen und %d Personen", folder.Name, len(audience.RoleIDs), len(audience.AccountIDs))
	case domain.VisibilityAdmins:
		return fmt.Sprintf("Ordner „%s“: Nur Leitung", folder.Name)
	default:
		return fmt.Sprintf("Ordner „%s“: Alle Mitarbeitenden", folder.Name)
	}
}

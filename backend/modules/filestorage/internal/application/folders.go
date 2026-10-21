package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
)

// ListFolders returns the folders the actor may see plus what the actor may
// do, so the UI never has to guess authority.
func (s *Service) ListFolders(ctx context.Context, who filestorage.Actor) (filestorage.FolderOverview, error) {
	actor := s.actor(who)
	var overview domain.Overview
	err := s.runRead(ctx, "list_folders", func(ctx context.Context, o *op) error {
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

		audiences, err := s.folderManagementOverview(ctx, o, &overview, items)
		if err != nil {
			return err
		}
		for _, item := range items {
			overview.Folders = append(overview.Folders, domain.FolderView{FolderListItem: item, Audience: audiences[item.ID]})
		}
		return nil
	})
	if err != nil {
		return filestorage.FolderOverview{}, err
	}
	return toFolderOverview(overview), nil
}

// CreateFolder creates a folder with its share list and records the event.
func (s *Service) CreateFolder(ctx context.Context, value filestorage.FolderInput, who filestorage.Actor) (filestorage.Folder, error) {
	input, actor := value, s.actor(who)
	var view domain.FolderView
	err := s.run("create_folder", func(o *op) error {
		if err := requireManager(actor); err != nil {
			return err
		}
		if err := requireActor(actor); err != nil {
			return err
		}
		if err := normalizeFolderInput(&input); err != nil {
			return err
		}
		audience := domain.Audience{RoleIDs: input.RoleIDs, AccountIDs: input.AccountIDs}
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
	return toFolder(view), err
}

// UpdateFolder rewrites name, visibility and share list of a folder.
func (s *Service) UpdateFolder(ctx context.Context, folderID int64, value filestorage.FolderInput, who filestorage.Actor) (filestorage.Folder, error) {
	input, actor := value, s.actor(who)
	var view domain.FolderView
	err := s.run("update_folder", func(o *op) error {
		if err := requireManager(actor); err != nil {
			return err
		}
		if err := requireActor(actor); err != nil {
			return err
		}
		if err := normalizeFolderInput(&input); err != nil {
			return err
		}
		audience := domain.Audience{RoleIDs: input.RoleIDs, AccountIDs: input.AccountIDs}
		return s.write(ctx, o, func(txCtx context.Context, o *op) error {
			var updateErr error
			view, updateErr = s.updateFolderRecord(txCtx, o, folderID, input, actor, audience)
			return updateErr
		})
	})
	return toFolder(view), err
}

// DeleteFolder queues cleanup for every file still on disk and removes the
// folder; the file rows cascade. Bytes are removed by the sweep.
func (s *Service) DeleteFolder(ctx context.Context, folderID int64, who filestorage.Actor) error {
	actor := s.actor(who)
	return s.run("delete_folder", func(o *op) error {
		if err := requireManager(actor); err != nil {
			return err
		}
		if err := requireActor(actor); err != nil {
			return err
		}
		return s.write(ctx, o, func(txCtx context.Context, o *op) error {
			return s.deleteFolderRecord(txCtx, o, folderID, actor)
		})
	})
}

// ListAudienceOptions returns the roles and persons a folder can be shared with.
func (s *Service) ListAudienceOptions(ctx context.Context, who filestorage.Actor) (filestorage.AudienceOptions, error) {
	actor := s.actor(who)
	var roles []filestorage.AudienceRole
	var accounts []filestorage.AudienceAccount
	err := s.runRead(ctx, "list_audience_options", func(ctx context.Context, _ *op) error {
		if err := requireManager(actor); err != nil {
			return err
		}
		var err error
		if roles, err = s.audienceRoles(ctx); err != nil {
			return err
		}
		accounts, err = s.audienceAccounts(ctx)
		return err
	})
	if err != nil {
		return filestorage.AudienceOptions{}, err
	}
	return filestorage.AudienceOptions{
		Roles:    append(make([]filestorage.AudienceRole, 0, len(roles)), roles...),
		Accounts: append(make([]filestorage.AudienceAccount, 0, len(accounts)), accounts...),
	}, nil
}

// normalizeFolderInput trims and validates the input and drops the share
// lists when the visibility does not use them.
func normalizeFolderInput(input *filestorage.FolderInput) error {
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return fmt.Errorf("%w: name is required", filestorage.ErrInvalid)
	}
	if len([]rune(input.Name)) > domain.MaxFolderNameLength {
		return fmt.Errorf("%w: name is too long", filestorage.ErrInvalid)
	}
	if !domain.IsValidVisibility(input.Visibility) {
		return fmt.Errorf("%w: unknown visibility", filestorage.ErrInvalid)
	}
	input.RoleIDs = domain.DedupeIDs(input.RoleIDs)
	input.AccountIDs = domain.DedupeIDs(input.AccountIDs)
	if input.Visibility != domain.VisibilitySelected {
		input.RoleIDs, input.AccountIDs = nil, nil
	} else if len(input.RoleIDs) == 0 && len(input.AccountIDs) == 0 {
		return fmt.Errorf("%w: a selected folder needs at least one role or person", filestorage.ErrInvalid)
	}
	return nil
}

// audienceRoles lists the shareable roles by name. The guardian tier is
// dropped because parents never reach the tenant portal.
func (s *Service) audienceRoles(ctx context.Context) ([]filestorage.AudienceRole, error) {
	roles, err := s.deps.Identity.ListShareableRoles(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(roles, func(left, right filestorage.AudienceRole) int {
		return strings.Compare(strings.ToLower(left.Name), strings.ToLower(right.Name))
	})
	return roles, nil
}

// audienceAccounts keeps the picker on accounts that are backed by a person of
// the school and orders them by name, so it shows names rather than e-mail
// addresses.
func (s *Service) audienceAccounts(ctx context.Context) ([]filestorage.AudienceAccount, error) {
	ids, err := s.deps.Identity.ListActiveSchoolAccountIDs(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]filestorage.AudienceAccount, 0, len(ids))
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
		result = append(result, filestorage.AudienceAccount{AccountID: id, FirstName: name.FirstName, LastName: name.LastName})
	}
	slices.SortStableFunc(result, func(left, right filestorage.AudienceAccount) int {
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
	if err := s.validateAudienceRoles(ctx, audience.RoleIDs); err != nil {
		return err
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
				return fmt.Errorf("%w: unknown person", filestorage.ErrInvalid)
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

func (s *Service) validateAudienceRoles(ctx context.Context, roleIDs []int64) error {
	if len(roleIDs) > 0 {
		roles, err := s.audienceRoles(ctx)
		if err != nil {
			return err
		}
		allowed := make(map[int64]struct{}, len(roles))
		for _, role := range roles {
			allowed[role.ID] = struct{}{}
		}
		for _, id := range roleIDs {
			if _, ok := allowed[id]; !ok {
				return fmt.Errorf("%w: unknown role", filestorage.ErrInvalid)
			}
		}
	}
	return nil
}

func (s *Service) folderManagementOverview(ctx context.Context, o *op, overview *domain.Overview, items []domain.FolderListItem) (map[int64]domain.Audience, error) {
	var audiences map[int64]domain.Audience
	var err error
	if overview.CanManage {
		ids := make([]int64, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		var stats domain.OperationStats
		if audiences, stats, err = s.deps.Folders.GetAudience(ctx, ids); err != nil {
			return nil, err
		}
		o.add(stats)
		if overview.UsedBytes, stats, err = s.deps.Files.TotalStoredBytes(ctx); err != nil {
			return nil, err
		}
		o.add(stats)
		if overview.MaxBytes, err = s.deps.Settings.MaxStorageBytes(ctx); err != nil {
			return nil, err
		}
	}
	return audiences, nil
}

func (s *Service) deleteFolderRecord(txCtx context.Context, o *op, folderID int64, actor domain.Actor) error {
	folder, found, stats, err := s.deps.Folders.FindByID(txCtx, folderID)
	o.add(stats)
	if err != nil {
		return err
	}
	if !found {
		return filestorage.ErrNotFound
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
}

func (s *Service) updateFolderRecord(txCtx context.Context, o *op, folderID int64, input filestorage.FolderInput, actor domain.Actor, audience domain.Audience) (view domain.FolderView, err error) {
	existing, found, stats, err := s.deps.Folders.FindByID(txCtx, folderID)
	o.add(stats)
	if err != nil {
		return view, err
	}
	if !found {
		return view, filestorage.ErrNotFound
	}
	if err := s.validateAudience(txCtx, audience); err != nil {
		return view, err
	}
	existing.Name = input.Name
	existing.Visibility = input.Visibility
	folder, stats, err := s.deps.Folders.Update(txCtx, existing)
	o.add(stats)
	if err != nil {
		return view, err
	}
	stats, err = s.deps.Folders.ReplaceAudience(txCtx, folder.ID, audience)
	o.add(stats)
	if err != nil {
		return view, err
	}
	view = domain.FolderView{FolderListItem: domain.FolderListItem{Folder: folder}, Audience: audience}
	return view, s.record(txCtx, actor, ports.EventFolderUpdated, &folder.ID, nil, nil, folderEventDetail(folder, audience))
}

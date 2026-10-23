package students

import (
	slog "log/slog"

	chi "github.com/go-chi/chi/v5"
	render "github.com/go-chi/render"
	common "github.com/moto-nrw/project-phoenix/api/common"
	permissions "github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	collation "github.com/moto-nrw/project-phoenix/sharedkernel/collation"
)

type ResourceConfig struct {
	ActiveEnrollments      ActiveEnrollments
	AuthenticatedDevice    DeviceIdentity
	ChildQuota             ChildQuotaReader
	ClassListEntries       ClassListEntryReader
	DeviceAuthenticator    common.Middleware
	FamilyProtection       FamilyProtectionCapability
	Logger                 *slog.Logger
	PeopleDirectory        peopleModule.Capability
	Persons                PersonRecords
	StudentAuditService    StudentChangeHistory
	StudentDocumentService StudentDocumentHistoryAccess
	StudentNotes           StudentNotesCapability
	StudentPhotos          StudentPhotoLifecycle
	Uploads                common.UploadStorage
	UserContextService     CallerContext
}

type Resource struct{ ResourceConfig }

func NewResource(cfg ResourceConfig) *Resource { return &Resource{ResourceConfig: cfg} }
func (rs *Resource) Router(protected func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))
	protected(r, rs.Register)
	r.Group(rs.mountDeviceRoutes)
	return r
}
func (rs *Resource) Register(r chi.Router, withTx common.Middleware) {
	// The school classes and the photo also open for students:read, the
	// view-only child access of the role guest (ADR 0088); notes and the
	// change history stay behind users:read.
	childRead := common.RequiresAnyPermission(permissions.UsersRead, permissions.StudentsRead)
	r.With(common.RequiresPermission(permissions.UsersDelete), withTx).Get("/child-quota", rs.getChildQuota)
	r.With(childRead, withTx).Get("/school-classes", rs.listSchoolClasses)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/change-history", rs.getStudentChangeHistory)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/notes", rs.listStudentNotes)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Get("/{id}/note-subjects", rs.listStudentNoteSubjects)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Post("/{id}/notes", rs.createStudentNote)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Put("/{id}/notes/{noteId}", rs.updateStudentNote)
	r.With(common.RequiresPermission(permissions.UsersRead), withTx).Delete("/{id}/notes/{noteId}", rs.deleteStudentNote)
	r.With(common.RequiresPermission(permissions.ConfigManage), withTx).Get("/{id}/family-protection", rs.getFamilyProtection)
	r.With(common.RequiresPermission(permissions.ConfigManage), withTx).Put("/{id}/family-protection", rs.setFamilyProtection)
	r.With(common.RequiresPermission(permissions.UsersUpdate)).Post("/{id}/photo", rs.uploadStudentPhoto)
	r.With(common.RequiresPermission(permissions.UsersUpdate), withTx).Delete("/{id}/photo", rs.deleteStudentPhoto)
	r.With(childRead).Get("/{id}/photo/{filename}", rs.serveStudentPhoto)
}

func (rs *Resource) mountDeviceRoutes(r chi.Router) {
	r.Use(rs.requiredDeviceAuthenticator())
	r.Use(common.TenantTxMiddleware)

	// RFID tag assignment endpoint
	r.Post("/{id}/rfid", rs.assignRFIDTag)
	r.Delete("/{id}/rfid", rs.unassignRFIDTag)
}

func CompareSchoolClasses(a, b string) int { return collation.CompareSchoolClasses(a, b) }
func CompareGermanNames(al, af, bl, bf string) int {
	return collation.CompareGermanNames(al, af, bl, bf)
}

// StudentDocumentHistoryAccess applies the document owner's category gates to audit reads.
type StudentDocumentHistoryAccess interface {
	CanSeeEveryStudentDocumentCategory([]string) bool
	CanSeeStudentDocumentCategory(string, []string) bool
}

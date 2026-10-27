package announcement

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	announcementService "github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// Class letters (#4800): a Lehrkraft writes parent letters to the classes
// she reaches from moto schule, mounted at /school/parent-letters. The
// letter is the same parent announcement the OGS writes, with the same
// handlers behind a narrower mantle: one class target in the caller's reach,
// only the caller's own letters, and the OGS's mutating routes refuse them.

// ClassLetterCaller is what the class-letter routes read of the caller: the
// classes she reaches (education.class_teachers, every class with
// class_day:all_classes) and the name parents read as the sender.
type ClassLetterCaller interface {
	MySchoolClasses(ctx context.Context) ([]string, error)
	SenderName(ctx context.Context) (string, error)
}

var (
	errClassLetterTarget      = errors.New("a class letter goes to exactly one class")
	errClassLetterUnreachable = errors.New("the class is not assigned to the caller")
	errClassLetterCaller      = errors.New("class letters are not configured")
)

// SchoolRouter returns the class-letter surface for school-scope tokens.
func (rs *Resource) SchoolRouter(protectedSchoolRoutes func(chi.Router, func(chi.Router, common.Middleware))) chi.Router {
	r := chi.NewRouter()
	protectedSchoolRoutes(r, func(r chi.Router, withTx common.Middleware) {
		// Every request writes as the Lehrkraft: the service then changes
		// only her own class letters and none of the OGS.
		r = r.With(classLetterAuthor)
		write := common.RequiresPermission(permissions.ClassDayParentLetterWrite)
		r.With(write, withTx).Get("/", rs.listClassLetters)
		r.With(write, withTx).Post("/", rs.createClassLetter)

		// Reading, withdrawing and deleting the own letter needs no class
		// reach: the letter stays the author's after the class moved on.
		own := r.With(write, withTx, rs.ownClassLetter(false))
		own.Get("/{announcementId}", rs.get)
		own.Delete("/{announcementId}", rs.delete)
		own.Post("/{announcementId}/unpublish", rs.unpublish)
		own.Get("/{announcementId}/stats", rs.stats)
		own.Get("/{announcementId}/recipients", rs.recipients)
		own.Get("/{announcementId}/letter-status", rs.letterStatus)
		own.Get("/{announcementId}/declaration-status", rs.declarationStatus)
		own.Get("/{announcementId}/declaration-export", rs.declarationExport)

		// Writing to parents again needs the class still in reach.
		reach := r.With(write, withTx, rs.ownClassLetter(true))
		reach.Post("/{announcementId}/publish", rs.publish)
		reach.Post("/{announcementId}/remind", rs.remindUnanswered)
		reach.Post("/{announcementId}/resend-failed", rs.resendFailed)
	})
	return r
}

// listClassLetters narrows the school's list to the caller's own class
// letters, drafts included: one query plan with the OGS list.
func (rs *Resource) listClassLetters(w http.ResponseWriter, r *http.Request) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	rows, err := rs.Service.ListParentAnnouncements(r.Context(), true)
	if err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	out := make([]announcementResponse, 0)
	for _, a := range rows {
		if a.IsClassLetter() && a.CreatedBy == int64(claims.ID) {
			out = append(out, toAnnouncementResponse(&a))
		}
	}
	common.Respond(w, r, http.StatusOK, out, "Class letters retrieved")
}

func (rs *Resource) createClassLetter(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeInput(w, r)
	if !ok {
		return
	}
	sender, err := rs.classLetterSender(r.Context(), in.Targets)
	if err != nil {
		renderClassLetterError(w, r, err)
		return
	}
	in.Sender = sender
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	a, err := rs.Service.CreateParentAnnouncement(r.Context(), int64(claims.ID), in)
	if err != nil {
		renderAnnouncementError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, toAnnouncementResponse(a), "Class letter created")
}

// classLetterSender resolves the one class target against the caller's
// reach and names the sender. The class is taken as the school spells it.
func (rs *Resource) classLetterSender(ctx context.Context, targets []announcementService.ParentAnnouncementTargetInput) (*announcementService.ParentLetterSender, error) {
	if len(targets) != 1 || targets[0].TargetType != "class" || targets[0].RefText == nil {
		return nil, errClassLetterTarget
	}
	class, err := rs.reachedClass(ctx, *targets[0].RefText)
	if err != nil {
		return nil, err
	}
	name, err := rs.Classes.SenderName(ctx)
	if err != nil {
		return nil, err
	}
	return &announcementService.ParentLetterSender{Name: name, SchoolClass: class}, nil
}

func (rs *Resource) reachedClass(ctx context.Context, class string) (string, error) {
	if rs.Classes == nil {
		return "", errClassLetterCaller
	}
	classes, err := rs.Classes.MySchoolClasses(ctx)
	if err != nil {
		return "", err
	}
	wanted := strings.TrimSpace(class)
	for _, reached := range classes {
		if strings.EqualFold(strings.TrimSpace(reached), wanted) {
			return reached, nil
		}
	}
	return "", errClassLetterUnreachable
}

// ownClassLetter admits the caller's own class letters only; any other
// announcement answers 404, so the route reveals nothing of the OGS's.
// withReach additionally requires the letter's class in the caller's reach.
func (rs *Resource) ownClassLetter(withReach bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, ok := parseAnnouncementID(w, r)
			if !ok {
				return
			}
			if err := rs.checkOwnClassLetter(r, id, withReach); err != nil {
				renderClassLetterError(w, r, err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (rs *Resource) checkOwnClassLetter(r *http.Request, id int64, withReach bool) error {
	a, err := rs.Service.GetParentAnnouncement(r.Context(), id)
	if err != nil {
		return err
	}
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if !a.IsClassLetter() || a.CreatedBy != int64(claims.ID) {
		return announcementService.ErrParentAnnouncementNotFound
	}
	if withReach {
		_, err = rs.reachedClass(r.Context(), a.Sender.SchoolClass)
	}
	return err
}

// classLetterAuthor marks the request as written by the Lehrkraft.
func classLetterAuthor(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := sessiontoken.ClaimsFromCtx(r.Context())
		ctx := announcementService.WithClassLetterAuthor(r.Context(), int64(claims.ID))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func renderClassLetterError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, errClassLetterTarget):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, errClassLetterUnreachable):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, common.CodeCommunicationClassLetterClassNotReachable))
	default:
		renderAnnouncementError(w, r, err)
	}
}

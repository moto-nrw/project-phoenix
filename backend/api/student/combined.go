package student

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/logging"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"github.com/sirupsen/logrus"
)

// Define the additional interface needed for custom user operations
type CustomUserStore interface {
	CreateCustomUser(ctx context.Context, user *models2.CustomUser) error
}

// CombinedStudentRequest represents the request payload for creating a student with a new user
type CombinedStudentRequest struct {
	FirstName   string `json:"first_name"`
	SecondName  string `json:"second_name"`
	SchoolClass string `json:"school_class"`
	NameLG      string `json:"name_lg"`
	ContactLG   string `json:"contact_lg"`
	GroupID     int64  `json:"group_id"`
	InHouse     bool   `json:"in_house"`
	WC          bool   `json:"wc"`
	SchoolYard  bool   `json:"school_yard"`
	Bus         bool   `json:"bus"`
}

// Bind preprocesses a CombinedStudentRequest
func (csr *CombinedStudentRequest) Bind(r *http.Request) error {
	// Basic validation
	if csr.FirstName == "" {
		return errors.New("first_name is required")
	}
	if csr.SecondName == "" {
		return errors.New("second_name is required")
	}
	if csr.SchoolClass == "" {
		return errors.New("school_class is required")
	}
	if csr.GroupID == 0 {
		return errors.New("group_id is required")
	}

	return nil
}

// CreateStudentWithUser creates a new CustomUser and Student in one transaction
func (rs *Resource) CreateStudentWithUser(w http.ResponseWriter, r *http.Request) {
	logger := logging.GetLogEntry(r)
	ctx := r.Context()

	// Parse and validate the request
	data := &CombinedStudentRequest{}
	if err := render.Bind(r, data); err != nil {
		logger.WithError(err).Warn("Invalid combined student creation request")
		render.Render(w, r, ErrInvalidRequest(err))
		return
	}

	// Check if we have access to the user store for creating users
	customUserStore, ok := rs.UserStore.(CustomUserStore)
	if !ok {
		logger.Error("UserStore does not implement CustomUserStore interface")
		render.Render(w, r, ErrInternalServerError(errors.New("user creation not supported")))
		return
	}

	// Create new CustomUser
	customUser := &models2.CustomUser{
		FirstName:  data.FirstName,
		SecondName: data.SecondName,
	}

	if err := customUserStore.CreateCustomUser(ctx, customUser); err != nil {
		logger.WithError(err).Error("Failed to create CustomUser")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithField("custom_users_id", customUser.ID).Info("CustomUser created successfully")

	// Create Student with the newly created CustomUser ID
	student := &models2.Student{
		SchoolClass:  data.SchoolClass,
		NameLG:       data.NameLG,
		ContactLG:    data.ContactLG,
		GroupID:      data.GroupID,
		InHouse:      data.InHouse,
		WC:           data.WC,
		SchoolYard:   data.SchoolYard,
		Bus:          data.Bus,
		CustomUserID: customUser.ID, // Link to the newly created user - field name matches model struct field
	}

	if err := rs.Store.CreateStudent(ctx, student); err != nil {
		logger.WithError(err).Error("Failed to create student")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	// Get the complete student record with all relationships
	completeStudent, err := rs.Store.GetStudentByID(ctx, student.ID)
	if err != nil {
		logger.WithError(err).Error("Failed to retrieve newly created student")
		render.Render(w, r, ErrInternalServerError(err))
		return
	}

	logger.WithFields(logrus.Fields{
		"student_id":      student.ID,
		"custom_users_id": customUser.ID,
		"name":            customUser.FirstName + " " + customUser.SecondName,
	}).Info("Student with user created successfully")

	render.Status(r, http.StatusCreated)
	render.JSON(w, r, completeStudent)
}

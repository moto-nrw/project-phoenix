package test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// FindProjectRoot walks up the directory tree from the current working directory
// until it finds a directory containing go.mod. Returns the parent of that directory
// (the actual project root where .env lives).
//
// This approach is self-healing: it works regardless of how deep the test file is
// in the directory structure, eliminating fragile "../.." path counting.
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check if go.mod exists in this directory
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// Found backend/go.mod, return parent (project-phoenix/)
			return filepath.Dir(dir), nil
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding go.mod
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// LoadTestEnv loads the .env file from the project root.
// This is the standard way to configure test database connections.
//
// Usage in test files:
//
//	func setupTestDB(t *testing.T) *bun.DB {
//	    testpkg.LoadTestEnv(t)
//	    // ... rest of setup
//	}
func LoadTestEnv(t *testing.T) {
	t.Helper()

	projectRoot, err := FindProjectRoot()
	if err != nil {
		t.Logf("Warning: Could not find project root: %v", err)
		return
	}

	envPath := filepath.Join(projectRoot, ".env")
	if err := godotenv.Load(envPath); err != nil {
		t.Logf("Warning: Could not load %s: %v", envPath, err)
	}
}

// SetupTestDB creates a test database connection using the standard configuration.
// It automatically loads .env from project root and configures the database DSN.
//
// This is the preferred way to get a database connection in tests:
//
//	func TestSomething(t *testing.T) {
//	    db := testpkg.SetupTestDB(t)
//	    defer db.Close()
//	    // ... test code
//	}
func SetupTestDB(t *testing.T) *bun.DB {
	t.Helper()

	// Load .env from project root
	LoadTestEnv(t)

	// Initialize viper to read environment variables
	viper.AutomaticEnv()

	// Try to get DSN from environment (order: TEST_DB_DSN, test_db_dsn, DB_DSN, db_dsn)
	testDSN := os.Getenv("TEST_DB_DSN")
	if testDSN == "" {
		testDSN = viper.GetString("test_db_dsn")
	}
	if testDSN == "" {
		testDSN = os.Getenv("DB_DSN")
	}
	if testDSN == "" {
		testDSN = viper.GetString("db_dsn")
	}
	if testDSN == "" {
		t.Skip("No test database configured (set TEST_DB_DSN or DB_DSN)")
	}

	// Set the DSN in viper so DBConn() uses it
	viper.Set("db_dsn", testDSN)
	viper.Set("db_debug", false) // Set to true for SQL debugging

	db, err := database.DBConn()
	require.NoError(t, err, "Failed to connect to test database")

	return db
}

// TestData holds test data for authorization tests
type TestData struct {
	// Users
	AdminUser   *userModels.Person
	TeacherUser *userModels.Person
	StudentUser *userModels.Person
	RegularUser *userModels.Person

	// Students
	Student1 *userModels.Student
	Student2 *userModels.Student

	// Staff
	TeacherStaff *userModels.Staff

	// Teachers
	Teacher1 *userModels.Teacher

	// Groups
	Group1 *education.Group
	Group2 *education.Group

	// Active Groups
	ActiveGroup1 *active.Group
	ActiveGroup2 *active.Group

	// Visits
	Visit1 *active.Visit
	Visit2 *active.Visit

	// JWT Tokens
	AdminToken   string
	TeacherToken string
	StudentToken string
	UserToken    string
}

// CreateTestJWTAuth creates a JWT auth service with a test secret
func CreateTestJWTAuth() (*jwt.TokenAuth, error) {
	// Use the regular JWT secret from environment variables
	testSecret := viper.GetString("AUTH_JWT_SECRET")
	if testSecret == "" {
		testSecret = "TEST_SECRET_KEY_DO_NOT_USE_IN_PRODUCTION"
	}
	return jwt.NewTokenAuthWithSecret(testSecret)
}

// SetupTestDatabase configures the test database connection
func SetupTestDatabase() {
	// Set test database configuration for in-memory or test database
	// Use DB_DSN environment variable or fallback to a test-specific connection
	testDsn := viper.GetString("DB_DSN")
	if testDsn == "" {
		// Default to SSL with certificate verification for test database
		testDsn = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=verify-ca&sslrootcert=/var/lib/postgresql/ssl/certs/ca.crt"
	}
	viper.Set("test_db_dsn", testDsn)
	// You could also use an in-memory SQLite database for tests if desired
}

// CreateTestData creates a comprehensive test data set
func CreateTestData(tb testing.TB) *TestData {
	data := &TestData{}

	// Setup test database
	SetupTestDatabase()

	// Create JWT auth service with test secret
	tokenAuth, err := CreateTestJWTAuth()
	require.NoError(tb, err)

	// Create admin user
	adminID := int64(1)
	data.AdminUser = &userModels.Person{
		Model: base.Model{
			ID: 1,
		},
		FirstName: "Admin",
		LastName:  "User",
		AccountID: &adminID,
	}

	// Create teacher user
	teacherID := int64(2)
	data.TeacherUser = &userModels.Person{
		Model: base.Model{
			ID: 2,
		},
		FirstName: "Teacher",
		LastName:  "User",
		AccountID: &teacherID,
	}

	// Create student user
	studentID := int64(3)
	data.StudentUser = &userModels.Person{
		Model: base.Model{
			ID: 3,
		},
		FirstName: "Student",
		LastName:  "User",
		AccountID: &studentID,
	}

	// Create regular user
	userID := int64(4)
	data.RegularUser = &userModels.Person{
		Model: base.Model{
			ID: 4,
		},
		FirstName: "Regular",
		LastName:  "User",
		AccountID: &userID,
	}

	// Create groups
	data.Group1 = &education.Group{
		Model: base.Model{
			ID: 1,
		},
		Name: "Class A",
	}

	data.Group2 = &education.Group{
		Model: base.Model{
			ID: 2,
		},
		Name: "Class B",
	}

	// Create teacher staff
	data.TeacherStaff = &userModels.Staff{
		Model: base.Model{
			ID: 1,
		},
		PersonID: data.TeacherUser.ID,
		Person:   data.TeacherUser,
	}

	// Create teacher
	data.Teacher1 = &userModels.Teacher{
		Model: base.Model{
			ID: 1,
		},
		StaffID: data.TeacherStaff.ID,
		Staff:   data.TeacherStaff,
	}

	// Create students
	groupID1 := data.Group1.ID
	data.Student1 = &userModels.Student{
		Model: base.Model{
			ID: 1,
		},
		PersonID: data.StudentUser.ID,
		Person:   data.StudentUser,
		GroupID:  &groupID1,
	}

	groupID2 := data.Group2.ID
	data.Student2 = &userModels.Student{
		Model: base.Model{
			ID: 2,
		},
		PersonID: 5, // Different person
		GroupID:  &groupID2,
	}

	// Create active groups
	data.ActiveGroup1 = &active.Group{
		Model: base.Model{
			ID: 1,
		},
		GroupID:   data.Group1.ID,
		RoomID:    1,
		StartTime: time.Now().Add(-1 * time.Hour),
	}

	data.ActiveGroup2 = &active.Group{
		Model: base.Model{
			ID: 2,
		},
		GroupID:   data.Group2.ID,
		RoomID:    2,
		StartTime: time.Now().Add(-1 * time.Hour),
	}

	// Create visits
	data.Visit1 = &active.Visit{
		Model: base.Model{
			ID: 1,
		},
		StudentID:     data.Student1.ID,
		ActiveGroupID: data.ActiveGroup1.ID,
		EntryTime:     time.Now().Add(-30 * time.Minute),
		Student:       data.Student1,
		ActiveGroup:   data.ActiveGroup1,
	}

	data.Visit2 = &active.Visit{
		Model: base.Model{
			ID: 2,
		},
		StudentID:     data.Student2.ID,
		ActiveGroupID: data.ActiveGroup2.ID,
		EntryTime:     time.Now().Add(-30 * time.Minute),
		Student:       data.Student2,
		ActiveGroup:   data.ActiveGroup2,
	}

	// Create JWT tokens
	adminClaims := jwt.AppClaims{
		ID:          1,
		Username:    "admin",
		Roles:       []string{"admin"},
		Permissions: []string{"admin:*"},
	}
	data.AdminToken, err = tokenAuth.CreateJWT(adminClaims)
	require.NoError(tb, err)

	teacherClaims := jwt.AppClaims{
		ID:          2,
		Username:    "teacher",
		Roles:       []string{"teacher"},
		Permissions: []string{"groups:read", "visits:read"},
	}
	data.TeacherToken, err = tokenAuth.CreateJWT(teacherClaims)
	require.NoError(tb, err)

	studentClaims := jwt.AppClaims{
		ID:          3,
		Username:    "student",
		Roles:       []string{"student"},
		Permissions: []string{},
	}
	data.StudentToken, err = tokenAuth.CreateJWT(studentClaims)
	require.NoError(tb, err)

	userClaims := jwt.AppClaims{
		ID:          4,
		Username:    "user",
		Roles:       []string{"user"},
		Permissions: []string{},
	}
	data.UserToken, err = tokenAuth.CreateJWT(userClaims)
	require.NoError(tb, err)

	return data
}

// CreateTestAuthorizationService creates a test authorization service with registered policies
func CreateTestAuthorizationService(tb testing.TB) authorize.AuthorizationService {
	authService := authorize.NewAuthorizationService()

	// Register test policies here as needed
	// For example:
	// testPolicy := NewTestPolicy()
	// err := authService.RegisterPolicy(testPolicy)
	// require.NoError(t, err)

	return authService
}

// MockJWTContext adds JWT claims and permissions to a context
func MockJWTContext(ctx context.Context, claims jwt.AppClaims, permissions []string) context.Context {
	ctx = context.WithValue(ctx, ctxKeyClaims, claims)
	ctx = context.WithValue(ctx, ctxKeyPermissions, permissions)
	return ctx
}

// Context keys for testing
type ctxKey int

const (
	ctxKeyClaims ctxKey = iota
	ctxKeyPermissions
)

// CreateTestClaims creates test JWT claims
func CreateTestClaims(id int64, username string, roles []string, permissions []string) jwt.AppClaims {
	return jwt.AppClaims{
		ID:          int(id),
		Username:    username,
		Roles:       roles,
		Permissions: permissions,
		CommonClaims: jwt.CommonClaims{
			IssuedAt:  time.Now().Unix(),
			ExpiresAt: time.Now().Add(1 * time.Hour).Unix(),
		},
	}
}

// TestPermissionScenario tests a permission scenario
type TestPermissionScenario struct {
	Name            string
	Permission      string
	UserPermissions []string
	ExpectedResult  bool
}

// RunPermissionScenarios runs a series of permission test scenarios
func RunPermissionScenarios(t *testing.T, scenarios []TestPermissionScenario) {
	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			// This is a simplified example - real implementation would
			// test against actual permission checking logic
			hasPermission := false
			for _, perm := range scenario.UserPermissions {
				if perm == scenario.Permission || perm == "admin:*" || perm == "*:*" {
					hasPermission = true
					break
				}
			}

			require.Equal(t, scenario.ExpectedResult, hasPermission)
		})
	}
}

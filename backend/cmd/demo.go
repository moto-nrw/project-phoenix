package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	mathrand "math/rand"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	backendapi "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	seedapi "github.com/moto-nrw/project-phoenix/seed/api"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/simulate"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const standingDemoSlug = "messe-demo"

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "Run the standing demo school with database-backed state",
	RunE: func(cmd *cobra.Command, _ []string) error {
		baseURL, _ := cmd.Flags().GetString("url")
		once, _ := cmd.Flags().GetBool("once")
		heartbeat, _ := cmd.Flags().GetString("heartbeat")
		if err := validateDemoTarget(baseURL, os.Getenv("APP_ENV")); err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		db, err := database.DBConnForDemo()
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()
		schools, err := backendapi.NewDemoRuntime(db)
		if err != nil {
			return err
		}
		// One lease for both modes: the fallback and the scheduler never run side by side.
		err = schools.WithDemoLease(ctx, standingDemoSlug, func(ctx context.Context) error {
			if viper.GetBool("demo_standing_school") {
				return runStandingDemo(ctx, schools, baseURL, heartbeat, once)
			}
			return runDemoSchools(ctx, schools, baseURL, heartbeat, once)
		})
		if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
			return nil
		}
		return err
	},
}

func init() {
	RootCmd.AddCommand(demoCmd)
	demoCmd.Flags().String("url", "http://localhost:8080", "Internal backend URL (demo: http://server:8080)")
	demoCmd.Flags().Bool("once", false, "Provision or reload the school, execute one tick and exit (smoke verification)")
	demoCmd.Flags().String("heartbeat", "", "File rewritten after every successful tick; the container healthcheck reads its age")
}

func validateDemoTarget(baseURL, environment string) error {
	if err := assertDevOnlyTarget(baseURL, environment); err != nil {
		return err
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("demo URL must use HTTP or HTTPS")
	}
	if strings.EqualFold(strings.TrimSpace(environment), "demo") && parsed.Hostname() != "server" {
		return fmt.Errorf("APP_ENV=demo requires the internal server host")
	}
	return nil
}

// writeDemoHeartbeat records a successful tick for the container healthcheck. Only
// the file's modification time matters; an empty path disables the heartbeat.
func writeDemoHeartbeat(path string) error {
	if path == "" {
		return nil
	}
	return os.WriteFile(path, []byte(time.Now().UTC().Format(time.RFC3339)+"\n"), 0o600)
}

func runStandingDemo(ctx context.Context, schools *backendapi.DemoRuntime, baseURL, heartbeat string, once bool) error {
	saved, err := schools.LoadDemoSchool(ctx, standingDemoSlug)
	if err != nil {
		return err
	}
	adapter := newSeedCommandAdapter(baseURL, false)
	if err := waitDemoServer(ctx, adapter); err != nil {
		return err
	}
	if saved == nil {
		options := seedapi.SeedOptions{TenantSlug: standingDemoSlug, SchoolName: "moto Demo-Schule"}
		if err := provisionDemoSchool(ctx, schools, adapter, options); err != nil {
			return err
		}
	}
	school, err := loadDemoSchool(ctx, schools, standingDemoSlug, baseURL)
	if err != nil {
		return err
	}
	return school.run(ctx, once, true, func() {
		if err := writeDemoHeartbeat(heartbeat); err != nil {
			slog.Warn("demo heartbeat not written; the healthcheck will report unhealthy", "error", err)
		}
	})
}

// demoSchool is one seeded demo school with its own client and ticker.
// visitorID and visitorParentID are the caregiver and the parent who carry
// the visitor's name (#3463, #3468).
type demoSchool struct {
	slug            string
	state           *simulate.SeedState
	client          *seedapi.Client
	ticker          *simulate.DemoTicker
	visitorID       int64
	visitorParentID int64
}

func loadDemoSchool(ctx context.Context, schools *backendapi.DemoRuntime, slug, baseURL string) (*demoSchool, error) {
	saved, err := schools.LoadDemoSchool(ctx, slug)
	if err != nil {
		return nil, err
	}
	if saved == nil {
		return nil, fmt.Errorf("demo provisioning did not save its school")
	}
	var contract seedapi.SeedState
	if err := json.Unmarshal(saved.SeedJSON, &contract); err != nil {
		return nil, fmt.Errorf("invalid stored demo state")
	}
	profile, err := contract.SelectProfile(seedapi.DefaultProfileKey)
	if err != nil {
		return nil, err
	}
	if profile.School.ID != saved.SchoolID {
		return nil, fmt.Errorf("stored demo school identity mismatch")
	}
	state, err := simulate.SeedStateFromContract(&contract, seedapi.DefaultProfileKey)
	if err != nil {
		return nil, err
	}
	state.BaseURL = baseURL // Never trust a persisted endpoint as an HTTP target.
	// Every school keeps its own client: a client holds one school's login.
	client := seedapi.NewClientWithAdapter(newSeedCommandAdapter(baseURL, false), false)
	query, err := demoVisitsQuery(schools, saved.SchoolID, profile)
	if err != nil {
		return nil, err
	}
	visitorParentID := seedapi.VisitorParentAccountID(profile)
	ticker, err := simulate.NewDemoTicker(simulate.DemoTickOptions{
		State: state, Client: client, Now: time.Now, Visits: query,
		// The other parents ask for pickup changes and write messages (#3468);
		// they keep a client of their own, apart from the admin's login.
		Parents: simulate.OtherDemoParents(profile.Credentials.Parents, visitorParentID),
		ParentClient: func() simulate.DemoParentClient {
			return seedapi.NewClientWithAdapter(newSeedCommandAdapter(baseURL, false), false)
		},
	})
	if err != nil {
		return nil, err
	}
	if len(state.Accounts.Admin) == 0 {
		return nil, fmt.Errorf("stored demo has no admin")
	}
	return &demoSchool{
		slug: slug, state: state, client: client, ticker: ticker,
		visitorID: seedapi.VisitorAccountID(profile), visitorParentID: visitorParentID,
	}, nil
}

// tick signs the school's admin in when due and executes one tick.
func (s *demoSchool) tick(ctx context.Context, nextLogin *time.Time) error {
	if time.Now().After(*nextLogin) {
		admin := s.state.Accounts.Admin[0]
		if err := s.client.Login(admin.Email, admin.Password, s.state.Bootstrap.TenantSlug); err != nil {
			return fmt.Errorf("%w: %w", errDemoLogin, err)
		}
		*nextLogin = time.Now().Add(10 * time.Minute)
	}
	return s.ticker.Tick(ctx)
}

var errDemoLogin = errors.New("demo admin login failed")

// run ticks the school every 5–8 seconds until ctx ends; ticked reports every
// successful tick. The standing demo stops on a failed login and lets the
// container restart; one school among many only retries.
func (s *demoSchool) run(ctx context.Context, once, stopOnLogin bool, ticked func()) error {
	var nextLogin time.Time
	for {
		if ctx.Err() != nil {
			return nil
		}
		tickErr := s.tick(ctx, &nextLogin)
		if once || (stopOnLogin && errors.Is(tickErr, errDemoLogin)) {
			return tickErr
		}
		if tickErr != nil && ctx.Err() == nil {
			slog.Warn("demo tick failed; retrying",
				"school", s.slug,
				"error", tickErr,
			)
			nextLogin = time.Time{}
		}
		if tickErr == nil {
			ticked()
		}
		if err := waitDemoInterval(ctx, 5*time.Second+time.Duration(mathrand.Intn(3001))*time.Millisecond); err != nil {
			return nil
		}
	}
}

// provisionDemoSchool seeds the school of options and stores its state under
// its slug. Tenant slug and school name come from the caller.
func provisionDemoSchool(ctx context.Context, schools *backendapi.DemoRuntime, adapter seedapi.Adapter, options seedapi.SeedOptions) error {
	email, password, pin := viper.GetString("operator_email"), viper.GetString("operator_password"), viper.GetString("ogs_device_pin")
	if email == "" || password == "" || pin == "" {
		return fmt.Errorf("demo provisioning requires OPERATOR_EMAIL, OPERATOR_PASSWORD and OGS_DEVICE_PIN")
	}
	staffPassword, err := seedapi.GenerateSeedPassword(services.SecureRandomSource())
	if err != nil {
		return err
	}
	slug := options.TenantSlug
	options.OnlyProfile, options.StaffPassword, options.StandingDemo = seedapi.DefaultProfileKey, staffPassword, true
	options.SaveState = func(ctx context.Context, state *seedapi.SeedState) error {
		profile, err := state.SelectProfile(seedapi.DefaultProfileKey)
		if err != nil {
			return err
		}
		profile.Credentials.Operator = nil
		raw, err := json.Marshal(state)
		if err != nil {
			return err
		}
		return schools.RememberDemoSchool(ctx, slug, backendapi.DemoSchoolRecord{SchoolID: profile.School.ID, SeedJSON: raw})
	}
	_, err = seedapi.NewSeeder(adapter, services.SecureRandomSource(), false, options).Seed(ctx, email, password, pin)
	return err
}

func demoVisitsQuery(runtime *backendapi.DemoRuntime, schoolID int64, profile *seedapi.SeedProfile) (func(context.Context) ([]simulate.DemoVisit, error), error) {
	var webDeviceID int64
	for _, device := range profile.Devices {
		if device.Protected && device.DeviceType == "virtual" {
			webDeviceID = device.ID
		}
	}
	if webDeviceID == 0 {
		return nil, fmt.Errorf("demo school has no protected web device")
	}
	return func(ctx context.Context) ([]simulate.DemoVisit, error) {
		var rows []backendapi.DemoVisit
		today := timezone.TodayDate()
		ctx = tenant.WithTenantID(tenant.WithUnitOfWork(ctx, runtime.TenantRuntime()), schoolID)
		err := tenant.WithinCurrentTenant(ctx, func(ctx context.Context) error {
			var err error
			rows, err = runtime.LatestDemoVisits(ctx, webDeviceID, today.AddDays(-1).String(), today.String())
			return err
		})
		if err != nil {
			return nil, err
		}
		result := make([]simulate.DemoVisit, 0, len(rows))
		for _, row := range rows {
			result = append(result, simulate.DemoVisit{StudentID: row.StudentID, Active: row.Active, Web: row.Web, ChangedAt: row.ChangedAt})
		}
		return result, nil
	}, nil
}

func waitDemoServer(ctx context.Context, adapter seedapi.Adapter) error {
	for {
		if err := adapter.CheckHealth(ctx); err == nil {
			return nil
		}
		if err := waitDemoInterval(ctx, time.Second); err != nil {
			return err
		}
	}
}

func waitDemoInterval(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

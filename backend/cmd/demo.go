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
		if err := validateDemoTarget(baseURL, os.Getenv("APP_ENV")); err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		db, err := database.DBConn()
		if err != nil {
			return err
		}
		defer func() { _ = db.Close() }()
		schools, err := backendapi.NewDemoRuntime(db)
		if err != nil {
			return err
		}
		err = schools.WithDemoLease(ctx, standingDemoSlug, func(ctx context.Context) error {
			return runStandingDemo(ctx, schools, baseURL, once)
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

func runStandingDemo(ctx context.Context, schools *backendapi.DemoRuntime, baseURL string, once bool) error {
	saved, err := schools.LoadDemoSchool(ctx, standingDemoSlug)
	if err != nil {
		return err
	}
	adapter := newSeedCommandAdapter(baseURL, false)
	if err := waitDemoServer(ctx, adapter); err != nil {
		return err
	}
	if saved == nil {
		if err := provisionStandingDemo(ctx, schools, adapter); err != nil {
			return err
		}
		saved, err = schools.LoadDemoSchool(ctx, standingDemoSlug)
		if err != nil {
			return err
		}
	}
	if saved == nil {
		return fmt.Errorf("demo provisioning did not save its school")
	}
	var contract seedapi.SeedState
	if err := json.Unmarshal(saved.SeedJSON, &contract); err != nil {
		return fmt.Errorf("invalid stored demo state")
	}
	profile, err := contract.SelectProfile(seedapi.DefaultProfileKey)
	if err != nil {
		return err
	}
	if profile.School.ID != saved.SchoolID {
		return fmt.Errorf("stored demo school identity mismatch")
	}
	state, err := simulate.SeedStateFromContract(&contract, seedapi.DefaultProfileKey)
	if err != nil {
		return err
	}
	state.BaseURL = baseURL // Never trust a persisted endpoint as an HTTP target.
	client := seedapi.NewClientWithAdapter(adapter, false)
	query, err := demoVisitsQuery(schools, saved.SchoolID, profile)
	if err != nil {
		return err
	}
	ticker, err := simulate.NewDemoTicker(simulate.DemoTickOptions{State: state, Client: client, Now: time.Now, Visits: query})
	if err != nil {
		return err
	}
	if len(state.Accounts.Admin) == 0 {
		return fmt.Errorf("stored demo has no admin")
	}
	admin := state.Accounts.Admin[0]
	var nextLogin time.Time
	for {
		if ctx.Err() != nil {
			return nil
		}
		if time.Now().After(nextLogin) {
			if err := client.Login(admin.Email, admin.Password, state.Bootstrap.TenantSlug); err != nil {
				return fmt.Errorf("demo admin login failed: %w", err)
			}
			nextLogin = time.Now().Add(10 * time.Minute)
		}
		tickErr := ticker.Tick(ctx)
		if once {
			return tickErr
		}
		if err := tickErr; err != nil && ctx.Err() == nil {
			slog.Warn("demo tick failed; retrying", "error", err)
			nextLogin = time.Time{}
		}
		if err := waitDemoInterval(ctx, 5*time.Second+time.Duration(mathrand.Intn(3001))*time.Millisecond); err != nil {
			return nil
		}
	}
}

func provisionStandingDemo(ctx context.Context, schools *backendapi.DemoRuntime, adapter seedapi.Adapter) error {
	email, password, pin := viper.GetString("operator_email"), viper.GetString("operator_password"), viper.GetString("ogs_device_pin")
	if email == "" || password == "" || pin == "" {
		return fmt.Errorf("demo provisioning requires OPERATOR_EMAIL, OPERATOR_PASSWORD and OGS_DEVICE_PIN")
	}
	staffPassword, err := seedapi.GenerateSeedPassword(services.SecureRandomSource())
	if err != nil {
		return err
	}
	options := seedapi.SeedOptions{
		TenantSlug: standingDemoSlug, SchoolName: "moto Demo-Schule", OnlyProfile: seedapi.DefaultProfileKey,
		StaffPassword: staffPassword, StandingDemo: true,
		SaveState: func(ctx context.Context, state *seedapi.SeedState) error {
			profile, err := state.SelectProfile(seedapi.DefaultProfileKey)
			if err != nil {
				return err
			}
			profile.Credentials.Operator = nil
			raw, err := json.Marshal(state)
			if err != nil {
				return err
			}
			return schools.RememberDemoSchool(ctx, standingDemoSlug, backendapi.DemoSchoolRecord{SchoolID: profile.School.ID, SeedJSON: raw})
		},
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
		ctx = tenant.WithTenantID(tenant.WithUnitOfWork(ctx, runtime.TenantRuntime()), schoolID)
		err := tenant.WithinCurrentTenant(ctx, func(ctx context.Context) error {
			var err error
			rows, err = runtime.LatestDemoVisits(ctx, webDeviceID)
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

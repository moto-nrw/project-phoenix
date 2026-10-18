package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
)

type Settings struct {
	Enabled            func(context.Context) (bool, error)
	Host               func(context.Context) (string, error)
	Port               func(context.Context) (int, error)
	Username           func(context.Context) (string, error)
	Password           func(context.Context) (string, error)
	RemoteDirectory    func(context.Context) (string, error)
	HostKeyFingerprint func(context.Context) (string, error)
}

type SettingKeys struct {
	Host               string
	Port               string
	Username           string
	Password           string
	RemoteDirectory    string
	HostKeyFingerprint string
}

// SettingsResolver turns the seven settings into a validated target.
//
// Completeness is decided here; reachability is not. Whether the host is a
// public address and presents the configured key is decided at connect time
// by the transport — those are properties of the network, not of a form.
type SettingsResolver struct {
	Settings Settings
	Keys     SettingKeys
}

func (r SettingsResolver) read(ctx context.Context) (bool, domain.Target, error) {
	enabled, err := r.Settings.Enabled(ctx)
	if err != nil {
		return false, domain.Target{}, fmt.Errorf("resolve sftp enabled: %w", err)
	}

	target := domain.Target{}
	// Trimmed: surrounding whitespace in a host, path or fingerprint is
	// always a copy-paste artefact, never part of the value.
	for _, field := range []struct {
		read func(context.Context) (string, error)
		into *string
		name string
	}{
		{r.Settings.Host, &target.Host, "host"},
		{r.Settings.Username, &target.Username, "username"},
		{r.Settings.RemoteDirectory, &target.RemoteDirectory, "remote directory"},
		{r.Settings.HostKeyFingerprint, &target.HostKeyFingerprint, "host key fingerprint"},
	} {
		value, err := field.read(ctx)
		if err != nil {
			return false, domain.Target{}, fmt.Errorf("resolve sftp %s: %w", field.name, err)
		}
		*field.into = strings.TrimSpace(value)
	}

	// The password is taken EXACTLY as stored. Trimming it would authenticate
	// with a different secret than the school entered, and the failure would
	// look like a wrong password on the far side.
	password, err := r.Settings.Password(ctx)
	if err != nil {
		return false, domain.Target{}, fmt.Errorf("resolve sftp password: %w", err)
	}
	target.Password = password

	port, err := r.Settings.Port(ctx)
	if err != nil {
		return false, domain.Target{}, fmt.Errorf("resolve sftp port: %w", err)
	}
	target.Port = port

	return enabled, target, nil
}

// missing lists the setting keys that block a transfer, in form order. The
// password is checked for presence only; its content is never inspected.
func (r SettingsResolver) missing(target domain.Target) []string {
	var missing []string
	if target.Host == "" {
		missing = append(missing, r.Keys.Host)
	}
	if !domain.ValidPort(target.Port) {
		missing = append(missing, r.Keys.Port)
	}
	if target.Username == "" {
		missing = append(missing, r.Keys.Username)
	}
	if target.Password == "" {
		missing = append(missing, r.Keys.Password)
	}
	if !domain.ValidRemoteDirectory(target.RemoteDirectory) {
		missing = append(missing, r.Keys.RemoteDirectory)
	}
	if !domain.ValidFingerprint(target.HostKeyFingerprint) {
		missing = append(missing, r.Keys.HostKeyFingerprint)
	}
	return missing
}

func (r SettingsResolver) Resolve(ctx context.Context) (domain.Target, error) {
	enabled, target, err := r.read(ctx)
	if err != nil {
		return domain.Target{}, err
	}
	if !enabled {
		return domain.Target{}, fmt.Errorf("%w: transfer is switched off", domain.ErrNotConfigured)
	}
	if missing := r.missing(target); len(missing) > 0 {
		return domain.Target{}, fmt.Errorf("%w: missing %s", domain.ErrNotConfigured, strings.Join(missing, ", "))
	}
	return target, nil
}

func (r SettingsResolver) State(ctx context.Context) (domain.TargetState, error) {
	enabled, target, err := r.read(ctx)
	if err != nil {
		return domain.TargetState{}, err
	}
	state := domain.TargetState{
		Enabled:         enabled,
		Host:            target.Host,
		RemoteDirectory: target.RemoteDirectory,
		MissingSettings: r.missing(target),
	}
	if domain.ValidPort(target.Port) {
		state.Port = target.Port
	}
	return state, nil
}

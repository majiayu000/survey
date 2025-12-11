package db

import (
	"os"
	"testing"
)

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name    string
		envVars map[string]string
		want    Config
		wantErr bool
	}{
		{
			name: "all environment variables set",
			envVars: map[string]string{
				"DATABASE_HOST":     "db.example.com",
				"DATABASE_PORT":     "5433",
				"DATABASE_USER":     "testuser",
				"DATABASE_PASSWORD": "testpass",
				"DATABASE_NAME":     "testdb",
				"DATABASE_SSLMODE":  "require",
			},
			want: Config{
				Host:     "db.example.com",
				Port:     5433,
				User:     "testuser",
				Password: "testpass",
				Database: "testdb",
				SSLMode:  "require",
			},
			wantErr: false,
		},
		{
			name: "defaults applied when vars not set",
			envVars: map[string]string{
				"DATABASE_PASSWORD": "testpass",
			},
			want: Config{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Password: "testpass",
				Database: "survey",
				SSLMode:  "disable",
			},
			wantErr: false,
		},
		{
			name:    "missing password returns error",
			envVars: map[string]string{},
			want:    Config{},
			wantErr: true,
		},
		{
			name: "invalid port number",
			envVars: map[string]string{
				"DATABASE_PASSWORD": "testpass",
				"DATABASE_PORT":     "invalid",
			},
			want:    Config{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			clearDBEnv()

			// Set test environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
			}
			defer clearDBEnv()

			got, err := ConfigFromEnv()
			if (err != nil) != tt.wantErr {
				t.Errorf("ConfigFromEnv() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if got.Host != tt.want.Host {
					t.Errorf("ConfigFromEnv().Host = %v, want %v", got.Host, tt.want.Host)
				}
				if got.Port != tt.want.Port {
					t.Errorf("ConfigFromEnv().Port = %v, want %v", got.Port, tt.want.Port)
				}
				if got.User != tt.want.User {
					t.Errorf("ConfigFromEnv().User = %v, want %v", got.User, tt.want.User)
				}
				if got.Password != tt.want.Password {
					t.Errorf("ConfigFromEnv().Password = %v, want %v", got.Password, tt.want.Password)
				}
				if got.Database != tt.want.Database {
					t.Errorf("ConfigFromEnv().Database = %v, want %v", got.Database, tt.want.Database)
				}
				if got.SSLMode != tt.want.SSLMode {
					t.Errorf("ConfigFromEnv().SSLMode = %v, want %v", got.SSLMode, tt.want.SSLMode)
				}
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Password: "secret",
				Database: "survey",
				SSLMode:  "disable",
			},
			wantErr: false,
		},
		{
			name: "missing password",
			config: Config{
				Host:     "localhost",
				Port:     5432,
				User:     "postgres",
				Password: "",
				Database: "survey",
				SSLMode:  "disable",
			},
			wantErr: true,
		},
		{
			name: "invalid port (zero)",
			config: Config{
				Host:     "localhost",
				Port:     0,
				User:     "postgres",
				Password: "secret",
				Database: "survey",
				SSLMode:  "disable",
			},
			wantErr: true,
		},
		{
			name: "invalid port (negative)",
			config: Config{
				Host:     "localhost",
				Port:     -1,
				User:     "postgres",
				Password: "secret",
				Database: "survey",
				SSLMode:  "disable",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigConnectionString(t *testing.T) {
	cfg := Config{
		Host:     "db.example.com",
		Port:     5433,
		User:     "testuser",
		Password: "testpass",
		Database: "testdb",
		SSLMode:  "require",
	}

	got := cfg.ConnectionString()
	want := "host=db.example.com port=5433 user=testuser password=testpass dbname=testdb sslmode=require"

	if got != want {
		t.Errorf("Config.ConnectionString() = %v, want %v", got, want)
	}
}

// clearDBEnv clears all database-related environment variables
func clearDBEnv() {
	os.Unsetenv("DATABASE_HOST")
	os.Unsetenv("DATABASE_PORT")
	os.Unsetenv("DATABASE_USER")
	os.Unsetenv("DATABASE_PASSWORD")
	os.Unsetenv("DATABASE_NAME")
	os.Unsetenv("DATABASE_SSLMODE")
}

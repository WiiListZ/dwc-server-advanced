package api

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dwc-server-advanced/common"
	"dwc-server-advanced/database"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// The setup wizard can create the PostgreSQL database with Docker Compose.
// The compose project lives in <data dir>/postgres, binds PostgreSQL to
// 127.0.0.1 only and keeps its credentials in postgres/.env, so running the
// wizard again reuses the same database.

const dockerDatabaseCompose = `# Created by the dwc-server-advanced setup wizard.
# Manage it with: docker compose up -d  /  docker compose stop  (in this folder)
# The credentials and the project name are in .env next to this file.
services:
  postgres:
    image: postgres:16-alpine
    restart: unless-stopped
    environment:
      POSTGRES_DB: ${DWC_DB_NAME}
      POSTGRES_USER: ${DWC_DB_USER}
      POSTGRES_PASSWORD: ${DWC_DB_PASSWORD}
    ports:
      - "127.0.0.1:${DWC_DB_PORT}:5432"
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U \"$${POSTGRES_USER}\" -d \"$${POSTGRES_DB}\""]
      interval: 5s
      timeout: 5s
      retries: 20

volumes:
  postgres-data:
`

type dockerStatus struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	Version   string `json:"version,omitempty"`
}

type dockerDatabaseResult struct {
	DatabaseAddress string `json:"databaseAddress"`
	DatabaseName    string `json:"databaseName"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	ComposeFile     string `json:"composeFile"`
	Reused          bool   `json:"reused"`
}

func runDocker(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// checkDocker reports whether Docker with the compose plugin can be used.
func checkDocker() dockerStatus {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return dockerStatus{Reason: "The server runs inside a Docker container. Use the postgres service of docker-compose.yml with host \"postgres\"."}
	}
	if _, err := exec.LookPath("docker"); err != nil {
		return dockerStatus{Reason: "Docker is not installed on this machine."}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	version, err := runDocker(ctx, "", "info", "--format", "{{.ServerVersion}}")
	if err != nil {
		reason := "Docker is installed but not running. Start Docker (Docker Desktop on Windows and macOS) and check again."
		if strings.Contains(strings.ToLower(version), "permission denied") {
			reason = "This user cannot use Docker. Add it to the docker group, or create the database yourself."
		}
		return dockerStatus{Reason: reason}
	}
	if _, err := runDocker(ctx, "", "compose", "version", "--short"); err != nil {
		return dockerStatus{Reason: "Docker Compose is missing. Install the docker compose plugin."}
	}
	return dockerStatus{Available: true, Version: strings.TrimSpace(version)}
}

func randomPassword(n int) string {
	const letters = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	for i := range b {
		v, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			panic(err)
		}
		b[i] = letters[v.Int64()]
	}
	return string(b)
}

func freeLocalPort(from, to int) (int, bool) {
	for port := from; port <= to; port++ {
		l, err := net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(port))
		if err != nil {
			continue
		}
		_ = l.Close()
		return port, true
	}
	return 0, false
}

func readEnvFile(path string) map[string]string {
	values := map[string]string{}
	file, err := os.Open(path)
	if err != nil {
		return values
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if key, value, ok := strings.Cut(line, "="); ok {
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return values
}

// createDockerDatabase writes the compose project into dataDir/postgres,
// starts it and waits until PostgreSQL accepts connections.
func createDockerDatabase(dataDir string) (dockerDatabaseResult, error) {
	if status := checkDocker(); !status.Available {
		return dockerDatabaseResult{}, errors.New(status.Reason)
	}

	dir := filepath.Join(dataDir, "postgres")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return dockerDatabaseResult{}, fmt.Errorf("cannot create %s: %w", dir, err)
	}
	envPath := filepath.Join(dir, ".env")
	composePath := filepath.Join(dir, "docker-compose.yml")

	env := readEnvFile(envPath)
	reused := env["DWC_DB_PASSWORD"] != "" && env["DWC_DB_PORT"] != ""
	if !reused {
		port, ok := freeLocalPort(5432, 5532)
		if !ok {
			return dockerDatabaseResult{}, errors.New("no free port between 5432 and 5532 on 127.0.0.1")
		}
		// One compose project per server folder, so two folders never share
		// a container or a volume.
		sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(dataDir))))
		env = map[string]string{
			"COMPOSE_PROJECT_NAME": "dwc-server-advanced-db-" + hex.EncodeToString(sum[:])[:8],
			"DWC_DB_NAME":          "wwfc",
			"DWC_DB_USER":          "dwc",
			"DWC_DB_PASSWORD":      randomPassword(24),
			"DWC_DB_PORT":          strconv.Itoa(port),
		}
		content := "# PostgreSQL credentials for the dwc-server-advanced database. Keep private.\n"
		for _, key := range []string{"COMPOSE_PROJECT_NAME", "DWC_DB_NAME", "DWC_DB_USER", "DWC_DB_PASSWORD", "DWC_DB_PORT"} {
			content += key + "=" + env[key] + "\n"
		}
		if err := os.WriteFile(envPath, []byte(content), 0o600); err != nil {
			return dockerDatabaseResult{}, fmt.Errorf("cannot write %s: %w", envPath, err)
		}
	}
	if err := os.WriteFile(composePath, []byte(dockerDatabaseCompose), 0o644); err != nil {
		return dockerDatabaseResult{}, fmt.Errorf("cannot write %s: %w", composePath, err)
	}

	// The first start downloads the PostgreSQL image, which can take minutes.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if out, err := runDocker(ctx, dir, "compose", "up", "-d"); err != nil {
		return dockerDatabaseResult{}, fmt.Errorf("docker compose up failed: %s", lastLines(out, 6))
	}

	result := dockerDatabaseResult{
		DatabaseAddress: "127.0.0.1:" + env["DWC_DB_PORT"],
		DatabaseName:    env["DWC_DB_NAME"],
		Username:        env["DWC_DB_USER"],
		Password:        env["DWC_DB_PASSWORD"],
		ComposeFile:     composePath,
		Reused:          reused,
	}
	if result.DatabaseAddress == "127.0.0.1:5432" {
		result.DatabaseAddress = "127.0.0.1"
	}

	cfg := common.DefaultConfig()
	cfg.DatabaseAddress, cfg.DatabaseName, cfg.Username, cfg.Password = result.DatabaseAddress, result.DatabaseName, result.Username, result.Password
	deadline := time.Now().Add(2 * time.Minute)
	var lastErr error
	for time.Now().Before(deadline) {
		conn, err := database.TryConnect(cfg, 5*time.Second)
		if err == nil {
			conn.Close()
			return result, nil
		}
		lastErr = err
		time.Sleep(2 * time.Second)
	}
	return result, fmt.Errorf("the database container started but does not accept connections: %v", lastErr)
}

func lastLines(text string, n int) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

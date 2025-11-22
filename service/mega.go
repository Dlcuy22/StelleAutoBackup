package service

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// MegaService is a thin wrapper around megacmd CLI (mega-login, mega-put, mega-whoami, mega-logout, mega-mkdir)
type MegaService struct {
	Email   string
	Pass    string
	Timeout time.Duration
}

// NewMegaService constructs a new MegaService
func NewMegaService(email, pass string, timeout time.Duration) *MegaService {
	if timeout == 0 {
		timeout = 15 * time.Second
	}
	return &MegaService{Email: email, Pass: pass, Timeout: timeout}
}

// run executes a command with timeout and returns stdout+stderr and error
func (m *MegaService) run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Login logs into MEGA via CLI
func (m *MegaService) Login() error {
	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout)
	defer cancel()

	out, err := m.run(ctx, "mega-login", m.Email, m.Pass)
	if err != nil {
		return fmt.Errorf("mega-login failed: %w | output: %s", err, out)
	}
	return nil
}

// Logout logs out current session
func (m *MegaService) Logout() error {
	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout)
	defer cancel()

	out, err := m.run(ctx, "mega-logout")
	if err != nil {
		return fmt.Errorf("mega-logout failed: %w | output: %s", err, out)
	}
	return nil
}

// WhoAmI runs mega-whoami and returns raw output
func (m *MegaService) WhoAmI() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout)
	defer cancel()

	out, err := m.run(ctx, "mega-whoami")
	return out, err
}

// parseAccountEmail attempts to extract "Account e-mail: <email>" from output
func parseAccountEmail(out string) (string, bool) {
	// regex case-insensitive: Account e-mail: <email>
	re := regexp.MustCompile(`(?i)Account\s+e-?mail:\s*(\S+)`)
	m := re.FindStringSubmatch(out)
	if len(m) >= 2 {
		return strings.TrimSpace(m[1]), true
	}
	return "", false
}

// EnsureLoggedIn ensures MEGA is logged in with the desiredEmail.
// Behavior:
// - if mega-whoami shows desiredEmail => ok
// - if mega-whoami shows another email => logout then login with m.Email/m.Pass
// - if mega-whoami shows "Not logged in" => login
func (m *MegaService) EnsureLoggedIn(desiredEmail string) error {
	out, _ := m.WhoAmI() // ignore error from whoami: we'll infer from output
	trim := strings.TrimSpace(out)

	// 1) try to parse account email
	if email, ok := parseAccountEmail(out); ok {
		// logged in with some account
		if email == desiredEmail {
			// already logged in with correct account
			return nil
		}
		// logged in but wrong account -> logout then login
		// attempt logout (best-effort)
		_ = m.Logout()
		// attempt login
		if err := m.Login(); err != nil {
			return fmt.Errorf("re-login after logout failed: %v", err)
		}
		return nil
	}

	// 2) check Not logged in text (example: "[... cmd ERR  Not logged in.]")
	if strings.Contains(trim, "Not logged in") || strings.Contains(strings.ToLower(trim), "not logged in") {
		// not logged in -> perform login once
		if err := m.Login(); err != nil {
			return fmt.Errorf("login failed: %v | whoami output: %s", err, out)
		}
		return nil
	}

	// 3) other outputs (server starting, or weird text) — be conservative: if whoami shows nothing meaningful, try login once
	if trim == "" {
		// no useful output: try login (this may start the megacmd server)
		if err := m.Login(); err != nil {
			return fmt.Errorf("login attempt after empty whoami failed: %v", err)
		}
		return nil
	}

	// Fallback: try to parse email again (case when server started and printed lines before account)
	if email, ok := parseAccountEmail(out); ok {
		if email == desiredEmail {
			return nil
		}
		_ = m.Logout()
		if err := m.Login(); err != nil {
			return fmt.Errorf("re-login fallback failed: %v", err)
		}
		return nil
	}

	// As last resort, try login
	if err := m.Login(); err != nil {
		return fmt.Errorf("final login attempt failed: %v | whoami output: %s", err, out)
	}
	return nil
}

// Upload uploads localPath to remotePath (remotePath may be folder or full path).
// If remotePath is empty, upload to current remote dir (account root).
func (m *MegaService) Upload(localPath, remotePath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout*4)
	defer cancel()

	args := []string{localPath}
	if remotePath != "" {
		args = append(args, remotePath)
	}

	out, err := m.run(ctx, "mega-put", args...)
	if err != nil {
		return out, fmt.Errorf("mega-put failed: %w | output: %s", err, out)
	}
	return out, nil
}

func (m *MegaService) Mkdir(remotePath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout)
	defer cancel()

	out, err := m.run(ctx, "mega-mkdir", remotePath)
	if err != nil {
		return out, fmt.Errorf("mega-mkdir failed: %w | output: %s", err, out)
	}
	return out, nil
}

func (m *MegaService) List(remotePath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout)
	defer cancel()

	args := []string{}
	if remotePath != "" {
		args = append(args, remotePath)
	}
	out, err := m.run(ctx, "mega-ls", args...)
	if err != nil {
		return out, fmt.Errorf("mega-ls failed: %w | output: %s", err, out)
	}
	return out, nil
}

func (m *MegaService) PathExists(remotePath string) (bool, error) {
	out, err := m.List(remotePath)
	if err != nil {
		// mega-ls returns non-zero if path not exist; treat that as not exist
		if out == "" {
			return false, nil
		}
		// but if there's some other error, surface it
		return false, err
	}
	trim := strings.TrimSpace(out)
	return trim != "", nil
}

func (m *MegaService) EnsurePathRecursive(remotePath string) error {
	clean := filepath.ToSlash(remotePath)
	if clean == "" || clean == "/" {
		return nil
	}
	// we will create incrementally: /a, /a/b, /a/b/c
	parts := strings.Split(strings.TrimPrefix(clean, "/"), "/")
	cur := ""
	for _, p := range parts {
		cur = "/" + filepath.ToSlash(filepath.Join(strings.TrimPrefix(cur, "/"), p))
		ok, err := m.PathExists(cur)
		if err != nil {
			return err
		}
		if !ok {
			_, err := m.Mkdir(cur)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

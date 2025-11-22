# service.mega.md
> Documentation for the `service.MegaService` small wrapper around the MEGAcmd CLI tools.

## Overview

`MegaService` is a thin Go wrapper that uses the **MEGAcmd** command-line client (the `megacmd` binary) by invoking subprocesses (e.g. `mega-login`, `mega-whoami`, `mega-put`, `mega-mkdir`, `mega-ls`, etc.). It does **not** speak HTTP to MEGA — it shells out to the installed MEGAcmd program and parses the textual output.

This module focuses on pragmatic convenience for scripts and small services that need to perform common MEGA operations from Go, while handling some of the real-world messiness of a CLI-based integration (mixed stdout/stderr, server startup delays, locale-dependent messages, etc.).

---

## Safety & Practical Notes (please read)

* **Security:** The code supports passing a plaintext password to `mega-login`. Passing passwords on the command-line is insecure in multi-tenant systems because other processes/user accounts may be able to observe process arguments. Prefer interactive login, session tokens, or reusing a started `megacmd` session in production.
* **Dependency on host binary:** Behavior depends on the installed `megacmd` binary and its version. Output strings and exit codes vary across versions and locales.
* **Race conditions:** `EnsurePathRecursive` and `EnsureLoggedIn` use simple linear strategies — concurrent processes modifying the same session or paths may race.

---

## Quick example

```go
m := service.NewMegaService("me@example.com", "supersecret", 0) // 0 => default timeout (15s)
if err := m.EnsureLoggedIn("me@example.com"); err != nil {
    // handle login problem
}

if err := m.EnsurePathRecursive("/backups/daily"); err != nil {
    // handle remote path creation
}

out, err := m.Upload("/tmp/archive.tar.gz", "/backups/daily/")
if err != nil {
    // upload failed, out contains combined stdout+stderr
}
```

---

## API Reference

### `type MegaService struct`

Fields:

* `Email string` — account email used for login.
* `Pass string` — account password (plaintext in memory; be careful).
* `Timeout time.Duration` — default timeout used for external commands (per-command context).

---

### `func NewMegaService(email, pass string, timeout time.Duration) *MegaService`

Constructs a new `MegaService`. If `timeout == 0` a sensible default of **15 seconds** is used.

---

### `func (m *MegaService) Login() error`

Runs `mega-login <email> <password>` using the configured `Email` and `Pass`.

* Uses a context timeout of `m.Timeout`.
* Returns a wrapped error with combined output when login fails.
* **Security reminder:** Passing `Pass` on the command line can expose it to other processes.

---

### `func (m *MegaService) Logout() error`

Runs `mega-logout` to clear the current CLI session.

* Best-effort: returns an error if logout fails.
* Note: logging out the CLI session may affect other processes that rely on the same `megacmd` session.

---

### `func (m *MegaService) WhoAmI() (string, error)`

Runs `mega-whoami` and returns the raw combined stdout+stderr.

* The output is raw text; callers can parse it to determine logged-in account or detect "Not logged in" messages.

---

### `func (m *MegaService) EnsureLoggedIn(desiredEmail string) error`

Ensures the MEGAcmd session is logged into `desiredEmail`. Behavior (summary):

1. Call `mega-whoami` and try to parse an "Account e-mail: <email>" line.

   * If it matches `desiredEmail` → done.
   * If it shows a different email → attempt `mega-logout` then `mega-login`.
2. If output contains "Not logged in" or similar → call `Login()` once.
3. If `whoami` returned empty (server starting or garbled output) → try `Login()`.
4. If all else fails → final `Login()` attempt and return any error.

Notes:

* `EnsureLoggedIn` is conservative and tolerant of noisy `whoami` outputs.
* It ignores `whoami` errors and infers outcome from the textual output.

---

### `func (m *MegaService) Upload(localPath, remotePath string) (string, error)`

Runs `mega-put localPath [remotePath]`.

* Uses an extended timeout: `m.Timeout * 4`.
* Returns combined stdout+stderr and an error on failure.
* If `remotePath` is empty the file is uploaded to the account's current remote working directory.

---

### `func (m *MegaService) Mkdir(remotePath string) (string, error)`

Runs `mega-mkdir remotePath`.

* Treats common "Folder already exists" messages as success (returns output, nil).
* For other failures returns a wrapped error with output.
* Useful for idempotent directory creation in backup flows.

---

### `func (m *MegaService) List(remotePath string) (string, error)`

Runs `mega-ls [remotePath]` and returns raw output.

* If `remotePath` is empty it lists the current remote directory.
* Returns combined stdout+stderr on success or an error containing the output on failure.

---

### `func (m *MegaService) PathExists(remotePath string) (bool, error)`

Checks whether `remotePath` exists using `mega-ls`.

* Treats a blank output as "not exists". This accommodates `mega-ls` returning a non-zero exit code when a path doesn't exist.
* If `mega-ls` fails with a non-empty output, that error is propagated.

---

### `func (m *MegaService) EnsurePathRecursive(remotePath string) error`

Creates `remotePath` and any missing parent directories incrementally (e.g. `/a/b/c` → create `/a`, `/a/b`, `/a/b/c`).

* Not atomic; parallel calls creating same path may race.
* Uses `PathExists` and `Mkdir` incrementally.
* Accepts both `"/a/b/c"` and `a/b/c` forms via `filepath.ToSlash`.

---

## Internal helpers

### `func (m *MegaService) run(ctx context.Context, name string, args ...string) (string, error)`

* Executes the external command using `exec.CommandContext`.
* Returns combined stdout+stderr as `string` and any execution error.
* `CombinedOutput` is used because MEGAcmd sometimes writes useful info to `stderr`.

### `func parseAccountEmail(out string) (string, bool)`

* Internal regex-based parser that extracts `Account e-mail: <email>` (case-insensitive).
* Regex used: `(?i)Account\s+e-?mail:\s*(\S+)`
* Returns `(email, true)` if found, otherwise `("", false)`.

Example `mega-whoami` outputs this module expects to cope with:

* Logged in: `Account e-mail: me@example.com`
* Not logged in: `[...] cmd ERR  Not logged in.`
* Server starting: `MEGAcmd Server not running. Initiating in the background... Resuming session ...`

---

## Error handling & troubleshooting

* Command failures return an `error` plus the combined CLI output string; inspect the returned output to diagnose MEGAcmd-specific problems.
* Timeouts: each call uses a context with `m.Timeout` (uploads use `m.Timeout * 4`). Increase `Timeout` if uploads are large or the environment is slow.
* Localization & version differences: the code looks for English phrases (e.g. `"Folder already exists"`, `"Not logged in"`). If your MEGAcmd is configured in another language you may need to adapt the string checks.


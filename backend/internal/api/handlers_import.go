package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// journalBin returns the compiled import-bets binary. The runtime sets
// ODDSVAULT_BIN; the fallback makes `go run ./cmd/api` work from the repo.
func journalBin() (string, error) {
	if bin := os.Getenv("ODDSVAULT_BIN"); bin != "" {
		path := filepath.Join(bin, "import-bets")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	root, err := repoRoot()
	if err != nil {
		return "", err
	}
	path := filepath.Join(root, ".local", "bin", "import-bets")
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("import-bets binary not built; run make up")
}

func repoRoot() (string, error) {
	if root := os.Getenv("ODDSVAULT_ROOT"); root != "" {
		return root, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "scripts", "ocr-bets.py")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate repository root")
		}
		dir = parent
	}
}

func (s *Server) runImport(ctx context.Context, jsonPath string) (string, error) {
	bin, err := journalBin()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "-file", jsonPath)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		return text, fmt.Errorf("import failed: %w", err)
	}
	return text, nil
}

// handleJournalImportJSON accepts the same export schema as cmd/import-bets.
func (s *Server) handleJournalImportJSON(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 16<<20))
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_body", "could not read request body")
		return
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		writeError(w, r, http.StatusBadRequest, "empty_body", "send the JSON export")
		return
	}

	dir, err := os.MkdirTemp("", "oddsvault-import-*")
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "temp_failed", err.Error())
		return
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "bets.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		writeError(w, r, http.StatusInternalServerError, "temp_failed", err.Error())
		return
	}

	output, err := s.runImport(r.Context(), path)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"ok":     false,
			"error":  err.Error(),
			"output": output,
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "output": output})
}

// handleJournalImportScreenshots runs the vendored OCR parser over uploaded
// images, then imports the resulting export.
func (s *Server) handleJournalImportScreenshots(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_form", "expected multipart form data")
		return
	}
	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeError(w, r, http.StatusBadRequest, "no_files", "attach at least one screenshot")
		return
	}

	root, err := repoRoot()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "config_error", err.Error())
		return
	}

	dir, err := os.MkdirTemp("", "oddsvault-shots-*")
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, "temp_failed", err.Error())
		return
	}
	defer os.RemoveAll(dir)

	for index, header := range files {
		ext := strings.ToLower(filepath.Ext(header.Filename))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			writeError(w, r, http.StatusBadRequest, "unsupported_file", "use JPG or PNG screenshots")
			return
		}
		src, err := header.Open()
		if err != nil {
			writeError(w, r, http.StatusBadRequest, "upload_failed", err.Error())
			return
		}
		dstPath := filepath.Join(dir, fmt.Sprintf("%03d%s", index, ext))
		dst, err := os.Create(dstPath)
		if err != nil {
			src.Close()
			writeError(w, r, http.StatusInternalServerError, "temp_failed", err.Error())
			return
		}
		_, copyErr := io.Copy(dst, src)
		dst.Close()
		src.Close()
		if copyErr != nil {
			writeError(w, r, http.StatusInternalServerError, "upload_failed", copyErr.Error())
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	script := filepath.Join(root, "scripts", "ocr-bets.py")
	ocr := exec.CommandContext(ctx, "python3", script, dir)
	ocr.Env = append(os.Environ(), "OCR_CACHE="+filepath.Join(os.TempDir(), "oddsvault-ocr-cache"))
	var stderr strings.Builder
	ocr.Stderr = &stderr
	raw, err := ocr.Output()
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"ok":     false,
			"error":  "OCR failed",
			"output": strings.TrimSpace(stderr.String()),
		})
		return
	}

	jsonPath := filepath.Join(dir, "parsed.json")
	if err := os.WriteFile(jsonPath, raw, 0o600); err != nil {
		writeError(w, r, http.StatusInternalServerError, "temp_failed", err.Error())
		return
	}

	bets := countBets(raw)
	output, err := s.runImport(ctx, jsonPath)
	if err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"ok":     false,
			"error":  err.Error(),
			"output": output,
			"bets":   bets,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"bets":   bets,
		"output": output,
	})
}

// countBets is a cheap scan for the number of parsed slips, used only in the
// response summary.
func countBets(raw []byte) int {
	count := 0
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "\"external_id\"") {
			count++
		}
	}
	return count
}

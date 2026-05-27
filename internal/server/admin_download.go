package server

import (
	"archive/zip"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"openforge/internal/shared/profile"
)

// handleDownloadOffline creates a zip archive of the /data/offline directory
// and streams it to the client. Returns 404 if the directory doesn't exist.
func handleDownloadOffline(of *profile.OpenForge) http.HandlerFunc {
	const offlineDir = "/data/offline"

	return func(w http.ResponseWriter, r *http.Request) {
		of.FeatureFlags.RLock()
		enabled := of.FeatureFlags.DistributionArtifacts
		of.FeatureFlags.RUnlock()
		if !enabled {
			writeError(w, 404, "feature disabled")
			return
		}

		// Check if offline directory exists
		info, err := os.Stat(offlineDir)
		if os.IsNotExist(err) || !info.IsDir() {
			writeError(w, http.StatusNotFound, "offline deployment bundle not found - run generate.sh first")
			return
		}
		if err != nil {
			slog.Error("failed to check offline directory", "error", err)
			writeError(w, http.StatusInternalServerError, "failed to access offline bundle")
			return
		}

		// Set response headers for zip download
		filename := fmt.Sprintf("openforge-offline-%s.zip", time.Now().Format("2006-01-02-150405"))
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

		// Create zip writer
		zipWriter := zip.NewWriter(w)
		defer zipWriter.Close()

		// Walk the offline directory and add files to zip
		err = filepath.Walk(offlineDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Skip directories (they'll be created implicitly)
			if info.IsDir() {
				return nil
			}

			// Get relative path for zip entry
			relPath, err := filepath.Rel(offlineDir, path)
			if err != nil {
				return err
			}

			// Create zip entry
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			header.Name = relPath
			header.Method = zip.Deflate

			writer, err := zipWriter.CreateHeader(header)
			if err != nil {
				return err
			}

			// Open and copy file content
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(writer, file)
			return err
		})

		if err != nil {
			slog.Error("failed to create offline zip", "error", err)
			// Can't write error response since we already started streaming
			return
		}

		slog.Info("offline bundle streamed to client")
	}
}
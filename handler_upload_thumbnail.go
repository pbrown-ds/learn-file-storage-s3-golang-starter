package main

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	// Set a const maxMemory to 10MB
	const maxMemory = 10 << 20

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	// Get video metadata from the SQLite database.
	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get video", err)
		return
	}

	if video.ID == uuid.Nil {
		respondWithError(w, http.StatusNotFound, "Video not found", nil)
		return
	}

	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "You don't own this video", nil)
		return
	}

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse multipart form", err)
		return
	}

	// Get file data and file headers
	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't get thumbnail", err)
		return
	}
	defer file.Close()

	// Determine File Extension by image type
	mediaType, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Couldn't parse media type", err)
		return
	}

	var extension string
	switch mediaType {
	case "image/jpeg":
		extension = ".jpg"
	case "image/png":
		extension = ".png"
	default:
		respondWithError(w, http.StatusBadRequest, "Unsupported image type", nil)
		return
	}

	// Create unique file path
	filename := fmt.Sprintf("%s%s", videoID, extension)
	filepath := filepath.Join(cfg.assetsRoot, filename)

	// Create new file
	dst, err := os.Create(filepath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't create thumbnail file", err)
		return
	}
	defer dst.Close()

	// Copy contents from the `multipart.File` to the new file on disk
	_, err = io.Copy(dst, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't save thumbnail", err)
		return
	}

	thumbnail_url := fmt.Sprintf(
		"http://localhost:%s/assets/%s",
		cfg.port,
		filename,
	)

	video.ThumbnailURL = &thumbnail_url

	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

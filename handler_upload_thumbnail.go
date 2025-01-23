package main

import (
	"crypto/rand"
	"encoding/base64"
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


	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't parse form", err)
		return
	}

	file, fileHeader, err := r.FormFile("thumbnail")
	defer file.Close()
	if err != nil {
		respondWithError(w, http.DefaultMaxHeaderBytes, "Couldn't get thumbnail", err)
		return
	}
	

	mediaType := fileHeader.Header.Get("Content-Type")
	mediaType, _, err = mime.ParseMediaType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't parse mime type", err)
		return
	}
	if mediaType != "image/jpeg" && mediaType != "image/png" {
		respondWithError(w, http.StatusUnsupportedMediaType, "Not a jpeg or png", nil)
		return
	}
	extension, err := mime.ExtensionsByType(mediaType)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get the extenstion of the file", err)
		return
	}
	if len(extension) < 1 {
		respondWithError(w, http.StatusInternalServerError, "Couldn't get the extenstion of the file", nil)
		return
	}


	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't get video with matching id", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "UserId and video userid dont match", nil)
		return
	}
	slice := make([]byte, 32)
	_, err = rand.Read(slice)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't generate random sequence", err)
		return
	}
	urlEncoding := base64.RawURLEncoding.EncodeToString(slice)

	path := filepath.Join(cfg.assetsRoot, urlEncoding+extension[0])
	imageFile, err := os.Create(path)
	if err != nil {
		fmt.Println(path)
		respondWithError(w, http.StatusInternalServerError, "Couldn't create file", err)
		return
	}

	io.Copy(imageFile, file)
	thumbnailUrl := fmt.Sprintf("http://localhost:%s/%s", cfg.port, path)
	video.ThumbnailURL = &thumbnailUrl
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusNotModified, "Could not update video", nil)
		return
	}

	respondWithJSON(w, http.StatusOK, video)
}

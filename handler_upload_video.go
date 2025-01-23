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

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	http.MaxBytesReader(w, r.Body, 1 << 30)
	
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

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't find a video with that id", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Uploader id and user id do not match", nil)
		return
	}

	videoFile, fileHeader, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Couldn't get the video header", err)
		return
	}
	defer videoFile.Close()

	mediaType, _, err := mime.ParseMediaType(fileHeader.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't pase mime from header", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusUnsupportedMediaType, "Media type not mp4", nil)
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


	file, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not create temporary file on disk", err)
		return
	}
	defer os.Remove("tubely-upload.mp4")
	defer file.Close()

	_, err = io.Copy(file, videoFile)
	if err != nil {
		respondWithError(w, http.StatusInsufficientStorage, "Unable to copy to disk", err)
		return
	}

	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to seek to front of file", err)
		return
	}

	path, err := processVideoForFastStart(file.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to process video", err)
		return
	}
	processedFile, err := os.Open(path)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not open file", err)
		return
	}
	ratio, err := getVideoAspectRatio(file.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not get aspect ratio", err)
		return
	}
	ratioPrefix := "other"
	if ratio == "16:9" {
		ratioPrefix = "landscape"
	}
	if ratio == "9:16" {
		ratioPrefix = "portrait"
	}

	slice := make([]byte, 32)
	_, err = rand.Read(slice)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Couldn't generate random sequence", err)
		return
	}
	fileKey := base64.RawURLEncoding.EncodeToString(slice)+".mp4"
	fileKey = filepath.Join(ratioPrefix, fileKey)

	vidUrl := cfg.s3CfDistribution+"/"+fileKey
	video.VideoURL = &vidUrl
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to update video", err)
		return
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket: &cfg.s3Bucket,
		Key: &fileKey,
		Body: processedFile,
		ContentType: &mediaType,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not upload to s3 bucket", err)
		return
	}

	fmt.Printf("Uploaded Video: %s", *video.VideoURL)
	respondWithJSON(w, http.StatusAccepted, video)
}

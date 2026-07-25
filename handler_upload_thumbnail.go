package main

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

const maxMemory = 10 << 20

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

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not parse multiform request", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "not able to parse form file", err)
		return
	}
	defer file.Close()

	mediaType := header.Header.Get("Content-Type")

	// get the extension of the image from MIME type
	imageExtension := strings.Split(mediaType, "/")[1]

	imgPath, err := cfg.writeThumbnailFileToDisk(file, videoID, imageExtension)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not write image to disk", err)
		return
	}

	vidMetaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "user not authorized", err)
		return
	}

	thumbnailURL := fmt.Sprintf("http://localhost:%s/%s", cfg.port, imgPath)

	vidMetaData.ThumbnailURL = &thumbnailURL

	if err := cfg.db.UpdateVideo(vidMetaData); err != nil {
		respondWithError(w, http.StatusBadRequest, "could not update video", err)
		return
	}

	respondWithJSON(w, http.StatusOK, vidMetaData)
}

func (cfg *apiConfig) writeThumbnailFileToDisk(imageData multipart.File, videoID uuid.UUID, extension string) (string, error) {
	imagePath := fmt.Sprintf("/%s.%s", videoID, extension)

	imageAbsPath := filepath.Join(cfg.assetsRoot, imagePath)

	newFile, err := os.Create(imageAbsPath)
	if err != nil {
		return "", fmt.Errorf("could not create file at path %s", imageAbsPath)
	}

	written, err := io.Copy(newFile, imageData)
	if err != nil {
		return "", fmt.Errorf("could not copy multipart")
	}

	log.Printf("wrote %d bytes to disk at %s", written, imageAbsPath)

	return imageAbsPath, nil
}

package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

const uploadLimit = 1 << 30

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	http.MaxBytesReader(w, r.Body, uploadLimit)

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

	_, err = auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	vidMetaData, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "user not authorized", err)
		return
	}

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not parse multiform request", err)
		return
	}

	// Multipart processing of request
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "not able to parse form file", err)
		return
	}
	defer file.Close()

	// Validation of media type
	fileType := header.Header.Get("Content-type")
	mediaType, _, err := mime.ParseMediaType(fileType)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not get media mime type", err)
		return
	}

	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Content uploaded not in a video format", err)
		return
	}

	// Saving file to a temporary file

	tmpFile, err := os.CreateTemp("/tmp", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not create a temp file", err)
		return
	}

	tmpFilePath := tmpFile.Name()
	defer os.Remove(tmpFilePath)
	defer tmpFile.Close()

	writtenBytes, err := io.Copy(tmpFile, file)
	if err != nil {
		respondWithError(w, 500, "could not copy contents to tmp file", err)
		return
	}

	log.Printf("wrote %d bytes to file %s\n", writtenBytes, tmpFilePath)

	tmpFile.Seek(0, io.SeekStart)

	// Putting in S3

	// Get random 32 bit name

	randByte := make([]byte, 32)
	rand.Read(randByte)

	base64RandomName := base64.StdEncoding.EncodeToString(randByte) + ".mp4"

	putObjetInputs := s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &base64RandomName,
		Body:        tmpFile,
		ContentType: &mediaType,
	}

	_, err = cfg.s3Client.PutObject(r.Context(), &putObjetInputs)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "could not upload to s3", err)
		return
	}

	// Update video URL in db
	vidURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, base64RandomName)
	vidMetaData.VideoURL = &vidURL

	if err := cfg.db.UpdateVideo(vidMetaData); err != nil {
		respondWithError(w, http.StatusBadRequest, "could not upload video", err)
		return
	}

    log.Printf("video added to db with url: %s\n", *vidMetaData.VideoURL)

	respondWithJSON(w, http.StatusOK, vidMetaData)
}

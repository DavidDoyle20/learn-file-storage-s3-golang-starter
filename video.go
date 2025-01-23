package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
)

func getVideoAspectRatio(filepath string) (string, error) {
	type Metadata struct {
		Streams []struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"streams"`
	}
	execCmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filepath)

	var byteBuffer bytes.Buffer
	execCmd.Stdout = &byteBuffer
	if err := execCmd.Run(); err != nil {
		return "", fmt.Errorf("error running ffprobe: %v", err)
	}

	var meta Metadata
	err := json.Unmarshal(byteBuffer.Bytes(), &meta)
	if err != nil {
		return "", err
	}
	if len(meta.Streams) < 1 {
		return "", fmt.Errorf("no streams found")
	}

	width := meta.Streams[0].Width
	height := meta.Streams[0].Height

	gcd := findGCD(width, height)
	ratioString := fmt.Sprintf("%d:%d", width/gcd, height/gcd)
	ratio := float64(width)/float64(height)
	landscapeRatio := float64(16)/9
	portraitRatio := float64(9)/16

	if areAspectRatiosClose(ratio, landscapeRatio, 0.01) {
		ratioString = "16:9"
	}
	if areAspectRatiosClose(ratio, portraitRatio, 0.01) {
		ratioString = "9:16"
	}

	return ratioString, nil
}

func processVideoForFastStart(filepath string) (string, error) {
	outputPath := filepath+".processing"
	execCmd := exec.Command("ffmpeg", "-i", filepath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputPath)
	if err := execCmd.Run(); err != nil {
		return "", err
	}
	return outputPath, nil
}

/**
func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	presignClient := s3.NewPresignClient(s3Client)
	req, err := presignClient.PresignGetObject(context.Background(), &s3.GetObjectInput{
		Bucket: &bucket,
		Key: &key,
	}, 
	s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	if video.VideoURL == nil {
		return video, nil
	}
	// what is there is no commas?
	fmt.Println(video.VideoURL)
	args := strings.Split(*video.VideoURL, ",")
	fmt.Println("here!!!")
	if len(args) < 2 {
		return database.Video{}, fmt.Errorf("video %s not in the correct format", *video.VideoURL)
	}
	bucket := args[0]
	key := args[1]
	presignedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, 60 * time.Minute)
	if err != nil {
		return database.Video{}, err
	}
	video.VideoURL = &presignedURL
	return video, nil
}
	**/

func findGCD(a, b int) int {
    for b != 0 {
        a, b = b, a % b
    }
    return a
}

func areAspectRatiosClose(ratio1, ratio2, tolerance float64) bool {
	diff := math.Abs(ratio1 - ratio2)
	return diff <= tolerance * math.Max(ratio1, ratio2)
}
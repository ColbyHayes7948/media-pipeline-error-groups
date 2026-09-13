package main

import (
	"context"
	"fmt"
	"os"

	mediaerrors "example.com/media-pipeline-errors"
)

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "set INFRAI_API_KEY")
		os.Exit(2)
	}

	assetID := "asset-2026-08-06-001"
	stage := "audio-normalize"
	client := mediaerrors.NewClient(apiKey)
	result, err := client.CaptureException(context.Background(), mediaerrors.CaptureInput{
		Title:       "media transform failed",
		Message:     "audio stream has no decodable frames",
		Level:       "error",
		Fingerprint: []string{"transcode", stage},
		Exception:   "DecodeError: audio stream has no decodable frames",
		Context: map[string]string{
			"asset_id": assetID,
			"stage":    stage,
			"pipeline": "vod-transcode",
		},
	}, "capture:"+assetID+":"+stage)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Printf("captured media error: %v\n", result)
}

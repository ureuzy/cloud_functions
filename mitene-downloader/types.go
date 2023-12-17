package mitene_downloader

import (
	"cloud.google.com/go/storage"
	"context"
	"log"
	"time"
)

type MediaFiles []*MediaFile
type MediaFile struct {
	ID                 int       `json:"id"`
	UUID               string    `json:"uuid"`
	UserID             string    `json:"userId"`
	MediaType          string    `json:"mediaType"`
	OriginalHash       string    `json:"originalHash"`
	HasComment         bool      `json:"hasComment"`
	TookAt             time.Time `json:"tookAt"`
	AudienceType       string    `json:"audienceType"`
	MediaWidth         int       `json:"mediaWidth"`
	MediaHight         int       `json:"mediaHeight"`
	MediaOrientation   int       `json:"mediaOrientation"`
	Latitude           float32   `json:"latitude"`
	Longitude          float32   `json:"longitude"`
	MediaDeviceModel   string    `json:"mediaDeviceModel"`
	DeviceFilePath     string    `json:"deviceFilePath"`
	VideoDuration      int       `json:"videoDuration"`
	ContentType        string    `json:"contentType"`
	Origin             string    `json:"origin"`
	ThumbnailGenerated bool      `json:"thumbnailGenerated"`
	ExpiringURL        string    `json:"expiringUrl"`
	ExpiringThumbURL   string    `json:"expiringThumbUrl"`
}

type WrapperStorageClient struct {
	*storage.Client
	BucketName string
}

func (r *WrapperStorageClient) exists(ctx context.Context, path string) bool {
	attrs, err := r.Bucket(r.BucketName).Object(path).Attrs(ctx)
	if err != nil {
		log.Fatal(err)
		return false
	}
	if attrs == nil {
		return false
	}
	return true
}

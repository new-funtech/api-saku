package cleanup

import (
	"context"
	"log"
	"time"

	"github.com/ganiramadhan/ganipedia/backend/pkg/utils"
	"github.com/robfig/cron/v3"
)

type Service struct {
	cron *cron.Cron
}

func NewService() *Service {
	return &Service{
		cron: cron.New(),
	}
}

func (s *Service) Start() {
	_, err := s.cron.AddFunc("@hourly", func() {
		s.cleanupTempFiles()
	})
	if err != nil {
		log.Printf("Failed to register cleanup cron job: %v", err)
		return
	}

	s.cron.Start()
	log.Println("Cleanup service started - running hourly to remove temp files older than 24 hours")
}

func (s *Service) Stop() {
	s.cron.Stop()
	log.Println("Cleanup service stopped")
}

func (s *Service) cleanupTempFiles() {
	ctx := context.Background()

	oldFiles, err := utils.ListOldTempFiles(ctx, 24*time.Hour)
	if err != nil {
		log.Printf("Error listing old temp files: %v", err)
		return
	}

	if len(oldFiles) == 0 {
		log.Println("No old temp files to clean up")
		return
	}

	log.Printf("Found %d old temp files to clean up", len(oldFiles))

	deletedCount := 0
	for _, fileKey := range oldFiles {
		if err := utils.DeleteFileFromS3(ctx, fileKey); err != nil {
			log.Printf("Failed to delete %s: %v", fileKey, err)
		} else {
			deletedCount++
		}
	}

	log.Printf("Cleanup completed: deleted %d of %d temp files", deletedCount, len(oldFiles))
}

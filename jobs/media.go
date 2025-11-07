package jobs

import (
	"EventHunting/collections"
	"EventHunting/utils"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cloudinary/cloudinary-go/v2/api/admin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func deletedCommentMedias(publicUrlIDs []string) []error {
	var (
		batchSize = 100
		cld       = utils.GetCloudinary()
		maxRetry  = 3
		wg        sync.WaitGroup
	)

	var allErrors []error
	var mu sync.Mutex
	//Chia thành nhiều batch để xóa
	for i := 0; i < len(publicUrlIDs); i += batchSize {
		end := i + batchSize
		if end > len(publicUrlIDs) {
			end = len(publicUrlIDs)
		}

		batch := publicUrlIDs[i:end]
		params := admin.DeleteAssetsParams{
			PublicIDs: batch,
		}

		wg.Add(1)
		//Xóa ảnh trên cld
		go func(p admin.DeleteAssetsParams, batchIndex int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			var batchErr error
			for j := 0; j < maxRetry; j++ {
				_, batchErr = cld.Admin.DeleteAssets(ctx, p)
				if batchErr == nil {
					break
				}
				time.Sleep(500 * time.Millisecond)
			}

			if batchErr != nil {
				mu.Lock()
				allErrors = append(allErrors, fmt.Errorf("failed to delete batch (index %d): %w", batchIndex, batchErr))
				mu.Unlock()
			}
		}(params, i)
	}

	wg.Wait()

	if len(allErrors) > 0 {
		return allErrors
	}
	return nil
}

func DeletedMedias(collectionName string) []error {
	var (
		softDeleteDays = 30
		mediaEntry     = &collections.Media{}
		err            error
		filter         = bson.M{
			"deleted_at": bson.M{
				"$lte": time.Now().Add(-time.Duration(softDeleteDays) * 24 * time.Hour),
			},
			"collection_name": collectionName,
		}
		maxDBRetry = 5
	)
	publicUrlIDs := make([]string, 0)
	mediaIDs := make([]primitive.ObjectID, 0)
	//Lấy tất cả các media đã xóa mềm quá 30 ngày
	medias, err := mediaEntry.Find(filter)
	if err != nil {
		return []error{err}
	}
	//Thêm publicUrlIds, mediaIds vào danh sách cần xóa
	for _, media := range medias {
		publicUrlIDs = append(publicUrlIDs, media.PublicUrlId)
		mediaIDs = append(mediaIDs, media.ID)
	}
	//Xóa các media trên cld
	errs := deletedCommentMedias(publicUrlIDs)

	if len(errs) > 0 {
		return errs
	}
	//Xóa media trong db
	deleteFilter := bson.M{
		"_id": bson.M{
			"$in": mediaIDs,
		},
	}

	//Thực hiện retry để đảm bảm sẽ xóa media trong db
	for i := 0; i < maxDBRetry; i++ {
		err = mediaEntry.DeleteMany(deleteFilter)
		if err == nil {
			break
		}
		time.Sleep(time.Duration(i+1) * 2 * time.Second)
	}

	if err != nil {
		return []error{fmt.Errorf("Lỗi xóa ảnh trong Db: %w", err)}
	}

	return nil
}

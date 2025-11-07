package jobs

//func DeletedComment() error {
//	var (
//		softDeleteDays = 30
//		commentEntry = &collections.Comment{}
//		err           error
//		filter := bson.M{
//			"deleted_at" : bson.M{
//				"$lte": time.Now().Add(-time.Duration(softDeleteDays) * 24 * time.Hour),
//			},
//		}
//	)
//	publicUrlIDs := make([]string, 0)
//	comments, err := commentEntry.Find(filter)
//	for _, comment := range comments {
//		if len(comment.Medias) > 0 {
//			for _, media := range comment.Medias {
//				publicUrlIDs = append(publicUrlIDs, media.)
//			}
//		}
//	}
//}

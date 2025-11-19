package collections

import (
	"EventHunting/database"
	"EventHunting/utils"
	"context"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Blog struct {
	ID           primitive.ObjectID `bson:"_id" json:"id"`
	Title        string             `bson:"title" json:"title"`
	View         int                `bson:"view" json:"view"`
	ThumbnailUrl string             `bson:"thumbnail_url,omitempty" json:"thumbnail_url"`
	ThumbnailID  primitive.ObjectID `bson:"thumbnail_id,omitempty" json:"thumbnail_id"`

	Content     string `bson:"content"`
	ContentHtml string `bson:"content_html"`
	//PublicImgIds []string             `bson:"public_img_ids,omitempty" json:"public_img_ids,omitempty"`
	//Medias []struct {
	//	Type   consts.MediaFormat `bson:"type" json:"type"` // Image, Video
	//	Url    string             `bson:"url" json:"url"`
	//	Status consts.MediaStatus `bson:"status" json:"status"` // Process, Pending, Success, Error
	//} `bson:"medias" json:"medias"`
	MediaIDs []primitive.ObjectID `bson:"media_ids,omitempty" json:"media_ids,omitempty"`
	Medias   []Media              `bson:"-" json:"medias"`

	Comments      []Comment            `bson:"-" json:"comments"`
	TagIds        []primitive.ObjectID `bson:"tag_ids,omitempty" json:"tag_ids,omitempty"`
	IsEdit        bool                 `bson:"is_edit" json:"is_edit"`
	IsLockComment bool                 `bson:"is_lock_comment" json:"is_lock_comment"`

	Account      Account `bson:"-" json:"account,omitempty"`
	Tags         Tags    `bson:"-" json:"tags,omitempty"`
	CommentCount int     `bson:"-" json:"comment_count"`

	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	CreatedBy primitive.ObjectID `bson:"created_by" json:"created_by"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
	UpdatedBy primitive.ObjectID `bson:"updated_by" json:"updated_by"`
	DeletedAt time.Time          `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
	DeletedBy primitive.ObjectID `bson:"deleted_by,omitempty" json:"deleted_by,omitempty"`
}

type Blogs []Blog

func (u *Blog) getCollectionName() string {
	return "blogs"
}

func (u *Blog) Create(ctx context.Context) error {
	var (
		db  = database.GetDB()
		err error
	)
	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	// TODO: Thêm log
	if u.ID.IsZero() {
		u.ID = primitive.NewObjectID()
	}
	_, err = db.Collection(u.getCollectionName()).InsertOne(ctx, u)

	// TODO: Thêm vào redis nếu sử dụng cache-aside

	if err != nil {
		return err
	}
	return nil
}

func (u *Blog) First(ctx context.Context, filter bson.M, opts ...*options.FindOneOptions) error {
	var (
		db = database.GetDB()
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	err := db.Collection(u.getCollectionName()).FindOne(ctx, filter, opts...).Decode(u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Blog) FindById(ctx context.Context, id primitive.ObjectID) error {
	var (
		db     = database.GetDB()
		filter = bson.M{
			"_id":        id,
			"deleted_at": bson.M{"$exists": false},
		}
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	var blog Blog
	err := db.Collection(u.getCollectionName()).FindOne(ctx, filter).Decode(&blog)
	if err != nil {
		return err
	}
	return nil
}

func (u *Blog) Find(ctx context.Context, filter bson.M, opts ...*options.FindOptions) (Blogs, error) {
	var (
		db    = database.GetDB()
		blogs Blogs
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	if filter == nil {
		filter = bson.M{}
	}
	filter["deleted_at"] = bson.M{"$exists": false}

	cursor, err := db.Collection(u.getCollectionName()).Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &blogs); err != nil {
		return nil, err
	}

	if blogs == nil {
		blogs = []Blog{}
	}

	return blogs, nil
}

func (u *Blog) Update(ctx context.Context, filter bson.M, updateDoc bson.M, opts ...*options.UpdateOptions) error {
	var (
		db = database.GetDB()
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	if filter == nil {
		filter = bson.M{}
	}

	res, err := db.Collection(u.getCollectionName()).UpdateOne(ctx, filter, updateDoc, opts...)
	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
		return mongo.ErrNoDocuments
	}

	return nil
}

func (u *Blog) DeleteMany(ctx context.Context, filter bson.M) error {
	var (
		db  = database.GetDB()
		err error
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	_, err = db.Collection(u.getCollectionName()).DeleteMany(ctx, filter)
	if err != nil {
		return err
	}

	return nil
}

func (u *Blog) CountDocuments(ctx context.Context, filter bson.M) (int64, error) {
	var (
		db = database.GetDB()
	)

	if ctx == nil {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
	}

	if filter == nil {
		filter = bson.M{}
	}
	filter["deleted_at"] = bson.M{"$exists": false}

	count, err := db.Collection(u.getCollectionName()).CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (u *Blog) GetView() int {
	redisClient := database.GetRedisClient().Client
	coldCount := u.View
	var hotCount int

	if redisClient == nil {
		log.Println("Lỗi do redis nil!")
		return coldCount
	}

	redisKey := "views:blog:" + u.ID.Hex()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	hotViewStr, err := redisClient.Get(ctx, redisKey).Result()

	if err == redis.Nil {
		hotCount = 0
	} else if err != nil {
		log.Printf("Lỗi do redis %s: %v", u.ID.Hex(), err)
		return coldCount
	} else {
		hotCount, err = strconv.Atoi(hotViewStr)
		if err != nil {
			log.Printf("Corrupt view count in Redis for blog %s (value: %s): %v", u.ID.Hex(), hotViewStr, err)
			hotCount = 0
		}
	}

	return hotCount + coldCount
}

func getBlogViewRedisKey(blogID primitive.ObjectID) string {
	return "views:blog:" + blogID.Hex()
}

func getBlogUniqueKey(blogID primitive.ObjectID) string {
	today := time.Now().UTC().Format("2006-01-02")
	return "unique_views:blog:" + blogID.Hex() + ":" + today
}

func (u *Blog) IncrementBlogView(accountID string) {
	redisClient := database.GetRedisClient().Client
	if redisClient == nil {
		log.Println("Redis client nil")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	uniqueKey := getBlogUniqueKey(u.ID)
	// Thêm account vào Set.
	added, err := redisClient.SAdd(ctx, uniqueKey, accountID).Result()
	if err != nil {
		log.Printf("Lỗi khi add và sadd view trong blog %s: %v", u.ID.Hex(), err)
		return
	}

	redisClient.Expire(ctx, uniqueKey, 25*time.Hour)

	if added == 1 {
		err := redisClient.Incr(ctx, getBlogViewRedisKey(u.ID)).Err()
		if err != nil {
			log.Printf("Lỗi khi tăng view blog redis %s: %v", u.ID.Hex(), err)
		}
	}
}

func (u *Blog) Preload(entries *Blogs, properties ...string) error {
	for _, property := range properties {
		switch property {
		case "AccountFirst":
			var (
				filter = bson.M{
					"_id":        u.CreatedBy,
					"deleted_at": nil,
				}
				err error
			)
			err = u.Account.First(filter)
			if err != nil {
				if err != mongo.ErrNoDocuments {
					return err
				}
			}
		case "AccountFind":
			var (
				accountMap map[primitive.ObjectID]Account
			)
			accountMap = make(map[primitive.ObjectID]Account)
			accountIDs := utils.ExtractUniqueIDs[Blog](*entries, func(blog Blog) []primitive.ObjectID {
				result := []primitive.ObjectID{
					blog.CreatedBy,
				}
				return result
			})

			if len(accountIDs) == 0 {
				continue
			}
			filterAccount := bson.M{
				"_id": bson.M{
					"$in": accountIDs,
				},
			}

			accounts, err := u.Account.Find(filterAccount)
			if err != nil {
				return err
			}

			for _, a := range accounts {
				accountMap[a.ID] = a
			}
			for i := range *entries {
				(*entries)[i].Account = accountMap[(*entries)[i].CreatedBy]
			}

		//case "AccountFind":
		//	var (
		//		//accountMap map[primitive.ObjectID]Account
		//		account  Account
		//		accounts Accounts
		//	)
		//	start := time.Now()
		//	if len(entries) == 0 {
		//		// Không làm gì cả
		//	}
		//	// 1. Lưu id của người tạo vào map -> set
		//	accIdsSet := make(map[primitive.ObjectID]struct{})
		//	for _, blog := range entries {
		//		if !blog.CreatedBy.IsZero() {
		//			accIdsSet[blog.CreatedBy] = struct{}{}
		//		}
		//	}
		//	//utils.TotalFinishRequest("SET", u.getCollectionName(), "FIND", start2)
		//	// 3. chuyển set -> slice
		//	accIds := make([]primitive.ObjectID, len(accIdsSet))
		//	for accId := range accIdsSet {
		//		accIds = append(accIds, accId)
		//	}
		//	// 4. lấy danh sách accounts theo slice
		//	filterAccount := bson.M{
		//		"_id": bson.M{
		//			"$in": accIds,
		//		},
		//		"deleted_at": nil,
		//	}
		//	accounts, _ = account.Find(filterAccount)
		//	if len(accounts) == 0 {
		//		// Xử lý khi accounts = 0
		//	}
		//	// 5. Lấy thông tin account theo id account có trong set
		//	accountMap := make(map[primitive.ObjectID]Account, len(accounts))
		//	for _, acc := range accounts {
		//		accountMap[acc.ID] = acc
		//	}
		//	// 6. Merge vào trong response trả về cho client
		//	for i := range entries {
		//		if acc, ok := accountMap[entries[i].CreatedBy]; ok {
		//			entries[i].Account = acc
		//		}
		//	}
		//	utils.TotalFinishRequest("Total", u.getCollectionName(), "FIND", start)
		//	return nil
		//	//accountMap = make(map[primitive.ObjectID]Account)
		//	//accountIDs := utils.ExtractUniqueIDs[Blog](*entities, func(blog Blog) []primitive.ObjectID {
		//	//	result := []primitive.ObjectID{
		//	//		blog.CreatedBy,
		//	//	}
		//	//	return result
		//	//})
		//	//
		//	//filterAccount := bson.M{
		//	//	"_id": bson.M{
		//	//		"$in": accountIDs,
		//	//	},
		//	//}
		//	//
		//	//accounts, err := u.Account.Find(filterAccount)
		//	//if err != nil {
		//	//	return err
		//	//}
		//	//
		//	//for _, a := range accounts {
		//	//	accountMap[a.ID] = a
		//	//}
		//	//for i := range *entities {
		//	//	(*entities)[i].Account = accountMap[(*entities)[i].CreatedBy]
		//	//}

		case "TagFirst":
			if len(u.TagIds) == 0 {
				continue
			}

			tagEntry := &Tag{}
			tags, err := tagEntry.Find(bson.M{
				"_id": bson.M{"$in": u.TagIds},
				"deleted_at": bson.M{
					"$exists": false,
				},
			})
			if err != nil {
				return err
			}
			u.Tags = tags

		case "TagFind":
			tagIds := utils.ExtractUniqueIDs[Blog](*entries, func(blog Blog) []primitive.ObjectID {
				return blog.TagIds
			})

			if len(tagIds) == 0 {
				continue
			}
			var (
				tagEntry = &Tag{}
				tagsMap  = make(map[primitive.ObjectID]Tag)
			)
			tags, err := tagEntry.Find(bson.M{
				"_id": bson.M{"$in": tagIds},
				"deleted_at": bson.M{
					"$exists": false,
				},
			})
			if err != nil {
				return err
			}
			for _, tag := range tags {
				tagsMap[tag.ID] = tag
			}
			for i := range *entries {
				tagsRe := []Tag{}
				for _, tag := range (*entries)[i].TagIds {
					tagsRe = append(tagsRe, tagsMap[tag])
				}
				(*entries)[i].Tags = tagsRe
			}
		case "CommentCountFirst":
			var (
				commentEntry = &Comment{}
				filter       = bson.M{
					"document_id": u.ID,
					"deleted_at": bson.M{
						"$exists": false,
					},
				}
			)
			cmtCount, err := commentEntry.CountDocument(filter)
			if err != nil {
				return err
			}
			u.CommentCount = int(cmtCount)
		case "CommentCountFind":
			var (
				blogIds         = make([]primitive.ObjectID, 0)
				commentEntry    = &Comment{}
				blogCountCmtMap = make(map[primitive.ObjectID]int32)
			)
			//Lấy danh sách id từ blog entries
			for _, blog := range *entries {
				blogIds = append(blogIds, blog.ID)
			}

			if len(blogIds) == 0 {
				continue
			}
			//Lấy count cmt từ mỗi blog
			pipeline := []bson.M{
				{
					"$match": bson.M{
						"document_id": bson.M{"$in": blogIds},
						"deleted_at": bson.M{
							"$exists": false,
						},
					},
				},
				{
					"$group": bson.M{
						"_id":           "$document_id",
						"comment_count": bson.M{"$sum": 1},
					},
				},
			}
			blogCountCmt, err := commentEntry.Aggregation(pipeline)
			if err != nil {
				return err
			}
			for _, b := range blogCountCmt {
				id, ok1 := b["_id"].(primitive.ObjectID)
				cnt, ok2 := b["comment_count"].(int32)
				if ok1 && ok2 {
					blogCountCmtMap[id] = cnt
				}
			}
			for i := range *entries {
				if cnt, ok := blogCountCmtMap[(*entries)[i].ID]; ok {
					(*entries)[i].CommentCount = int(cnt)
				} else {
					(*entries)[i].CommentCount = 0
				}
			}
		case "MediaFirst":
			if len(u.MediaIDs) == 0 {
				continue
			}
			var (
				mediaEntry  = &Media{}
				mediaFilter = bson.M{
					"_id": bson.M{
						"$in": u.MediaIDs,
					},
				}
			)
			medias, _ := mediaEntry.Find(nil, mediaFilter)
			u.Medias = medias
		case "MediaFind":
			mediaIds := utils.ExtractUniqueIDs[Blog](*entries, func(blog Blog) []primitive.ObjectID {
				return blog.MediaIDs
			})
			if len(mediaIds) == 0 {
				continue
			}
			var (
				mediaEntry = &Media{}
				mediasMap  = make(map[primitive.ObjectID]Media)
			)
			medias, err := mediaEntry.Find(nil, bson.M{"_id": bson.M{"$in": mediaIds}})
			if err != nil {
				return err
			}
			for _, media := range medias {
				mediasMap[media.ID] = media
			}
			for i := range *entries {
				mediasRe := []Media{}
				for _, mediaID := range (*entries)[i].MediaIDs {
					mediasRe = append(mediasRe, mediasMap[mediaID])
				}
				(*entries)[i].Medias = mediasRe
			}
		case "CommentFirst":
			var (
				commentEntry  = &Comment{}
				commentFilter = bson.M{
					"document_id": u.ID,
					"parent_id": bson.M{
						"$exists": false,
					},
					"deleted_at": bson.M{
						"$exists": false,
					},
				}
			)
			comments, _ := commentEntry.Find(commentFilter)
			err := commentEntry.Preload(comments, "AccountFind", "MediaFind")

			if err != nil {
				return err
			}

			if len(comments) > 0 {
				u.Comments = comments
			}
		}
	}
	return nil
}

func (u *Blog) ParseEntry() bson.M {
	result := bson.M{
		"_id":             u.ID,
		"title":           u.Title,
		"view":            u.GetView(),
		"thumbnail_url":   u.ThumbnailUrl,
		"thumbnail_id":    u.ThumbnailID,
		"comment_count":   u.CommentCount,
		"content":         u.Content,
		"content_html":    u.ContentHtml,
		"medias":          u.Medias,
		"is_edit":         u.IsEdit,
		"is_lock_comment": u.IsLockComment,

		"tag_ids":    u.TagIds,
		"created_at": u.CreatedAt,
		"created_by": u.CreatedBy,
		"updated_at": u.UpdatedAt,
		"updated_by": u.UpdatedBy,
	}

	if u.Account.ID != primitive.NilObjectID {
		result["account"] = bson.M{
			"name":       u.Account.Name,
			"avatar_url": u.Account.AvatarUrl,
		}
	}

	if len(u.Tags) > 0 {
		tags := make([]bson.M, 0, len(u.Tags))
		for _, tag := range u.Tags {
			tags = append(tags, bson.M{
				"name": tag.Name,
				"slug": tag.Slug,
			},
			)
		}
		result["tags"] = tags
	}

	if len(u.Medias) > 0 {
		medias := []bson.M{}
		for _, media := range u.Medias {
			medias = append(medias, media.ParseEntry())
		}
		result["medias"] = medias
	}

	if len(u.Comments) > 0 {
		comments := []bson.M{}
		for _, comment := range u.Comments {
			comments = append(comments, comment.ParseEntry())
		}
		result["comments"] = comments
	}

	return result
}

package collections

import (
	"EventHunting/consts"
	"EventHunting/database"
	"EventHunting/utils"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Blog struct {
	ID                primitive.ObjectID `bson:"_id" json:"id"`
	Title             string             `bson:"title" json:"title"`
	View              int                `bson:"view" json:"view"`
	ThumbnailLink     string             `bson:"thumbnail_link,omitempty" json:"thumbnail_link"`
	ThumbnailPublicId string             `bson:"thumbnail_public_id,omitempty" json:"thumbnail_public_id"`

	Content     string `bson:"content"`
	ContentHtml string `bson:"content_html"`
	//PublicImgIds []string             `bson:"public_img_ids,omitempty" json:"public_img_ids,omitempty"`
	Medias []struct {
		Type   consts.MediaFormat `bson:"type" json:"type"` // Image, Video
		Url    string             `bson:"url" json:"url"`
		Status consts.MediaStatus `bson:"status" json:"status"` // Process, Pending, Success, Error
	} `bson:"medias" json:"medias"`

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
	DeletedAt time.Time          `bson:"deleted_at" json:"deleted_at"`
	DeletedBy primitive.ObjectID `bson:"deleted_by" json:"deleted_by"`
}

type Blogs []Blog

func (u *Blog) getCollectionName() string {
	return "blogs"
}

func (u *Blog) Create() error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		err         error
	)
	defer cancel()

	// TODO: Thêm log
	u.ID = primitive.NewObjectID()
	_, err = db.Collection(u.getCollectionName()).InsertOne(ctx, u)

	// TODO: Thêm vào redis nếu sử dụng cache-aside

	if err != nil {
		return err
	}
	return nil
}

func (u *Blog) First(filter bson.M, opts ...*options.FindOneOptions) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()

	err := db.Collection(u.getCollectionName()).FindOne(ctx, filter, opts...).Decode(u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Blog) FindById(id primitive.ObjectID) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		filter      = bson.M{
			"_id":        id,
			"deleted_at": bson.M{"$exists": false},
		}
	)
	defer cancel()

	var blog Blog
	err := db.Collection(u.getCollectionName()).FindOne(ctx, filter).Decode(&blog)
	if err != nil {
		return err
	}
	return nil
}

func (u *Blog) Find(filter bson.M, opts ...*options.FindOptions) (Blogs, error) {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		blogs       Blogs
	)
	defer cancel()
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

func (u *Blog) Update(filter bson.M, updateDoc bson.M, opts ...*options.UpdateOptions) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()

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

func (u *Blog) CountDocuments(filter bson.M) (int64, error) {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()
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
			tags, err := tagEntry.Find(bson.M{"_id": bson.M{"$in": u.TagIds}})
			if err != nil {
				return err
			}
			u.Tags = tags

		case "TagFind":
			tagIds := utils.ExtractUniqueIDs[Blog](*entries, func(blog Blog) []primitive.ObjectID {
				return blog.TagIds
			})
			var (
				tagEntry = &Tag{}
				tagsMap  = make(map[primitive.ObjectID]Tag)
			)
			tags, err := tagEntry.Find(bson.M{"_id": bson.M{"$in": tagIds}})
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
					"blog_id": u.ID,
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
			//Lấy count cmt từ mỗi blog
			pipeline := []bson.M{
				{
					"$match": bson.M{
						"blog_id": bson.M{"$in": blogIds},
						"deleted_at": bson.M{
							"$exists": false,
						},
					},
				},
				{
					"$group": bson.M{
						"_id":           "$blog_id",
						"comment_count": bson.M{"$sum": 1},
					},
				},
			}
			blogCountCmt, err := commentEntry.Aggregation(pipeline)
			if err != nil {
				return err
			}
			for _, b := range blogCountCmt {
				bObjectID, _ := b["_id"].(primitive.ObjectID)
				blogCountCmtMap[bObjectID] = b["comment_count"].(int32)
			}
			for i := range *entries {
				(*entries)[i].CommentCount = int(blogCountCmtMap[(*entries)[i].ID])
			}
		}
	}
	return nil
}

func (u *Blog) ParseEntry() bson.M {
	result := bson.M{
		"_id":                 u.ID,
		"title":               u.Title,
		"view":                u.View,
		"thumbnail_link":      u.ThumbnailLink,
		"thumbnail_public_id": u.ThumbnailPublicId,
		"comment_count":       u.CommentCount,
		"content":             u.Content,
		"content_html":        u.ContentHtml,
		//"public_img_ids":      u.PublicImgIds,
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

	return result
}

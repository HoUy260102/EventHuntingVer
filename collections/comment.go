package collections

import (
	"EventHunting/consts"
	"EventHunting/database"
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

//type React struct {
//	ID         primitive.ObjectID `bson:"_id" json:"id"`
//	ModuleName string             `bson:"module_name" json:"module_name"`
//	Total      int64              `bson:"total" json:"total"`
//}

// TypeComment: Blogs, Events, ...

//type Comment struct {
//	ID primitive.ObjectID `bson:"_id" json:"id"`
//
//	ParentID    primitive.ObjectID   `bson:"parent_id,omitempty" json:"parent_id,omitempty"`
//	AncestorIDs []primitive.ObjectID `bson:"ancestor_ids,omitempty" json:"ancestor_ids,omitempty"`
//	//Replies    []Comment          `bson:"replies,omitempty" json:"replies,omitempty"`
//	ReplyCount      int64 `bson:"reply_count" json:"reply_count"`
//	DescendantCount int64 `bson:"descendant_count" json:"descendant_count"`
//	// Hashtag, Mention
//	Content     string `bson:"content" json:"content"` // Ê ObjectID(...) thấy comment tao vip không
//	ContentHTML string `bson:"content_html" json:"content_html"`
//	IsEdit      bool   `bson:"is_edit" json:"is_edit"`
//	//TotalReact  int64  `bson:"-" json:"number_react"`
//
//	Medias []struct {
//		Type   consts.MediaFormat `bson:"type" json:"type"` // Image, Video
//		Url    string             `bson:"url" json:"url"`
//		Status consts.MediaStatus `bson:"status" json:"status"` // Process, Pending, Success, Error
//	} `bson:"medias" json:"medias"`
//
//	//HashTagIds []string             `bson:"hash_tag_ids" json:"hash_tag_ids"` // [HaNoi, Hue, HoNnayToiBuon, NhacBuon]
//	//MentionIds []primitive.ObjectID `bson:"mention_ids" json:"mention_ids"`
//
//	BlogID   primitive.ObjectID `bson:"blog_id" json:"blog_id"`
//	Account  Account            `bson:"-" json:"account,omitempty"`
//	Category consts.CommentType `bson:"category" json:"category"`
//
//	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
//	CreatedBy primitive.ObjectID `bson:"created_by" json:"created_by"`
//	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
//	UpdatedBy primitive.ObjectID `bson:"updated_by" json:"updated_by"`
//	DeletedAt time.Time          `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
//	DeletedBy primitive.ObjectID `bson:"deleted_by,omitempty" json:"deleted_by,omitempty"`
//}

type Comment struct {
	ID primitive.ObjectID `bson:"_id" json:"id"`

	ParentID   primitive.ObjectID `bson:"parent_id,omitempty" json:"parent_id,omitempty"`
	ReplyCount int64              `bson:"reply_count" json:"reply_count"`
	// Hashtag, Mention
	Content     string `bson:"content" json:"content"`
	ContentHTML string `bson:"content_html" json:"content_html"`
	IsEdit      bool   `bson:"is_edit" json:"is_edit"`
	//TotalReact  int64  `bson:"-" json:"number_react"`

	Medias []struct {
		Type   consts.MediaFormat `bson:"type" json:"type"` // Image, Video
		Url    string             `bson:"url" json:"url"`
		Status consts.MediaStatus `bson:"status" json:"status"` // Process, Pending, Success, Error
	} `bson:"medias" json:"medias"`

	BlogID   primitive.ObjectID `bson:"blog_id" json:"blog_id"`
	Account  Account            `bson:"-" json:"account,omitempty"`
	Category consts.CommentType `bson:"category" json:"category"`

	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	CreatedBy primitive.ObjectID `bson:"created_by" json:"created_by"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
	UpdatedBy primitive.ObjectID `bson:"updated_by" json:"updated_by"`
	DeletedAt time.Time          `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
	DeletedBy primitive.ObjectID `bson:"deleted_by,omitempty" json:"deleted_by,omitempty"`
}

type Comments []Comment

func (u *Comment) getCollectionName() string {
	return "comments"
}

func (u *Comment) First(filter bson.M, opts ...*options.FindOneOptions) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		err         error
	)
	defer cancel()
	err = db.Collection(u.getCollectionName()).FindOne(ctx, filter, opts...).Decode(u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Comment) Find(filter bson.M, opts ...*options.FindOptions) (Comments, error) {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		err         error
		result      Comments = []Comment{}
	)
	defer cancel()
	cursor, err := db.Collection(u.getCollectionName()).Find(ctx, filter, opts...)
	if err != nil {
		return nil, err
	}
	for cursor.Next(ctx) {
		var comment Comment
		err = cursor.Decode(&comment)
		if err != nil {
			break
		}
		result = append(result, comment)
	}
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (u *Comment) Create() error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		err         error
	)
	defer cancel()
	if u.ID.IsZero() {
		u.ID = primitive.NewObjectID()
	}
	_, err = db.Collection(u.getCollectionName()).InsertOne(ctx, u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Comment) Update(filter bson.M, update bson.M, opts ...*options.UpdateOptions) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		err         error
	)
	defer cancel()
	_, err = db.Collection(u.getCollectionName()).UpdateOne(ctx, filter, update, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (u *Comment) UpdateMany(filter bson.M, update bson.M, opts ...*options.UpdateOptions) (int64, error) {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		err         error
	)
	defer cancel()
	res, err := db.Collection(u.getCollectionName()).UpdateMany(ctx, filter, update, opts...)
	if err != nil {
		return 0, err
	}
	return res.MatchedCount, nil
}

func (u *Comment) CountDocument(filter bson.M) (int64, error) {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		err         error
	)
	defer cancel()
	res, err := db.Collection(u.getCollectionName()).CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}
	return res, nil
}

func (u *Comment) Aggregation(pipeline []bson.M) ([]bson.M, error) {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		db          = database.GetDB()
		err         error
	)
	defer cancel()

	cursor, err := db.Collection(u.getCollectionName()).Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("Error in aggregation: %v", err)
		return nil, err
	}

	var result []bson.M
	if err := cursor.All(context.TODO(), &result); err != nil {
		log.Printf("Error decoding aggregation result: %v", err)
		return nil, err
	}

	return result, nil
}

func (u *Comment) Preload(entries Comments, properties ...string) error {
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
				account  Account
				accounts Accounts
			)
			if len(entries) == 0 {
				// Không làm gì cả
			}
			// 1. Lưu id của người tạo vào map -> set
			accIdsSet := make(map[primitive.ObjectID]struct{})
			for _, blog := range entries {
				if !blog.CreatedBy.IsZero() {
					accIdsSet[blog.CreatedBy] = struct{}{}
				}
			}
			// 3. chuyển set -> slice
			accIds := make([]primitive.ObjectID, len(accIdsSet))
			for accId := range accIdsSet {
				accIds = append(accIds, accId)
			}
			// 4. lấy danh sách accounts theo slice
			filterAccount := bson.M{
				"_id": bson.M{
					"$in": accIds,
				},
				"deleted_at": nil,
			}
			accounts, _ = account.Find(filterAccount)
			if len(accounts) == 0 {
				// Xử lý khi accounts = 0
			}
			// 5. Lấy thông tin account theo id account có trong set
			accountMap := make(map[primitive.ObjectID]Account, len(accounts))
			for _, acc := range accounts {
				accountMap[acc.ID] = acc
			}
			// 6. Merge vào trong response trả về cho client
			for i := range entries {
				if acc, ok := accountMap[entries[i].CreatedBy]; ok {
					entries[i].Account = acc
				}
			}
			return nil
		}
	}
	return nil
}

func (u *Comment) ParseEntry() bson.M {
	result := bson.M{
		"_id":       u.ID,
		"parent_id": u.ParentID,
		//"ancestor_ids":     u.AncestorIDs,
		"reply_count": u.ReplyCount,
		//"descendant_count": u.DescendantCount,
		"content":      u.Content,
		"content_html": u.ContentHTML,
		"is_edit":      u.IsEdit,
		"medias":       u.Medias,
		"blog_id":      u.BlogID,
		"category":     u.Category,
		"created_at":   u.CreatedAt,
		"created_by":   u.CreatedBy,
		"updated_at":   u.UpdatedAt,
		"updated_by":   u.UpdatedBy,
	}
	if !u.Account.ID.IsZero() {
		result["account"] = bson.M{
			"name":       u.Account.Name,
			"avatar_url": u.Account.AvatarUrl,
		}
	}
	// Chỉ thêm các field optional nếu có giá trị
	if !u.DeletedAt.IsZero() {
		result["deleted_at"] = u.DeletedAt
	}
	if u.DeletedBy != primitive.NilObjectID {
		result["deleted_by"] = u.DeletedBy
	}
	if u.ParentID != primitive.NilObjectID {
		result["parent_id"] = u.ParentID
	}
	//if len(u.AncestorIDs) > 0 {
	//	result["ancestor_ids"] = u.AncestorIDs
	//}
	if len(u.Medias) > 0 {
		result["medias"] = u.Medias
	}

	return result
}

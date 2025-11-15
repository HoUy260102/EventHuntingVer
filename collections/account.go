package collections

import (
	"EventHunting/database"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Account struct {
	// Thông tin định danh cơ bản
	ID       primitive.ObjectID `bson:"_id" json:"_id"`
	Email    string             `bson:"email" json:"email"`
	Password string             `bson:"password" json:"password"`
	Name     string             `bson:"name" json:"name"`
	Phone    string             `bson:"phone" json:"phone"`

	Address     string `bson:"address,omitempty" json:"address,omitempty"`
	AvatarUrl   string `bson:"avatar_url,omitempty" json:"avatar_url,omitempty"`
	AvatarUrlId string `bson:"avatar_url_id,omitempty" json:"avatar_url_id,omitempty"`

	// Thông tin riêng cho từng loại người dùng
	OrganizerInfo *Organizer `bson:"organizer_info,omitempty" json:"organizer_info,omitempty"`
	UserInfo      *User      `bson:"user_info,omitempty" json:"user_info,omitempty"`

	// --- Trạng thái khóa tài khoản ---
	IsLocked    bool      `bson:"is_locked" json:"is_locked"`
	LockAt      time.Time `bson:"lock_at,omitempty" json:"lock_at,omitempty"`
	LockUtil    time.Time `bson:"lock_util,omitempty" json:"lock_util,omitempty"`
	LockMessage string    `bson:"lock_message,omitempty" json:"lock_message,omitempty"`

	// --- Xác minh tài khoản ---
	IsVerified        bool      `bson:"is_verified" json:"is_verified"`
	VerifiedAt        time.Time `bson:"verified_at,omitempty" json:"verified_at,omitempty"`
	VerifySignUpToken string    `bson:"verify_sign_up_token,omitempty" json:"verify_sign_up_token,omitempty"`

	// --- Sự kiện quan tâm ---
	InterestedEvent []primitive.ObjectID `bson:"interested_event,omitempty" json:"interested_event,omitempty"`

	// --- Thông tin vai trò (Role/Subrole) ---
	RoleId    primitive.ObjectID `bson:"role_id,omitempty" json:"role_id,omitempty"`
	SubroleId primitive.ObjectID `bson:"subrole_id,omitempty" json:"subrole_id,omitempty"`

	// --- Liên kết hệ thống ngoài ---
	Provider string `bson:"provider,omitempty" json:"provider,omitempty"`

	// --- Thông tin khôi phục và liên kết công khai ---
	ResetPasswordToken string `bson:"reset_password_token,omitempty" json:"reset_password_token,omitempty"`

	// --- Trạng thái hoạt động ---
	IsActive bool `bson:"is_active" json:"is_active"`

	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	CreatedBy primitive.ObjectID `bson:"created_by" json:"created_by"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
	UpdatedBy primitive.ObjectID `bson:"updated_by" json:"updated_by"`
	DeletedAt time.Time          `bson:"deleted_at,omitempty" json:"deleted_at,omitempty"`
	DeletedBy primitive.ObjectID `bson:"deleted_by,omitempty" json:"deleted_by,omitempty"`
}

type Organizer struct {
	Decription  string `json:"decription,omitempty" bson:"decription,omitempty"`
	WebsiteUrl  string `json:"website_url,omitempty" bson:"website_url,omitempty"`
	ContactName string `json:"contact_name,omitempty" bson:"contact_name,omitempty"`
	//CostInforByRole    *CostInforByRole `bson:"cost_infor_by_role,omitempty" json:"role_info,omitempty"`
}

type User struct {
	Dob    time.Time `bson:"dob,omitempty" json:"dob,omitempty"`
	IsMale bool      `bson:"is_male" json:"is_male"`
}

//type ReCostInfo struct {
//	StartDate  *time.Time `bson:"start_date,omitempty" json:"start_date,omitempty"`
//	ExpireDate *time.Time `bson:"expire_date,omitempty" json:"expire_date,omitempty"`
//	IsPaid     bool       `bson:"is_paid,omitempty" json:"is_paid,omitempty"`
//	PaidAt     *time.Time `bson:"paid_at,omitempty" json:"paid_at,omitempty"`
//}

type Accounts []Account

func (u *Account) getCollectionName() string {
	return "accounts"
}

func (u *Account) First(filter bson.M) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()
	err := db.Collection(u.getCollectionName()).FindOne(ctx, filter).Decode(u)

	if err != nil {
		return err
	}
	return nil
}

func (u *Account) FindById(id primitive.ObjectID) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()

	err := db.Collection(u.getCollectionName()).FindOne(ctx, bson.M{"_id": id}).Decode(u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Account) Find(filter bson.M, opts ...*options.FindOptions) (Accounts, error) {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		accounts    Accounts
	)
	defer cancel()

	cursor, err := db.Collection(u.getCollectionName()).Find(ctx, filter, opts...)
	if err != nil {
		return accounts, err
	}
	defer cursor.Close(ctx)

	err = cursor.All(ctx, &accounts)
	if err != nil {
		return accounts, err
	}

	return accounts, nil
}

func (u *Account) FindOneAndUpdate(filter bson.M, update bson.M, opts ...*options.FindOneAndUpdateOptions) (Account, error) {

	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
		account     Account
	)
	defer cancel()

	after := options.FindOneAndUpdate().SetReturnDocument(options.After)
	opts = append(opts, after)

	err := db.Collection(u.getCollectionName()).FindOneAndUpdate(ctx, filter, update, opts...).Decode(&account)
	if err != nil {
		return Account{}, err
	}
	return account, nil
}

func (u *Account) Create() error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()

	u.ID = primitive.NewObjectID()
	_, err := db.Collection(u.getCollectionName()).InsertOne(ctx, u)
	if err != nil {
		return err
	}
	return nil
}

func (u *Account) Update(filter bson.M, update bson.M) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()
	res, err := db.Collection(u.getCollectionName()).UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if res.MatchedCount == 0 {
		return fmt.Errorf("Không tìm thấy document để update: %w", mongo.ErrNoDocuments)
	}
	return nil
}

func (u *Account) UpdateMany(filter bson.M, update bson.M) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()

	res, err := db.Collection(u.getCollectionName()).UpdateMany(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return fmt.Errorf("Không tìm thấy document để update: %w", mongo.ErrNoDocuments)
	}
	return nil
}

func (u *Account) Delete(filter bson.M, opts ...*options.DeleteOptions) error {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()

	_, err := db.Collection(u.getCollectionName()).DeleteOne(ctx, filter, opts...)
	if err != nil {
		return err
	}
	return nil
}

func (u *Account) DeleteMany(filter bson.M, opts ...*options.DeleteOptions) (int64, error) {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()

	res, err := db.Collection(u.getCollectionName()).DeleteMany(ctx, filter, opts...)
	if err != nil {
		return 0, err
	}

	if res.DeletedCount == 0 {
		return 0, mongo.ErrNoDocuments
	}
	return res.DeletedCount, nil
}

func (u *Account) CountDocuments(filter bson.M) (int64, error) {
	var (
		ctx, cancel = context.WithTimeout(context.Background(), 2*time.Second)
		db          = database.GetDB()
	)
	defer cancel()

	count, err := db.Collection(u.getCollectionName()).CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (a *Account) ParseEntry() bson.M {
	result := bson.M{
		"id":                   a.ID,
		"email":                a.Email,
		"password":             a.Password,
		"name":                 a.Name,
		"phone":                a.Phone,
		"address":              a.Address,
		"avatar_url":           a.AvatarUrl,
		"avatar_url_id":        a.AvatarUrlId,
		"organizer_info":       a.OrganizerInfo,
		"user_info":            a.UserInfo,
		"is_locked":            a.IsLocked,
		"lock_at":              a.LockAt,
		"lock_util":            a.LockUtil,
		"lock_message":         a.LockMessage,
		"is_verified":          a.IsVerified,
		"verified_at":          a.VerifiedAt,
		"verify_sign_up_token": a.VerifySignUpToken,
		"interested_event":     a.InterestedEvent,
		"role_id":              a.RoleId,
		"subrole_id":           a.SubroleId,
		"provider":             a.Provider,
		"reset_password_token": a.ResetPasswordToken,
		"is_active":            a.IsActive,
		"created_at":           a.CreatedAt,
		"created_by":           a.CreatedBy,
		"updated_at":           a.UpdatedAt,
		"updated_by":           a.UpdatedBy,
		"deleted_at":           a.DeletedAt,
		"deleted_by":           a.DeletedBy,
	}
	return result
}

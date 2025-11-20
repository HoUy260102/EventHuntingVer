package controllers

import (
	"EventHunting/collections"
	"EventHunting/consts"
	"EventHunting/database"
	"EventHunting/dto"
	"EventHunting/utils"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func CreateEvent(c *gin.Context) {
	var (
		req           dto.EventCreateReq
		mediaEntry    = &collections.Media{}
		provinceEntry = &collections.Province{}
		err           error
	)
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "Lỗi do bind dữ liệu", err.Error())
		return
	}

	//Validate dữ liệu
	if validateErrs := utils.ValidateEventCreateReq(req); len(validateErrs) > 0 {
		utils.ResponseError(c, http.StatusBadRequest, "", strings.Join(validateErrs, ", "))
		return
	}

	//Lấy ID của người tạo event
	creatorID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}

	newEvent := collections.Event{
		Name:             req.Name,
		EventInfo:        req.EventInfo,
		EventInfoHtml:    req.EventInfoHtml,
		View:             0,
		MaxTicketPerUser: req.MaxTicketPerUser,
		Active:           true,
		EventTime: struct {
			StartDate time.Time `bson:"start_date" json:"start_date"`
			EndDate   time.Time `bson:"end_date" json:"end_date"`
			StartTime string    `bson:"start_time" json:"start_time"`
			EndTime   string    `bson:"end_time" json:"end_time"`
		}{StartDate: req.EventTime.StartDate, EndDate: req.EventTime.EndDate, StartTime: req.EventTime.StartTime, EndTime: req.EventTime.EndTime},
		EventLocation: struct {
			Name    string `bson:"name" json:"name"`
			Address string `bson:"address" json:"address"`
			MapURL  string `bson:"map_url,omitempty" json:"map_url,omitempty"`
		}{Name: req.EventLocation.Name, Address: req.EventLocation.Address, MapURL: req.EventLocation.MapURL},
		NumberOfParticipants: 0,
		IsEdit:               false,
		ProvinceId:           req.ProvinceID,
		CreatedAt:            time.Now(),
		CreatedBy:            creatorID,
		UpdatedAt:            time.Now(),
		UpdatedBy:            creatorID,
	}

	//Kiểm tra trường không bắt buộc
	mediaIDs := []primitive.ObjectID{}
	if req.ThumbnailId != nil {
		err = mediaEntry.First(nil, bson.M{
			"_id": req.ThumbnailId,
		})
		switch {
		case err == nil:
			if mediaEntry.Type != consts.MEDIA_IMAGE {
				utils.ResponseError(c, http.StatusBadRequest, "", "Thumbnail phải là ảnh")
				return
			}
			newEvent.ThumbnailUrl = mediaEntry.Url
			newEvent.ThumbnailId = *req.ThumbnailId
		case errors.Is(err, mongo.ErrNoDocuments):
			utils.ResponseError(c, http.StatusBadRequest, "", fmt.Errorf("Không tìm thấy thumbnail: %v", err.Error()))
			return
		default:
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
			return
		}
	}

	//Logic kiểm tra ảnh
	if req.MediaIDs != nil {
		if len(*req.MediaIDs) > 0 {
			existedMediaIDMap := make(map[primitive.ObjectID]struct{})
			validMediaFilter := bson.M{
				"_id": bson.M{
					"$in": req.MediaIDs,
				},
			}
			medias, err := mediaEntry.Find(nil, validMediaFilter)
			for _, media := range medias {
				existedMediaIDMap[media.ID] = struct{}{}
			}
			if err != nil {
				utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
				return
			}
			invalidMeida := []string{}
			for _, mediaID := range *req.MediaIDs {
				if _, ok := existedMediaIDMap[mediaID]; !ok {
					invalidMeida = append(invalidMeida, mediaID.Hex())
				}
			}
			if len(medias) != len(*req.MediaIDs) {
				utils.ResponseError(c, http.StatusBadRequest, "", fmt.Errorf("MediaIDs [%s] không hợp lệ", strings.Join(invalidMeida, ", ")).Error())
				return
			}
		}
		newEvent.MediaIDs = *req.MediaIDs
		mediaIDs = append(mediaIDs, (*req.MediaIDs)...)
	}
	if req.TopicIDs != nil && len(*req.TopicIDs) > 0 {
		newEvent.TopicIDs = *req.TopicIDs
	}

	//Logic check province
	provinceFilter := bson.M{
		"_id": req.ProvinceID,
	}
	err = provinceEntry.First(nil, provinceFilter)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			utils.ResponseError(c, http.StatusBadRequest, "Lỗi do không tìm thấy province!", err.Error())
			return
		}
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}

	db := database.GetDB()
	session, err := db.Client().StartSession()
	if err != nil {
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi khi bắt đầu session!",
			Error:   err.Error(),
		})
		return
	}
	defer session.EndSession(c)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = mongo.WithSession(ctx, session, func(sessCtx mongo.SessionContext) error {
		_, err := sessCtx.WithTransaction(sessCtx, func(sessCtx mongo.SessionContext) (interface{}, error) {
			// Update media nếu có
			if len(mediaIDs) > 0 {
				mediaFilter := bson.M{"_id": bson.M{"$in": mediaIDs}, "status": "PENDING"}
				mediaUpdate := bson.M{"$set": bson.M{"status": "SUCCESS"}}
				const maxRetry = 3
				for i := 0; i < maxRetry; i++ {
					if err := mediaEntry.UpdateMany(sessCtx, mediaFilter, mediaUpdate); err != nil {
						if !mongo.IsTimeout(err) && !mongo.IsNetworkError(err) {
							return nil, err
						}
						time.Sleep(time.Duration(100*(1<<i)) * time.Millisecond)
					} else {
						break
					}
				}
			}

			// Tạo blog mới
			if err := newEvent.Create(sessCtx); err != nil {
				return nil, err
			}
			return nil, nil
		})
		return err
	})

	// Response cuối cùng dùng switch
	switch {
	case err == nil:
		_ = newEvent.Preload(nil, "AccountFirst", "MediaFirst", "TopicFirst", "ProvinceFirst")
		c.JSON(http.StatusCreated, dto.ApiResponse{
			Status:  http.StatusCreated,
			Message: "Sự kiện đã được tạo.",
			Data:    utils.PrettyJSON(newEvent.ParseEntry()),
		})
	default:
		c.JSON(http.StatusInternalServerError, dto.ApiResponse{
			Status:  http.StatusInternalServerError,
			Message: "Lỗi hệ thống!",
			Error:   err.Error(),
		})
	}
}

func UpdateEvent(c *gin.Context) {
	var (
		req           dto.EventUpdateReq
		mediaEntry    = &collections.Media{}
		provinceEntry = &collections.Province{}
		err           error
	)

	//Lấy Event ID từ URL param
	eventIDStr := c.Param("id")
	eventID, err := primitive.ObjectIDFromHex(eventIDStr)
	if err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "Event ID không hợp lệ", err.Error())
		return
	}

	// Bind JSON
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "Lỗi do bind dữ liệu", err.Error())
		return
	}

	// Validate DTO
	if validateErrs := utils.ValidateEventUpdateReq(req); len(validateErrs) > 0 {
		utils.ResponseError(c, http.StatusBadRequest, "", strings.Join(validateErrs, ", "))
		return
	}

	// Lấy ID người cập nhật
	updaterID, ok := utils.GetAccountID(c)
	if !ok {
		return
	}
	//Lấy roles tù context
	roles, err := utils.GetRoles(c)
	if err != nil {
		return
	}

	// Lấy sự kiện hiện tại để kiểm tra quyền
	eventEntry := &collections.Event{}
	err = eventEntry.First(nil, bson.M{"_id": eventID})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			utils.ResponseError(c, http.StatusNotFound, "", "Không tìm thấy sự kiện")
			return
		}
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi hệ thống khi tìm sự kiện!", err.Error())
		return
	}

	//Kiểm tra logic nghiệp vụ
	if validateErrs := validateEventUpdateBusiness(req, eventEntry); len(validateErrs) > 0 {
		utils.ResponseError(c, http.StatusBadRequest, "", strings.Join(validateErrs, ", "))
		return
	}

	// KIỂM TRA QUYỀN: Chỉ chủ sở hữu mới được sửa
	if !utils.CanModifyResource(eventEntry.CreatedBy, updaterID, roles) {
		utils.ResponseError(c, http.StatusForbidden, "", "Bạn không có quyền cập nhật sự kiện này")
		return
	}

	updateFields := bson.M{}
	mediaIDsToUpdate := []primitive.ObjectID{}
	mediaIDsToDelete := []primitive.ObjectID{}

	//Update
	if req.Name != nil {
		updateFields["name"] = *req.Name
	}
	if req.EventInfo != nil {
		updateFields["event_info"] = *req.EventInfo
	}
	if req.EventInfoHtml != nil {
		updateFields["event_info_html"] = *req.EventInfoHtml
	}
	if req.Price != nil {
		updateFields["price"] = *req.Price
	}
	if req.Active != nil {
		updateFields["active"] = *req.Active
	}
	if req.MaxParticipants != nil {
		updateFields["max_participants"] = *req.MaxParticipants
	}
	if req.TopicIDs != nil {
		updateFields["topic_ids"] = *req.TopicIDs
	}

	if req.EventTime != nil {
		if req.EventTime.StartDate != nil {
			updateFields["event_time.start_date"] = *req.EventTime.StartDate
		}
		if req.EventTime.EndDate != nil {
			updateFields["event_time.end_date"] = *req.EventTime.EndDate
		}
		if req.EventTime.StartTime != nil {
			updateFields["event_time.start_time"] = *req.EventTime.StartTime
		}
		if req.EventTime.EndTime != nil {
			updateFields["event_time.end_time"] = *req.EventTime.EndTime
		}
	}

	if req.EventLocation != nil {
		if req.EventLocation.Name != nil {
			updateFields["event_location.name"] = *req.EventLocation.Name
		}
		if req.EventLocation.Address != nil {
			updateFields["event_location.address"] = *req.EventLocation.Address
		}
		if req.EventLocation.MapURL != nil {
			updateFields["event_location.map_url"] = *req.EventLocation.MapURL
		}
	}
	// Xử lý tỉnh
	if req.ProvinceID != nil {
		//Logic check province
		provinceFilter := bson.M{
			"_id": req.ProvinceID,
		}
		err = provinceEntry.First(nil, provinceFilter)
		switch {
		case err == nil:
			updateFields["province_id"] = *req.ProvinceID
		case errors.Is(err, mongo.ErrNoDocuments):
			utils.ResponseError(c, http.StatusBadRequest, "Lỗi do không tìm thấy province!", err.Error())
			return
		default:
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
			return
		}
	}

	// Xử lý logic Media
	// Xử lý Thumbnail
	if req.ThumbnailId != nil {
		if *req.ThumbnailId != eventEntry.ThumbnailId {
			err = mediaEntry.First(nil, bson.M{"_id": req.ThumbnailId})
			switch {
			case err == nil:
				if mediaEntry.Type != consts.MEDIA_IMAGE {
					utils.ResponseError(c, http.StatusBadRequest, "", "Thumbnail phải là ảnh!")
					return
				}
				updateFields["thumbnail_url"] = mediaEntry.Url
				updateFields["thumbnail_id"] = *req.ThumbnailId
				mediaIDsToUpdate = append(mediaIDsToUpdate, *req.ThumbnailId)
				mediaIDsToDelete = append(mediaIDsToDelete, eventEntry.ThumbnailId)
			case errors.Is(err, mongo.ErrNoDocuments):
				utils.ResponseError(c, http.StatusNotFound, "", fmt.Errorf("Không tìm thấy thumbnail: %v", req.ThumbnailId))
				return
			default:
				utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
				return
			}
		}
	}

	// Xử lý MediaIDs
	if req.MediaIDs != nil {
		updateFields["media_ids"] = *req.MediaIDs
		oldMediaMap := make(map[primitive.ObjectID]bool)
		newMediaMap := make(map[primitive.ObjectID]bool)

		// Thêm danh sách media cũ
		for _, id := range eventEntry.MediaIDs {
			oldMediaMap[id] = true
		}

		for _, id := range *req.MediaIDs {
			newMediaMap[id] = true
		}

		for newID := range newMediaMap {
			if !oldMediaMap[newID] {
				mediaIDsToUpdate = append(mediaIDsToUpdate, newID)
			}
		}

		for oldID := range oldMediaMap {
			if !newMediaMap[oldID] {
				mediaIDsToDelete = append(mediaIDsToDelete, oldID)
			}
		}
	}
	updateFields["is_edit"] = true

	// Bắt đầu xử lý transaction cho việc cập nhật
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	db := database.GetDB()
	session, err := db.Client().StartSession()
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi khi bắt đầu session", err.Error())
		return
	}
	defer session.EndSession(ctx)

	err = mongo.WithSession(ctx, session, func(sessCtx mongo.SessionContext) error {

		// Cập nhật trạng thái media
		if len(mediaIDsToUpdate) > 0 {
			mediaFilter := bson.M{"_id": bson.M{"$in": mediaIDsToUpdate}, "status": "PENDING"}
			mediaUpdate := bson.M{"$set": bson.M{"status": "SUCCESS"}}

			if err := mediaEntry.UpdateMany(sessCtx, mediaFilter, mediaUpdate); err != nil {
				return err
			}
		}

		if len(mediaIDsToDelete) > 0 {
			mediaFilter := bson.M{"_id": bson.M{"$in": mediaIDsToDelete}, "status": "SUCCESS"}
			mediaUpdate := bson.M{"$set": bson.M{"status": "DELETED"}}

			if err := mediaEntry.UpdateMany(sessCtx, mediaFilter, mediaUpdate); err != nil {
				return err
			}
		}

		updateFields["updated_at"] = time.Now()
		updateFields["updated_by"] = updaterID

		finalUpdateDoc := bson.M{"$set": updateFields}
		filter := bson.M{"_id": eventID}

		return eventEntry.Update(sessCtx, filter, finalUpdateDoc)
	})

	// Xử lý kết quả transaction
	switch {
	case err == nil:
		freshCtx, freshCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer freshCancel()
		_ = eventEntry.First(freshCtx, bson.M{"_id": eventID})
		//TODO: Gửi thông báo tại đây
		if req.EventTime != nil || req.EventLocation != nil {

		}

		utils.ResponseSuccess(c, http.StatusOK, "", eventEntry, nil)
	case errors.Is(err, mongo.ErrNoDocuments):
		utils.ResponseError(c, http.StatusBadRequest, "", "Một số media không tìm thấy")
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
	}
}

func GetEvent(c *gin.Context) {
	var (
		eventEntry = &collections.Event{}
		err        error
	)
	id := c.Param("id")
	IDConvert, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		utils.ResponseError(c, http.StatusBadRequest, "Lỗi do convert string sang ObjectID", err.Error())
		return
	}

	viewerID, exists := c.Get("account_id")
	viewerIDStr := c.ClientIP()
	if exists {
		viewerIDStr = viewerID.(string)
	}

	baseFilter := bson.M{
		"_id": IDConvert,
		"deletd_at": bson.M{
			"$exists": false,
		},
	}
	err = eventEntry.First(nil, baseFilter)
	switch {
	case err == nil:
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			eventEntry.IncrementEventView(viewerIDStr)
		}()
		preLoadErr := eventEntry.Preload(nil, "AccountFirst", "MediaFirst", "TopicFirst", "CommentFirst", "CommentCountFirst", "ProvinceFirst", "TicketTypeFirst")
		if preLoadErr != nil {
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", preLoadErr.Error())
			return
		}
		wg.Wait()
		utils.ResponseSuccess(c, http.StatusOK, "", utils.PrettyJSON(eventEntry.ParseEntry()), nil)
	case errors.Is(err, mongo.ErrNoDocuments):
		utils.ResponseError(c, http.StatusNotFound, "", err.Error())
	default:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
	}
}

func GetListEvents(c *gin.Context) {
	var (
		eventEntry = &collections.Event{}
	)
	queryMap := c.Request.URL.Query()
	pagination := dto.GetPagination(c, "primary")
	filterSearch := utils.BuildEventSearchFilter(queryMap)

	//Set skip, sort
	skip := (pagination.Page - 1) * pagination.Length
	sortFilter := utils.BuildSortFilter(queryMap)
	opts := options.Find()
	opts.SetSort(sortFilter)
	opts.SetSkip(int64(skip))
	opts.SetLimit(int64(pagination.Length))

	//Total docs
	totalDocs, err := eventEntry.CountDocuments(nil, filterSearch)
	if err != nil {
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
		return
	}

	pagination.TotalDocs = int(totalDocs)
	pagination.BuildPagination()

	eventsRes, err := eventEntry.Find(nil, filterSearch, opts)
	switch {
	case err == nil && len(eventsRes) > 0:
		preloadErr := eventEntry.Preload(eventsRes, "AccountFind", "MediaFind", "CommentCountFind", "TopicFind", "ProvinceFind")
		if preloadErr != nil {
			utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", preloadErr.Error())
			return
		}
		utils.ResponseSuccess(c, http.StatusOK, "", eventsRes, &pagination)
	case err == nil && len(eventsRes) == 0:
		utils.ResponseError(c, http.StatusNotFound, "", nil)
	case err != nil:
		utils.ResponseError(c, http.StatusInternalServerError, "Lỗi do hệ thống!", err.Error())
	}
}

const (
	timeFormat = "15:04"
)

// validate nghiệp vụ
func validateEventUpdateBusiness(req dto.EventUpdateReq, oldEvent *collections.Event) []string {
	var errs []string

	var effectiveStartDate, effectiveEndDate time.Time
	var effectiveStartTimeStr, effectiveEndTimeStr string

	effectiveStartDate = oldEvent.EventTime.StartDate
	effectiveEndDate = oldEvent.EventTime.EndDate
	effectiveStartTimeStr = oldEvent.EventTime.StartTime
	effectiveEndTimeStr = oldEvent.EventTime.EndTime

	if req.EventTime != nil {
		if req.EventTime.StartDate != nil {
			effectiveStartDate = *req.EventTime.StartDate
		}
		if req.EventTime.EndDate != nil {
			effectiveEndDate = *req.EventTime.EndDate
		}
		if req.EventTime.StartTime != nil {
			effectiveStartTimeStr = *req.EventTime.StartTime
		}
		if req.EventTime.EndTime != nil {
			effectiveEndTimeStr = *req.EventTime.EndTime
		}
	}

	if !effectiveStartDate.IsZero() && !effectiveEndDate.IsZero() {
		if effectiveEndDate.Before(effectiveStartDate) {
			errs = append(errs, "Ngày kết thúc không thể trước ngày bắt đầu")
		}
	}

	if effectiveStartTimeStr != "" && effectiveEndTimeStr != "" {
		startTime, err1 := time.Parse(timeFormat, effectiveStartTimeStr)
		endTime, err2 := time.Parse(timeFormat, effectiveEndTimeStr)

		if err1 == nil && err2 == nil {
			if endTime.Before(startTime) {
				errs = append(errs, "Giờ kết thúc không thể trước giờ bắt đầu")
			}
		}
	}

	return errs
}

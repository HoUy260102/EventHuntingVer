package utils

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func BuildAccountSearchFilter(params map[string][]string) bson.M {
	filter := bson.M{}
	andConditions := []bson.M{}

	// Tìm theo keyword
	if keywords, ok := params["keyword"]; ok && len(keywords) > 0 && keywords[0] != "" {
		keyword := regexp.QuoteMeta(keywords[0])
		or := []bson.M{
			{"name": bson.M{"$regex": keyword, "$options": "i"}},
			{"email": bson.M{"$regex": keyword, "$options": "i"}},
		}
		andConditions = append(andConditions, bson.M{"$or": or})
	}

	// address
	if v, ok := params["address"]; ok && len(v) > 0 && v[0] != "" {
		andConditions = append(andConditions, bson.M{
			"address": bson.M{"$regex": v[0], "$options": "i"},
		})
	}

	// phone
	if phone, ok := params["phone"]; ok && len(phone) > 0 && phone[0] != "" {
		andConditions = append(andConditions, bson.M{
			"phone": bson.M{"$regex": phone[0], "$options": "i"},
		})
	}

	// role_id
	if roleIds, ok := params["role_id"]; ok && len(roleIds) > 0 {
		var objectIDs []primitive.ObjectID
		for _, rid := range roleIds {
			if rid == "" {
				continue
			}
			id, err := primitive.ObjectIDFromHex(rid)
			if err == nil {
				objectIDs = append(objectIDs, id)
			}
		}
		if len(objectIDs) > 0 {
			andConditions = append(andConditions, bson.M{"role_id": bson.M{"$in": objectIDs}})
		}
	}

	// created_at từ ngày
	dateFields := []string{"created_at", "dob"}

	for _, field := range dateFields {
		fromKey := field + "_from"
		toKey := field + "_to"

		fromList, hasFrom := params[fromKey]
		toList, hasTo := params[toKey]

		if hasFrom || hasTo {
			rangeCond := bson.M{}

			if hasFrom && len(fromList) > 0 && fromList[0] != "" {
				t, _ := time.Parse("2006-01-02", fromList[0])
				rangeCond["$gte"] = t
			}
			if hasTo && len(toList) > 0 && toList[0] != "" {
				t, _ := time.Parse("2006-01-02", toList[0])
				rangeCond["$lte"] = t
			}
			if field == "dob" {
				field = "user_infor.dob"
			}
			andConditions = append(andConditions, bson.M{field: rangeCond})
		}
	}

	if len(andConditions) > 0 {
		filter["$and"] = andConditions
	}

	return filter
}

// Blog
func BuildBlogSearchFilter(params map[string][]string) bson.M {
	filter := bson.M{}
	andConditions := []bson.M{}

	//Kiểm tra có truyền keyword
	if keywords, ok := params["keyword"]; ok && len(keywords) > 0 {
		keyword := strings.TrimSpace(keywords[0])
		keyword = regexp.QuoteMeta(keyword)
		if keyword != "" {
			andConditions = append(andConditions, bson.M{
				"$text": bson.M{"$search": keyword},
			})
		}
	}

	//Kiểm tra có truyền tag ids
	if rawTagIds, ok := params["tag_ids"]; ok && len(rawTagIds) > 0 {
		var tagObjectIDs []primitive.ObjectID
		for _, idStr := range rawTagIds {
			if objID, err := primitive.ObjectIDFromHex(idStr); err == nil {
				tagObjectIDs = append(tagObjectIDs, objID)
			}
		}
		if len(tagObjectIDs) > 0 {
			andConditions = append(andConditions, bson.M{
				"tag_ids": bson.M{"$in": tagObjectIDs},
			})
		}
	}

	if len(andConditions) > 0 {
		filter["$and"] = andConditions
	}

	return filter
}

// Event
func BuildEventSearchFilter(params map[string][]string) bson.M {
	filter := bson.M{}

	var andConditions []bson.M

	andConditions = append(andConditions, bson.M{
		"deleted_at": bson.M{"$exists": false},
	})

	if keywords, ok := params["keyword"]; ok && len(keywords) > 0 {
		keyword := strings.TrimSpace(keywords[0])
		if keyword != "" {
			andConditions = append(andConditions, bson.M{
				"$text": bson.M{"$search": keyword},
			})
		}
	}

	// Lọc theo Topic IDs
	// params: ?topic_ids=id1&topic_ids=id2
	if rawTopicIDs, ok := params["topic_ids"]; ok && len(rawTopicIDs) > 0 {
		var topicObjectIDs []primitive.ObjectID
		for _, idStr := range rawTopicIDs {

			if objID, err := primitive.ObjectIDFromHex(idStr); err == nil {
				topicObjectIDs = append(topicObjectIDs, objID)
			}
		}
		if len(topicObjectIDs) > 0 {
			andConditions = append(andConditions, bson.M{
				"topic_ids": bson.M{"$in": topicObjectIDs},
			})
		}
	}

	// Lọc theo trạng thái Active
	// params: ?active=true
	if activeStatus, ok := params["active"]; ok && len(activeStatus) > 0 {
		if activeBool, err := strconv.ParseBool(activeStatus[0]); err == nil {
			andConditions = append(andConditions, bson.M{
				"active": activeBool,
			})
		}
	}

	// Lọc theo ID người tổ chức
	// params: ?organizer_id=...
	if orgIDs, ok := params["organizer_id"]; ok && len(orgIDs) > 0 {
		var orgObjectIDs []primitive.ObjectID
		for _, idStr := range orgIDs {
			if objID, err := primitive.ObjectIDFromHex(idStr); err == nil {
				orgObjectIDs = append(orgObjectIDs, objID)
			}
		}
		if len(orgObjectIDs) > 0 {
			andConditions = append(andConditions, bson.M{
				"created_by": bson.M{
					"$in": orgObjectIDs,
				},
			})
		}
	}

	// Lọc theo Khoảng giá (Price Range)
	// params: ?price_min=100000&price_max=500000
	priceFilter := bson.M{}
	if priceMinStr, ok := params["price_min"]; ok && len(priceMinStr) > 0 {
		if priceMin, err := strconv.Atoi(priceMinStr[0]); err == nil && priceMin >= 0 {
			priceFilter["$gte"] = priceMin
		}
	}
	if priceMaxStr, ok := params["price_max"]; ok && len(priceMaxStr) > 0 {
		if priceMax, err := strconv.Atoi(priceMaxStr[0]); err == nil && priceMax >= 0 {
			priceFilter["$lte"] = priceMax
		}
	}
	if len(priceFilter) > 0 {
		andConditions = append(andConditions, bson.M{"price": priceFilter})
	}

	// Lọc theo Khoảng ngày
	// params: ?start_date_from=2025-12-01T00:00:00Z&start_date_to=2025-12-31T23:59:59Z
	dateFilter := bson.M{}
	if dateFromStr, ok := params["start_date_from"]; ok && len(dateFromStr) > 0 {
		if dateFrom, err := time.Parse("2006-01-02", dateFromStr[0]); err == nil {
			dateFilter["$gte"] = dateFrom
		}
	}
	if dateToStr, ok := params["start_date_to"]; ok && len(dateToStr) > 0 {
		if dateTo, err := time.Parse("2006-01-02", dateToStr[0]); err == nil {
			dateFilter["$lte"] = dateTo
		}
	}

	if len(dateFilter) > 0 {
		andConditions = append(andConditions, bson.M{"event_time.start_date": dateFilter})
	}

	if len(andConditions) > 0 {
		filter["$and"] = andConditions
	}

	return filter
}

// Permission
func BuildPermissionSearchFilter(params map[string][]string) bson.M {
	filter := bson.M{}
	andConditions := []bson.M{}

	//Kiểm tra có truyền keyword
	if keywords, ok := params["keyword"]; ok && len(keywords) > 0 {
		keyword := strings.TrimSpace(keywords[0])
		keyword = regexp.QuoteMeta(keyword)
		if keyword != "" {
			andConditions = append(andConditions, bson.M{
				"name": bson.M{"$regex": keyword, "$options": "i"},
			})
		}
	}

	if len(andConditions) > 0 {
		filter["$and"] = andConditions
	}

	return filter
}

// Build filter sort
func BuildSortFilter(params map[string][]string) bson.D {
	sorts := bson.D{}

	//?sort=view_desc,created_at_asc
	if v, ok := params["sorts"]; ok && len(v) > 0 {
		for _, sortFeild := range v {
			lastIndex := strings.LastIndex(sortFeild, "_")
			var (
				field string
				order string
			)
			if lastIndex == -1 {
				field = sortFeild
				order = "asc"
			} else {
				field = sortFeild[:lastIndex]
				order = sortFeild[lastIndex+1:]
			}
			value := 1
			if strings.ToLower(order) == "desc" {
				value = -1
			}

			sorts = append(sorts, bson.E{Key: field, Value: value})
		}
	}

	if len(sorts) == 0 {
		sorts = bson.D{{Key: "created_at", Value: -1}}
	}

	return sorts
}

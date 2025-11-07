package utils

import (
	"regexp"
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

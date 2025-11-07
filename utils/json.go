package utils

import (
	"go.mongodb.org/mongo-driver/bson"
)

type ZeroChecker interface {
	IsZero() bool
}

func PrettyJSON(data bson.M) bson.M {
	for key, value := range data {
		if value == nil {
			delete(data, key)
			continue
		}
		switch v := value.(type) {
		case ZeroChecker:
			if v.IsZero() {
				delete(data, key)
			}
		case string:
			if v == "" {
				delete(data, key)
			}
		}

	}

	return data
}

package utils

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9-]`)
)

// GenerateSlug Tạo ra slug từ chuỗi string. Ví dụ: Công nghệ
//
// Param:
//   - input (string): là đầu vào muốn chuyển thành slug
//
// Return:
//
//   - quotient: kết quả sau khi chuyển. Ví dụ: cong-nghe
func GenerateSlug(input string) string {
	//Loại bỏ dấu
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

	// Áp dụng transformer
	normalized, _, _ := transform.String(t, input)

	lowercased := strings.ToLower(normalized)

	//Thay thế khoảng trắng bằng gạch ngang
	withHyphens := strings.ReplaceAll(lowercased, " ", "-")

	//Loại bỏ tất cả các ký tự không phải chữ, số, hoặc gạch ngang
	finalSlug := nonAlphanumericRegex.ReplaceAllString(withHyphens, "")

	//Xóa gạch ngang ở đầu hoặc cuối chuỗi
	finalSlug = strings.Trim(finalSlug, "-")

	return finalSlug
}

func GenerateUUIDTransaction(prefix string) string {
	id := strings.ToUpper(strings.ReplaceAll(uuid.New().String()[:8], "-", ""))
	t := time.Now().Format("20060102")
	return fmt.Sprintf("%s-%s-%s", prefix, t, id)
}

func ExtractUniqueIDs[T any](list []T, extract func(T) []primitive.ObjectID) []primitive.ObjectID {
	exists := make(map[primitive.ObjectID]bool)

	for _, item := range list {
		ids := extract(item)
		for _, id := range ids {
			if id != primitive.NilObjectID && !exists[id] {
				exists[id] = true
			}
		}
	}

	unique := make([]primitive.ObjectID, 0, len(exists))
	for id := range exists {
		unique = append(unique, id)
	}
	return unique
}

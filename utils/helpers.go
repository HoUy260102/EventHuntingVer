package utils

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var listAllowedRole = []string{
	"Admin",
}

var (
	nonAlphanumericRegex = regexp.MustCompile(`[^a-z0-9-]`)
	multipleHyphenRegex  = regexp.MustCompile(`-+`)
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
	input = strings.ReplaceAll(input, "Đ", "D")
	input = strings.ReplaceAll(input, "đ", "d")

	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

	// Áp dụng transformer
	normalized, _, _ := transform.String(t, input)

	lowercased := strings.ToLower(normalized)

	// Thay khoảng trắng bằng gạch ngang
	withHyphens := strings.ReplaceAll(lowercased, " ", "-")

	// Thay tất cả ký tự không phải chữ, số, hoặc "-" bằng "-"
	replaced := nonAlphanumericRegex.ReplaceAllString(withHyphens, "-")

	// Xóa các dấu "-" liên tiếp
	cleaned := multipleHyphenRegex.ReplaceAllString(replaced, "-")

	// Xóa "-" ở đầu hoặc cuối chuỗi
	finalSlug := strings.Trim(cleaned, "-")

	return finalSlug
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

func CanModifyResource(ownerID primitive.ObjectID, accountID primitive.ObjectID, roles []string) bool {
	for _, role := range roles {
		if slices.Contains(listAllowedRole, role) {
			return true
		}
	}
	return ownerID == accountID
}

func GenerateInvoiceNumber() string {
	return fmt.Sprintf("INV-%s-%d", time.Now().Format("20060102"), time.Now().UnixNano()%10000)
}

func hmacSha512(secret string, data string) string {
	h := hmac.New(sha512.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

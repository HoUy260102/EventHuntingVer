package utils

import (
	"EventHunting/configs"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type VnpayRequest struct {
	Version    string `json:"vnp_Version"`
	Command    string `json:"vnp_Command"`
	TmnCode    string `json:"vnp_TmnCode"`
	Locale     string `json:"vnp_Locale"`
	CurrCode   string `json:"vnp_CurrCode"`
	TxnRef     string `json:"vnp_TxnRef"`
	OrderInfo  string `json:"vnp_OrderInfo"`
	OrderType  string `json:"vnp_OrderType"`
	Amount     int64  `json:"vnp_Amount"` // This is amount * 100
	ReturnUrl  string `json:"vnp_ReturnUrl"`
	IpAddr     string `json:"vnp_IpAddr"`
	CreateDate string `json:"vnp_CreateDate"`
	ExpireDate string `json:"vnp_ExpireDate"`
	SecureHash string `json:"vnp_SecureHash"`
}

// Hàm tạo URL thanh toán VNPAY (BuildVnpayURL)
// Hàm này đã đúng
func BuildVnpayURL(req VnpayRequest) string {
	baseURL := "https://sandbox.vnpayment.vn/paymentv2/vpcpay.html"

	params := url.Values{}
	params.Add("vnp_Version", req.Version)
	params.Add("vnp_Command", req.Command)
	params.Add("vnp_TmnCode", req.TmnCode)
	params.Add("vnp_Locale", req.Locale)
	params.Add("vnp_CurrCode", req.CurrCode)
	params.Add("vnp_TxnRef", req.TxnRef)
	params.Add("vnp_OrderInfo", req.OrderInfo)
	params.Add("vnp_OrderType", req.OrderType)
	params.Add("vnp_Amount", strconv.FormatInt(req.Amount, 10)) // Dùng req.Amount
	params.Add("vnp_ReturnUrl", req.ReturnUrl)
	params.Add("vnp_IpAddr", req.IpAddr)
	params.Add("vnp_CreateDate", req.CreateDate)
	params.Add("vnp_ExpireDate", req.ExpireDate)

	encoded := params.Encode()

	return baseURL + "?" + encoded +
		"&vnp_SecureHashType=SHA512&vnp_SecureHash=" + req.SecureHash
}

// Tạo URL thanh toán (CreatePaymentURL)
func CreatePaymentURL(orderID string, amount int64, orderInfo string, ipAdrr string) (string, error) {
	loc, _ := time.LoadLocation("Asia/Ho_Chi_Minh")
	now := time.Now().In(loc)

	tmnCode := strings.TrimSpace(configs.GetVNPAYTmnCode())
	hashSecret := strings.TrimSpace(configs.GetVNPAYHashSecret())
	appBaseUrl := strings.TrimSpace(configs.GetServerDomain())

	log.Printf("VNPAY TmnCode: %s", tmnCode)
	log.Printf("VNPAY HashSecret (sau khi Trim): %s", hashSecret)

	req := VnpayRequest{
		Version:    "2.1.0",
		Command:    "pay",
		TmnCode:    tmnCode,
		Locale:     "vn",
		CurrCode:   "VND",
		TxnRef:     orderID,
		OrderInfo:  orderInfo,
		OrderType:  "other",
		Amount:     amount * 100, // (1) req.Amount = amount * 100
		ReturnUrl:  appBaseUrl + "/vnpay_return",
		IpAddr:     ipAdrr,
		CreateDate: now.Format("20060102150405"),
		ExpireDate: now.Add(15 * time.Minute).Format("20060102150405"),
	}

	// Build map params để tạo hash
	params := map[string]string{
		"vnp_Version":    req.Version,
		"vnp_Command":    req.Command,
		"vnp_TmnCode":    req.TmnCode,
		"vnp_Locale":     req.Locale,
		"vnp_CurrCode":   req.CurrCode,
		"vnp_TxnRef":     req.TxnRef,
		"vnp_OrderInfo":  req.OrderInfo,
		"vnp_OrderType":  req.OrderType,
		"vnp_Amount":     strconv.FormatInt(req.Amount, 10), // (2) Dùng req.Amount để hash. Đây là logic ĐÚNG.
		"vnp_ReturnUrl":  req.ReturnUrl,
		"vnp_IpAddr":     req.IpAddr,
		"vnp_CreateDate": req.CreateDate,
		"vnp_ExpireDate": req.ExpireDate,
	}

	// Loại bỏ các tham số rỗng
	for k, v := range params {
		if v == "" {
			delete(params, k)
		}
	}

	// Sort keys theo thứ tự alphabet
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build rawData để tạo hash
	var rawData strings.Builder
	for i, k := range keys {
		rawData.WriteString(k)
		rawData.WriteString("=")
		// SỬA LỖI QUAN TRỌNG:
		// Giá trị (value) PHẢI được URL Encode (giống hệt Java demo)
		rawData.WriteString(url.QueryEscape(params[k]))
		if i < len(keys)-1 {
			rawData.WriteString("&")
		}
	}

	log.Println("VNPAY RAW DATA (FOR HASHING):", rawData.String())

	// Tạo chữ ký
	req.SecureHash = hmacSha512(hashSecret, rawData.String())
	log.Printf("VNPAY Generated Hash: %s", req.SecureHash)

	return BuildVnpayURL(req), nil
}

// HÀM 2: VERIFY IPN
func VerifyIPNChecksum(vnpParams url.Values) error {
	receivedHash := vnpParams.Get("vnp_SecureHash")
	hashSecret := strings.TrimSpace(configs.GetVNPAYHashSecret())

	var keys []string
	for k := range vnpParams {
		if k == "vnp_SecureHash" || k == "vnp_SecureHashType" {
			continue
		}
		if vnpParams.Get(k) == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var hashDataBuffer strings.Builder
	for i, k := range keys {
		if i > 0 {
			hashDataBuffer.WriteString("&")
		}
		hashDataBuffer.WriteString(k)
		hashDataBuffer.WriteString("=")
		hashDataBuffer.WriteString(url.QueryEscape(vnpParams.Get(k)))
	}

	log.Println("VNPAY IPN RAW DATA (FOR HASHING):", hashDataBuffer.String())

	myHash := hmacSha512(hashSecret, hashDataBuffer.String())
	log.Printf("VNPAY IPN Calculated Hash: %s", myHash)
	log.Printf("VNPAY IPN Received Hash:   %s", receivedHash)

	if myHash != receivedHash {
		log.Printf("VNPAY IPN Checksum failed. Received: %s, Calculated: %s", receivedHash, myHash)
		return fmt.Errorf("sai chữ ký. Received: %s, Calculated: %s", receivedHash, myHash)
	}

	log.Println("VNPAY IPN Checksum success.")
	return nil
}

func ResponsePaymentMessage(code string) string {
	switch code {
	case "00":
		return "Giao dịch thành công"
	case "07":
		return "Trừ tiền thành công. Giao dịch bị nghi ngờ (liên quan tới lừa đảo, giao dịch bất thường)."
	case "09":
		return "Thẻ/Tài khoản của khách hàng chưa đăng ký dịch vụ InternetBanking tại ngân hàng."
	case "10":
		return "Khách hàng xác thực thông tin thẻ/tài khoản không đúng quá 3 lần."
	case "11":
		return "Đã hết hạn chờ thanh toán. Vui lòng thực hiện lại giao dịch."
	case "12":
		return "Thẻ/Tài khoản của khách hàng bị khóa."
	case "13":
		return "Quý khách nhập sai mật khẩu xác thực giao dịch (OTP). Vui lòng thực hiện lại giao dịch."
	case "24":
		return "Khách hàng hủy giao dịch."
	case "51":
		return "Tài khoản của quý khách không đủ số dư để thực hiện giao dịch."
	case "65":
		return "Tài khoản của quý khách đã vượt quá hạn mức giao dịch trong ngày."
	case "75":
		return "Ngân hàng thanh toán đang bảo trì."
	case "79":
		return "Khách hàng nhập sai mật khẩu thanh toán quá số lần quy định. Vui lòng thực hiện lại giao dịch."
	case "99":
		return "Lỗi khác (không có trong danh sách mã lỗi)."
	default:
		return "Không xác định mã lỗi."
	}
}

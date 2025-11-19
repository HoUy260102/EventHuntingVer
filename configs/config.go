package configs

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

var mpConfig map[string]interface{}

func LoadFileConfig() {
	_ = godotenv.Load()
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "local"
	}

	filePath := fmt.Sprintf("configs/config.%s.yaml", env)
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Fatal("Lỗi đọc file", err)
	}

	//Thay biến môi trường trong file YAML bằng giá trị thực tế.
	expandedYaml := os.ExpandEnv(string(data))

	if err := yaml.Unmarshal([]byte(expandedYaml), &mpConfig); err != nil {
		log.Fatal("Lỗi parsing YAML:", err)
		return
	}

	fmt.Println("Config thành công")
}

func GetServerPort() string {
	server := mpConfig["server"].(map[string]interface{})
	return fmt.Sprintf("%v", server["port"])
}

func GetServerDomain() string {
	server := mpConfig["server"].(map[string]interface{})
	return fmt.Sprintf("%v", server["domain"])
}

func GetDatabaseURI() string {
	db := mpConfig["database"].(map[string]interface{})
	return fmt.Sprintf("%v", db["uri"])
}

func GetDatabaseName() string {
	db := mpConfig["database"].(map[string]interface{})
	return fmt.Sprintf("%v", db["name"])
}

func GetJWTSecret() string {
	jwt := mpConfig["jwt"].(map[string]interface{})
	return fmt.Sprintf("%v", jwt["secret_key"])
}

func GetJWTIssuer() string {
	jwt := mpConfig["jwt"].(map[string]interface{})
	return fmt.Sprintf("%v", jwt["issuer"])
}

func GetJWTAccessExp() int {
	jwt := mpConfig["jwt"].(map[string]interface{})
	return int(jwt["jwt_access_token_expiration_time"].(int))
}

func GetJWTRefreshExp() int {
	jwt := mpConfig["jwt"].(map[string]interface{})
	return int(jwt["jwt_refresh_token_expiration_time"].(int))
}

func GetJWTApprovedExp() int {
	jwt := mpConfig["jwt"].(map[string]interface{})
	return int(jwt["jwt_aprroved_token_expiration_time"].(int))
}

func GetJWTVerifyExp() int {
	jwt := mpConfig["jwt"].(map[string]interface{})
	return int(jwt["jwt_verify_token_expiration_time"].(int))
}

func GetJWTResetExp() int {
	jwt := mpConfig["jwt"].(map[string]interface{})
	return int(jwt["jwt_reset_token_expiration_time"].(int))
}

func GetRedisAddr() string {
	redis := mpConfig["redis"].(map[string]interface{})
	return fmt.Sprintf("%v", redis["addr"])
}

func GetRedisPassword() string {
	redis := mpConfig["redis"].(map[string]interface{})
	return fmt.Sprintf("%v", redis["password"])
}

func GetRedisDB() int {
	redis := mpConfig["redis"].(map[string]interface{})
	return int(redis["db"].(int))
}

func GetSMTPHost() string {
	smtp := mpConfig["smtp"].(map[string]interface{})
	return fmt.Sprintf("%v", smtp["host"])
}

func GetSMTPPort() string {
	smtp := mpConfig["smtp"].(map[string]interface{})
	return fmt.Sprintf("%v", smtp["port"])
}

func GetSenderEmail() string {
	app := mpConfig["app"].(map[string]interface{})
	return fmt.Sprintf("%v", app["sender_email"])
}

func GetAppPassword() string {
	app := mpConfig["app"].(map[string]interface{})
	return fmt.Sprintf("%v", app["app_password"])
}

func GetGoogleClientID() string {
	google := mpConfig["google_oauth"].(map[string]interface{})
	return fmt.Sprintf("%v", google["client_id"])
}

func GetGoogleSecret() string {
	google := mpConfig["google_oauth"].(map[string]interface{})
	return fmt.Sprintf("%v", google["client_secret"])
}

func GetGoogleRedirectURL() string {
	google := mpConfig["google_oauth"].(map[string]interface{})
	return fmt.Sprintf("%v", google["redirect_url"])
}

func GetCloudinaryName() string {
	cloud := mpConfig["cloudinary"].(map[string]interface{})
	return fmt.Sprintf("%v", cloud["cloud_name"])
}

func GetCloudinaryKey() string {
	cloud := mpConfig["cloudinary"].(map[string]interface{})
	return fmt.Sprintf("%v", cloud["api_key"])
}

func GetCloudinarySecret() string {
	cloud := mpConfig["cloudinary"].(map[string]interface{})
	return fmt.Sprintf("%v", cloud["api_secret"])
}

func GetSessionSecret() string {
	session := mpConfig["session"].(map[string]interface{})
	return fmt.Sprintf("%v", session["secret"])
}

func GetDefaultPagination(name string) (int, int) {
	pagination := mpConfig["pagination"].(map[string]interface{})
	target, ok := pagination[name].(map[string]interface{})
	if !ok {
		return 1, 10
	}
	length := target["default_length"].(int)
	maxLength := target["default_max_length"].(int)
	return length, maxLength
}

func GetVNPAYTmnCode() string {
	vnPay := mpConfig["vn_pay"].(map[string]interface{})
	return fmt.Sprintf("%v", vnPay["tmncode"])
}

func GetVNPAYHashSecret() string {
	vnPay := mpConfig["vn_pay"].(map[string]interface{})
	return fmt.Sprintf("%v", vnPay["hash_secret"])
}

func GetVNPAYUrl() string {
	vnPay := mpConfig["vn_pay"].(map[string]interface{})
	return fmt.Sprintf("%v", vnPay["url"])
}

func GetRegisExpirationMinutes() int {
	jobs := mpConfig["jobs"].(map[string]interface{})
	target := jobs["registration"].(map[string]interface{})
	return target["expiration_minutes"].(int)
}

func GetMaxRetries() int {
	jobs := mpConfig["jobs"].(map[string]interface{})
	return jobs["max_retries"].(int)
}

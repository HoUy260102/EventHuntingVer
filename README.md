# EventHunting - Hệ thống Đặt vé & Thanh toán Sự kiện Online

**EventHunting** là hệ thống Backend cung cấp giải pháp cho việc quản lý sự kiện, đặt vé trực tuyến và thanh toán tự động.

## Tính năng Kỹ thuật Nổi bật

### 1. Xử lý Đồng thời & Giữ vé (Concurrency Control)
* **Vấn đề:** Nếu có nhiều người cùng mua vé vào một thời điểm mà số vé có giới hạn.
* **Giải pháp:** Sử dụng cơ chế **Optimistic Locking** kết hợp điều kiện nguyên tử `$lte` (Less than or Equal) trong MongoDB.
* **Kết quả:** Đảm bảo chính xác tuyệt đối số lượng vé bán ra, ngăn chặn hoàn toàn tình trạng bán quá số lượng.

### 2. Thanh toán & Transaction (ACID Compliance)
* **Tích hợp VNPAY:** Hỗ trợ thanh toán qua VNPAY.
* **MongoDB Multi-Document Transactions**:
    1.  Cập nhật trạng thái đơn hàng (`Pending` -> `Paid`).
    2.  Tạo hóa đơn điện tử (`Invoice`).
    3.  **Cam kết:** Cả hai hành động phải cùng thành công hoặc cùng thất bại (Rollback).

### 3. Hệ thống Email Bất đồng bộ (Redis Queue)
* **Mô hình:** Producer-Consumer.
* **Luồng xử lý:** API phản hồi ngay lập tức (<200ms) sau khi thanh toán thành công, đẩy Job gửi vé vào **Redis Queue**.
* **Worker:** Một tiến trình chạy ngầm (Goroutine) sẽ lấy Job từ Queue và thực hiện sinh vé tự động, gửi email vé (kèm QR Code) tới người dùng.
* **Auto-Retry:** Tự động thử lại (Exponential Backoff) nếu việc gửi email gặp sự cố.
---

## Kiến trúc Hệ thống (Architecture Flow)

## Công nghệ sử dụng:
**Ngôn ngữ** : Golang, Xử lý tác vụ đồng thời (Concurrency) mạnh mẽ, hiệu năng cao, phù hợp cho backend chịu tải lớn.
**Framework**: Gin Gonic, Web Framework nhẹ, tốc độ xử lý request nhanh nhất trong hệ sinh thái Go.
**Database**: MongoDB, Lưu trữ dữ liệu linh hoạt
**Cache / Queue**: Redis, Đóng vai trò là **Message Broker** cho hàng đợi gửi email và caching phiên làm việc. 
**Payment**: VNPAY, Cổng thanh toán tích hợp (ATM/QR/Visa). Xử lý checksum và IPN an toàn. 
**Auth** : JWT, Xác thực và phân quyền người dùng (Stateless Authentication). 

## Hướng dẫn chạy:
## Bước 1: Cài đặt Redis
1.  **Tải và cài đặt:**
    * **Windows:** Tải bản cài đặt tại [Redis for Windows](https://github.com/microsoftarchive/redis/releases) (Hoặc dùng Memurai).
    * **MacOS:** Chạy `brew install redis`.
    * **Linux:** `sudo apt install redis-server`.
2.  **Khởi động Redis:**
    * Đảm bảo Redis đang chạy ở port mặc định `6379`.

## Bước 2: Cài đặt & Cấu hình MongoDB

### 2.1. Cài đặt MongoDB
Tải **MongoDB Community Server** tại [trang chủ MongoDB](https://www.mongodb.com/try/download/community) và cài đặt như bình thường.

### 2.2. Chuyển sang chế độ Replica Set
Mặc định MongoDB chạy ở chế độ Standalone (không Transaction). Bạn cần sửa file config để bật Replication.

## Bước 3: Cài đặt Golang & Setup Dự án

1.  **Cài đặt Go:** Tải bản 1.21+ tại [go.dev/dl](https://go.dev/dl/).
2.  **Clone code:**
    ```bash
    git clone https://github.com/HoUy260102/EventHuntingVer.git
    cd EventHuntingVer
    ```
3.  **Tải thư viện:**
    ```bash
    go mod tidy
    ```
---

## Bước 4: Cấu hình file .env

Tạo file `.env` tại thư mục gốc dự án.
Điền các thông số giống trong file .env.example

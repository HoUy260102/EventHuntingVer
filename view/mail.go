package view

import (
	"EventHunting/collections"
	"EventHunting/configs"
	"fmt"
	"html/template"
	"log"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Ticket
type EmailTemplateData struct {
	RecipientName string
	EventName     string
	EventTime     string
	EventLocation string
	Tickets       []TicketTemplateData
}

type TicketTemplateData struct {
	QRCodeCID      string
	TicketTypeName string
	TicketPrice    int // Giả sử Price là int64
	RegisterDate   string
	TicketCode     string
}

// Dùng html/template thay vì strings.Builder để an toàn và dễ bảo trì
var ticketEmailTemplate = template.Must(template.New("ticketEmail").Parse(`
<html><body style='font-family: Arial, sans-serif; line-height: 1.6; margin: 0; padding: 0;'>
<div style='max-width: 640px; margin: 20px auto; padding: 20px; border: 1px solid #ddd; border-radius: 8px;'>
    <h2>Xin chào {{.RecipientName}},</h2>
    <p>Cảm ơn bạn đã đăng ký tham gia sự kiện của chúng tôi. Dưới đây là thông tin sự kiện và vé của bạn.</p>

    <h3 style='border-bottom: 2px solid #eee; padding-bottom: 5px;'>Thông tin sự kiện</h3>
    <p style='margin: 5px 0;'><strong>Sự kiện:</strong> {{.EventName}}</p>
    <p style='margin: 5px 0;'><strong>Thời gian:</strong> {{.EventTime}}</p>
    <p style='margin: 5px 0;'><strong>Địa điểm:</strong> {{.EventLocation}}</p>
    <br>

    <h3 style='border-bottom: 2px solid #eee; padding-bottom: 5px;'>Chi tiết vé</h3>
    <p>Vui lòng đưa mã QR bên dưới cho ban tổ chức tại cổng check-in.</p>

    {{range .Tickets}}
    <div style='border: 1px solid #ddd; border-radius: 8px; padding: 16px; margin-bottom: 20px;'>
        <table border='0' cellpadding='0' cellspacing='0' width='100%'>
            <tr>
                <td width='140' style='width: 140px; padding-right: 15px; vertical-align: top;'>
                    <img src='cid:{{.QRCodeCID}}' alt='Mã QR' width='120' height='120' style='width: 120px; height: 120px; border: 1px solid #eee;' />
                </td>
                <td style='vertical-align: top; font-size: 14px; line-height: 1.7;'>
                    <strong style='font-size: 16px; color: #333;'>{{.TicketTypeName}}</strong><br>
                    Giá vé: {{.TicketPrice}} VNĐ<br>
                    Ngày đăng ký: {{.RegisterDate}}<br>
                    Mã vé: <code style='font-size: 13px; background-color: #f4f4f4; padding: 2px 5px; border-radius: 4px;'>{{.TicketCode}}</code>
                </td>
            </tr>
        </table>
    </div>
    {{end}}

    <hr style='border: 0; border-top: 1px solid #eee; margin-top: 20px;'>
    <p style='font-size: 12px; color: #777;'>Trân trọng,<br>Đội ngũ EventHunting</p>
</div>
</body></html>
`))

func BuildTicketEmail(
	eventEntry *collections.Event,
	accountEntry *collections.Account,
	tickets collections.Tickets,
	ticketTypeMap map[primitive.ObjectID]collections.TicketType,
) (string, string, map[string][]byte, error) {

	eventTime := eventEntry.EventTime.StartDate.Format("02/01/2006") + "-" + eventEntry.EventTime.EndDate.Format("02/01/2006") + " lúc: " + eventEntry.EventTime.StartTime

	templateData := EmailTemplateData{
		RecipientName: accountEntry.Name,
		EventName:     eventEntry.Name,
		EventTime:     eventTime,
		EventLocation: eventEntry.EventLocation.Name + ", " + eventEntry.EventLocation.Address,
		Tickets:       []TicketTemplateData{},
	}

	embeddedFiles := make(map[string][]byte)
	vietnamLoc := time.FixedZone("ICT", 7*60*60)

	for i, ticket := range tickets {
		// Tạo QR
		qrLink := configs.GetServerDomain() + "/ticket/checkin/token?" + ticket.QRCodeData
		qrCodePng, err := qrcode.Encode(qrLink, qrcode.Medium, 256)
		if err != nil {
			return "", "", nil, fmt.Errorf("lỗi tạo mã QR cho vé %s: %w", ticket.QRCodeData, err)
		}

		cid := fmt.Sprintf("qrcode%d.png", i)
		embeddedFiles[cid] = qrCodePng

		ticketType, ok := ticketTypeMap[ticket.TicketTypeID]
		if !ok {
			log.Printf("WARNING: Không tìm thấy TicketTypeID %s trong map. Sử dụng tên mặc định.", ticket.TicketTypeID.Hex())
			ticketType = collections.TicketType{Name: "Vé (Không rõ loại)", Price: 0}
		}

		//Khởi tạo vé
		ticketData := TicketTemplateData{
			QRCodeCID:      cid,
			TicketTypeName: ticketType.Name,
			TicketPrice:    ticketType.Price,
			RegisterDate:   ticket.CreatedAt.In(vietnamLoc).Format("15:04 02/01/2006"),
			TicketCode:     ticket.QRCodeData,
		}

		templateData.Tickets = append(templateData.Tickets, ticketData)
	}

	var emailBody strings.Builder
	if err := ticketEmailTemplate.Execute(&emailBody, templateData); err != nil {
		return "", "", nil, fmt.Errorf("lỗi render email template: %w", err)
	}

	subject := fmt.Sprintf("Vé tham dự sự kiện: %s", eventEntry.Name)
	return subject, emailBody.String(), embeddedFiles, nil
}

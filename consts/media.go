package consts

type MediaFormat string
type MediaStatus string

const (
	MEDIA_IMAGE MediaFormat = "IMAGE"
	MEDIA_VIDEO MediaFormat = "VIDEO"
)

const (
	MediaStatusPending MediaStatus = "PENDING"
	MediaStatusSuccess MediaStatus = "SUCCESS"
	MediaStatusError   MediaStatus = "ERROR"
)

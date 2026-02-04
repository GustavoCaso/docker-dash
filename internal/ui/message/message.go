package message

// ShowBannerMsg is sent by components to display a banner notification
type ShowBannerMsg struct {
	Message string
	IsError bool
}

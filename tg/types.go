package tg

// UserMsg is a plain user text message
type UserMsg struct {
	ChatID  int64
	ID      int
	Text    string
	PhotoID string
}

// Btn is a telegram inline btn
type Btn struct {
	ID   string
	Text string
}

// BtnClick is a telegram inline btn reply
type BtnClick struct {
	BtnID string
	Msg   UserMsg
}

type BotMessage struct {
	ChatID       int64
	ReplyToMsgID int
	Text         string
	TextMarkdown bool
	Btns         []Btn
}

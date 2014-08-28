package soymsg

type Bundle interface {
	Message(id uint64) *Message
}

type Message struct {
	ID   uint64
	Body string
}
